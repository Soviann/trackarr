package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEnrichmentUpdate_LocksAndPreservesMatch(t *testing.T) {
	result := &matching.MatchResult{
		IMDBID:      "tt9999999",
		TMDBID:      111,
		TVDBID:      222,
		AniListID:   333,
		MatchStatus: model.MatchStatusConfirmed,
		MatchSource: matching.MatchSourceTMDBSearch,
	}

	t.Run("no locks applies every ID and rewrites match", func(t *testing.T) {
		u := service.BuildEnrichmentUpdateForTest(result, service.EnrichmentPayload{})
		require.NotNil(t, u.TMDBID)
		assert.Equal(t, int64(111), *u.TMDBID)
		require.NotNil(t, u.TVDBID)
		assert.Equal(t, int64(222), *u.TVDBID)
		require.NotNil(t, u.IMDBID)
		require.NotNil(t, u.AniListID)
		require.NotNil(t, u.MatchSource, "match source rewritten on a fresh match")
	})

	t.Run("locked IDs are not written, preserve keeps match state", func(t *testing.T) {
		u := service.BuildEnrichmentUpdateForTest(result, service.EnrichmentPayload{
			LockedIDs:     []string{service.LockTVDB, service.LockIMDB},
			PreserveMatch: true,
		})
		assert.Nil(t, u.TVDBID, "locked TVDB left untouched")
		assert.Nil(t, u.IMDBID, "locked IMDB left untouched")
		require.NotNil(t, u.TMDBID, "unlocked TMDB still back-filled")
		assert.Equal(t, int64(111), *u.TMDBID)
		assert.Nil(t, u.MatchStatus, "PreserveMatch keeps existing match status")
		assert.Nil(t, u.MatchSource, "PreserveMatch keeps existing match source")
	})
}

// fakeAniListPusher records PushSeasonState/PushMovieState invocations so
// dispatch tests can assert the worker routes each task kind to the right
// method with the payload decoded correctly.
type fakeAniListPusher struct {
	seasonCalls []int64
	movieCalls  []int64
	seasonErr   error
	movieErr    error
}

func (f *fakeAniListPusher) PushSeasonState(_ context.Context, id int64) error {
	f.seasonCalls = append(f.seasonCalls, id)
	return f.seasonErr
}

func (f *fakeAniListPusher) PushMovieState(_ context.Context, id int64) error {
	f.movieCalls = append(f.movieCalls, id)
	return f.movieErr
}

// TestHandleEnrichment_PersistsAllFieldsInSingleTx drives handleEnrichment
// against a real in-memory SQLite with a pre-populated watch_events count.
// The pipeline is deliberately kept empty (no HTTP clients) — the enrichment
// payload carries a TMDB ID so Run() short-circuits on Plex metadata IDs.
// The test asserts every Task 2 write lands atomically: title fields, watch
// minutes, alias names, and genre join rows.
func TestHandleEnrichment_PersistsAllFieldsInSingleTx(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Old Name", Language: "en", IsPrimary: true}})

	// Seed three watch events so recalcWatchtime has a non-zero multiplier.
	for range 3 {
		testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: id, Source: model.WatchEventSourceManual})
	}

	// Empty pipeline: TMDB/AniList/Gemini nil. Run() returns early on the
	// Plex-IDs branch because payload.TMDBID is non-zero, so no HTTP is hit.
	pipeline := matching.NewPipeline(nil, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, nil, nil, nil, nil, t.TempDir(), titleSvc, db)

	payload := service.EnrichmentPayload{
		TitleID:   id,
		TitleName: "Old Name",
		Year:      2024,
		TitleType: model.TitleTypeMovie,
		TMDBID:    42,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	after, err := titles.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, model.MatchStatusConfirmed, after.MatchStatus,
		"tx committed: match_status updated")
	require.NotNil(t, after.TMDBID)
	assert.Equal(t, int64(42), *after.TMDBID, "tx committed: tmdb_id persisted")
	require.NotNil(t, after.MatchSource)
	assert.Equal(t, matching.MatchSourcePlexIDs, *after.MatchSource)
}

// TestHandleEnrichment_ResolvesOtherIDsFromIMDB drives the auto-find path: an
// enrichment payload carrying only an IMDb id must resolve the other external
// IDs through TMDB's /find/{imdb_id} endpoint. The bug was that handleEnrichment
// dropped payload.IMDBID when building the matcher input, so the pipeline never
// had the id to resolve from and the title gained no TMDB id.
func TestHandleEnrichment_ResolvesOtherIDsFromIMDB(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2025,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Placeholder", Language: "en", IsPrimary: true}})

	const wantTMDB = int64(987654)
	mux := http.NewServeMux()
	mux.HandleFunc("/find/tt31974288", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"movie_results": []map[string]any{{"id": wantTMDB, "title": "Resolved Movie"}},
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	server := httptest.NewServer(mux)
	defer server.Close()

	tmdb := matching.NewTMDBClient("test-key")
	tmdb.SetBaseURL(server.URL)
	pipeline := matching.NewPipeline(tmdb, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, tmdb, nil, nil, nil, t.TempDir(), titleSvc, db)

	raw, err := json.Marshal(service.EnrichmentPayload{
		TitleID:   id,
		TitleName: "Placeholder",
		TitleType: model.TitleTypeMovie,
		IMDBID:    "tt31974288",
		LockedIDs: []string{service.LockIMDB},
	})
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	got, err := titles.GetByID(id)
	require.NoError(t, err)
	require.NotNil(t, got.TMDBID, "enrichment must resolve a TMDB id from the IMDb anchor via /find")
	assert.Equal(t, wantTMDB, *got.TMDBID)
}

// pendingRefreshFor returns the first pending refresh task targeting titleID, or
// nil. Used to assert season-backfill enqueue behaviour after enrichment.
func pendingRefreshFor(t *testing.T, tasks *repository.TaskRepository, titleID int64) *model.Task {
	t.Helper()
	pending, err := tasks.ListPending()
	require.NoError(t, err)
	for i := range pending {
		if pending[i].TaskType != model.TaskTypeRefresh {
			continue
		}
		var rp service.RefreshPayload
		if err := json.Unmarshal([]byte(pending[i].Payload), &rp); err != nil {
			continue
		}
		if rp.TitleID == titleID {
			return &pending[i]
		}
	}
	return nil
}

// TestHandleEnrichment_EnqueuesSeasonBackfillForSeries verifies that matching a
// series enqueues a refresh so its seasons/episodes are populated right away.
// Enrichment writes IDs/metadata but never creates seasons (only refresh does),
// so without this a just-matched series sits in the review queue with an empty
// episode list — the state that previously crashed the title page.
func TestHandleEnrichment_EnqueuesSeasonBackfillForSeries(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Some Series", Language: "en", IsPrimary: true}})

	pipeline := matching.NewPipeline(nil, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, nil, nil, nil, nil, t.TempDir(), titleSvc, db)

	raw, err := json.Marshal(service.EnrichmentPayload{
		TitleID:   id,
		TitleName: "Some Series",
		Year:      2024,
		TitleType: model.TitleTypeSeries,
		TMDBID:    1396,
	})
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	assert.NotNil(t, pendingRefreshFor(t, tasks, id),
		"matching a series should enqueue a refresh to backfill its seasons")
}

// TestHandleEnrichment_NoSeasonBackfillForMovie guards the inverse: movies have
// no seasons, so enrichment must not enqueue a redundant refresh for them.
func TestHandleEnrichment_NoSeasonBackfillForMovie(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Some Movie", Language: "en", IsPrimary: true}})

	pipeline := matching.NewPipeline(nil, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, nil, nil, nil, nil, t.TempDir(), titleSvc, db)

	raw, err := json.Marshal(service.EnrichmentPayload{
		TitleID:   id,
		TitleName: "Some Movie",
		Year:      2024,
		TitleType: model.TitleTypeMovie,
		TMDBID:    42,
	})
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	assert.Nil(t, pendingRefreshFor(t, tasks, id),
		"matching a movie must not enqueue a season-backfill refresh")
}

// TestHandleEnrichment_CtxCancelRollsBack makes sure a cancelled context
// prevents the persistence transaction from committing. With handleEnrichment
// now wrapping all writes in a single WithTxContext, BeginTx must fail before
// any Update/ReplaceNames hits the row.
func TestHandleEnrichment_CtxCancelRollsBack(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	origName := "Keep Me Pristine"
	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: origName, Language: "en", IsPrimary: true}})

	pipeline := matching.NewPipeline(nil, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, nil, nil, nil, nil, t.TempDir(), titleSvc, db)

	payload := service.EnrichmentPayload{
		TitleID:   id,
		TitleName: origName,
		Year:      2024,
		TitleType: model.TitleTypeMovie,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	taskID := testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before processing → BeginTx must fail
	worker.ProcessTask(ctx, queued[0])

	after, err := titles.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, model.MatchStatusUnconfirmed, after.MatchStatus,
		"match_status must not be overwritten when tx aborts")
	assert.Nil(t, after.MatchSource,
		"match_source must stay empty when tx aborts")

	task, err := tasks.GetByID(taskID)
	require.NoError(t, err)
	assert.NotEmpty(t, task.LastError, "task should record the cancellation error")
}

// TestProcessTask_AniListPushSeason_DispatchesToPusher verifies that the
// worker decodes an anilist_push_season payload and routes it to
// AniListPusher.PushSeasonState. A successful push deletes the task row
// (Complete) so queue hygiene is part of the contract under test.
func TestProcessTask_AniListPushSeason_DispatchesToPusher(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	pipeline := matching.NewPipeline(nil, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, nil, nil, nil, nil, t.TempDir(), titleSvc, db)
	fake := &fakeAniListPusher{}
	worker.SetAniListPush(fake)

	raw, err := json.Marshal(service.AniListPushSeasonPayload{SeasonID: 42})
	require.NoError(t, err)
	taskID := testutil.EnqueueTask(t, db, model.TaskTypeAniListPushSeason, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	assert.Equal(t, []int64{42}, fake.seasonCalls)
	assert.Empty(t, fake.movieCalls)

	_, err = tasks.GetByID(taskID)
	assert.Error(t, err, "successful task should be deleted after ProcessTask")
}

// TestProcessTask_AniListPushMovie_DispatchesToPusher mirrors the season
// test for the movie variant: distinct task kind, distinct payload type,
// distinct pusher method.
func TestProcessTask_AniListPushMovie_DispatchesToPusher(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	pipeline := matching.NewPipeline(nil, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, nil, nil, nil, nil, t.TempDir(), titleSvc, db)
	fake := &fakeAniListPusher{}
	worker.SetAniListPush(fake)

	raw, err := json.Marshal(service.AniListPushMoviePayload{TitleID: 77})
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeAniListPushMovie, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	assert.Equal(t, []int64{77}, fake.movieCalls)
	assert.Empty(t, fake.seasonCalls)
}

// TestProcessTask_AniListPushSeason_NoPusherFailsTask ensures a task for an
// unconfigured worker fails gracefully (so the bookkeeper retries later)
// instead of panicking on a nil interface deref.
func TestProcessTask_AniListPushSeason_NoPusherFailsTask(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)
	pipeline := matching.NewPipeline(nil, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, nil, nil, nil, nil, t.TempDir(), titleSvc, db)
	// Deliberately no SetAniListPush — simulates a test env without AniList.

	raw, err := json.Marshal(service.AniListPushSeasonPayload{SeasonID: 1})
	require.NoError(t, err)
	taskID := testutil.EnqueueTask(t, db, model.TaskTypeAniListPushSeason, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	worker.ProcessTask(context.Background(), queued[0])

	task, err := tasks.GetByID(taskID)
	require.NoError(t, err)
	require.NotNil(t, task.LastError)
	assert.Contains(t, *task.LastError, "anilist push service not configured",
		fmt.Sprintf("got: %v", task.LastError))
}

// newTaskTMDBMock spins a httptest server returning a TV details payload with
// a poster path and a fake image at /image/{poster}, so the worker exercises
// the real GetTVDetails → DownloadCover → persist sequence end-to-end.
func newTaskTMDBMock(t *testing.T, tmdbID int64, posterPath string) *matching.TMDBClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/tv/%d", tmdbID), func(w http.ResponseWriter, _ *http.Request) {
		p := posterPath
		_ = json.NewEncoder(w).Encode(matching.TMDBTVDetails{
			ID:         tmdbID,
			Name:       "Test Series",
			Status:     "Returning Series",
			PosterPath: &p,
		})
	})
	mux.HandleFunc("/image/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("fake-image-bytes"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := matching.NewTMDBClient("test-key")
	client.SetBaseURL(server.URL)
	return client
}

// TestHandleRefresh_TVSeries_PersistsCoverURL covers the SESSION-12 path:
// a refresh task for a TV title with a TMDB ID and no cover must fetch the
// poster, write it to the covers dir, and persist titles.cover_url. A
// regression here is silent — the cover never appears in the UI but no
// error surfaces in logs.
func TestHandleRefresh_TVSeries_PersistsCoverURL(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)
	tmdbID := int64(1399)
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2008,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		TMDBID:      &tmdbID,
	}, []model.TitleName{{Name: "Breaking Bad", Language: "en", IsPrimary: true}})

	dataDir := t.TempDir()
	tmdb := newTaskTMDBMock(t, tmdbID, "/poster.jpg")
	pipeline := matching.NewPipeline(nil, nil, nil, nil, dataDir)
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, tmdb, nil, nil, nil, dataDir, titleSvc, db)
	worker.SetCovers(service.NewCoverService(db, titles, tmdb, nil, dataDir))

	raw, err := json.Marshal(service.RefreshPayload{TitleID: titleID})
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeRefresh, string(raw), nil)
	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)

	worker.ProcessTask(context.Background(), queued[0])

	got, err := titles.GetByID(titleID)
	require.NoError(t, err)
	require.NotNil(t, got.CoverURL, "TMDB poster must land in titles.cover_url after handleRefresh")
	assert.Equal(t, "poster.jpg", *got.CoverURL)

	// And the file must actually exist on disk under {dataDir}/covers/.
	written, err := os.ReadFile(filepath.Join(dataDir, "covers", *got.CoverURL))
	require.NoError(t, err)
	assert.Equal(t, "fake-image-bytes", string(written))
}

// TestHandleCoverFetch_TVSeries_PersistsCoverURL covers the second untested
// task type. The cover_fetch payload is what `enrichment` enqueues after
// matching when no cover came down with the metadata, so this path is on the
// critical "newly added title" timeline.
func TestHandleCoverFetch_TVSeries_PersistsCoverURL(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)
	tmdbID := int64(1399)
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2008,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		TMDBID:      &tmdbID,
	}, []model.TitleName{{Name: "Breaking Bad", Language: "en", IsPrimary: true}})

	dataDir := t.TempDir()
	tmdb := newTaskTMDBMock(t, tmdbID, "/cover.jpg")
	pipeline := matching.NewPipeline(nil, nil, nil, nil, dataDir)
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, tmdb, nil, nil, nil, dataDir, titleSvc, db)
	worker.SetCovers(service.NewCoverService(db, titles, tmdb, nil, dataDir))

	raw, err := json.Marshal(service.CoverFetchPayload{
		TitleID:   titleID,
		TMDBID:    tmdbID,
		TitleType: model.TitleTypeSeries,
	})
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeCoverFetch, string(raw), nil)
	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)

	worker.ProcessTask(context.Background(), queued[0])

	got, err := titles.GetByID(titleID)
	require.NoError(t, err)
	require.NotNil(t, got.CoverURL, "TMDB poster must land in titles.cover_url after handleCoverFetch")
	assert.Equal(t, "cover.jpg", *got.CoverURL)

	written, err := os.ReadFile(filepath.Join(dataDir, "covers", *got.CoverURL))
	require.NoError(t, err)
	assert.Equal(t, "fake-image-bytes", string(written))
}

// ----------------------------------------------------------------------------
// isSearchSource / resolvedName unit tests
// ----------------------------------------------------------------------------

func TestIsSearchSource(t *testing.T) {
	assert.True(t, service.IsSearchSourceForTest(matching.MatchSourceTMDBSearch))
	assert.True(t, service.IsSearchSourceForTest(matching.MatchSourceAniListSearch))
	assert.True(t, service.IsSearchSourceForTest(matching.MatchSourceGeminiFuzzy))
	assert.False(t, service.IsSearchSourceForTest(matching.MatchSourcePlexIDs))
	assert.False(t, service.IsSearchSourceForTest(matching.MatchSourceCrossRef))
	assert.False(t, service.IsSearchSourceForTest(matching.MatchSourceManual))
	assert.False(t, service.IsSearchSourceForTest(matching.MatchSourceNone))
	assert.False(t, service.IsSearchSourceForTest(""))
}

func TestResolvedName(t *testing.T) {
	t.Run("returns primary name", func(t *testing.T) {
		result := &matching.MatchResult{
			Names: []model.TitleName{
				{Name: "Secondary", Language: "fr", IsPrimary: false},
				{Name: "Primary Title", Language: "en", IsPrimary: true},
			},
		}
		got := service.ResolvedNameForTest(result, service.EnrichmentPayload{TitleName: "Original"})
		assert.Equal(t, "Primary Title", got)
	})

	t.Run("falls back to first name when no primary", func(t *testing.T) {
		result := &matching.MatchResult{
			Names: []model.TitleName{
				{Name: "First", Language: "en", IsPrimary: false},
				{Name: "Second", Language: "fr", IsPrimary: false},
			},
		}
		got := service.ResolvedNameForTest(result, service.EnrichmentPayload{TitleName: "Original"})
		assert.Equal(t, "First", got)
	})

	t.Run("falls back to payload name when names empty", func(t *testing.T) {
		result := &matching.MatchResult{}
		got := service.ResolvedNameForTest(result, service.EnrichmentPayload{TitleName: "Original"})
		assert.Equal(t, "Original", got)
	})

	t.Run("returns last primary when multiple primaries exist", func(t *testing.T) {
		result := &matching.MatchResult{
			Names: []model.TitleName{
				{Name: "Original Plex Name", Language: "en", IsPrimary: true},
				{Name: "TMDB Resolved Name", Language: "en", IsPrimary: true},
			},
		}
		got := service.ResolvedNameForTest(result, service.EnrichmentPayload{TitleName: "fallback"})
		assert.Equal(t, "TMDB Resolved Name", got)
	})
}

// matchEventsForTitle queries match_events rows for the given title and kind directly.
func matchEventsForTitle(t *testing.T, db *sql.DB, titleID int64, kind model.MatchEventKind) []model.MatchEvent {
	t.Helper()
	rows, err := db.QueryContext(context.Background(),
		`SELECT id, title_id, kind, detail FROM match_events WHERE title_id = ? AND kind = ?`,
		titleID, kind,
	)
	require.NoError(t, err)
	defer rows.Close()
	var events []model.MatchEvent
	for rows.Next() {
		var ev model.MatchEvent
		require.NoError(t, rows.Scan(&ev.ID, &ev.TitleID, &ev.Kind, &ev.Detail))
		events = append(events, ev)
	}
	require.NoError(t, rows.Err())
	return events
}

// newTMDBSearchMock returns an httptest-backed TMDBClient that answers search
// queries with a single confirmed result, so the pipeline takes the tmdb_search
// branch and sets MatchStatusConfirmed.
func newTMDBSearchMock(t *testing.T, tmdbID int64, resolvedTitle string) *matching.TMDBClient {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/search/movie", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": tmdbID, "title": resolvedTitle, "release_date": "2024-01-01"},
			},
		})
	})
	mux.HandleFunc("/search/tv", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": tmdbID, "name": resolvedTitle, "first_air_date": "2024-01-01"},
			},
		})
	})
	mux.HandleFunc(fmt.Sprintf("/movie/%d", tmdbID), func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(matching.TMDBMovieDetails{
			ID:    tmdbID,
			Title: resolvedTitle,
		})
	})
	mux.HandleFunc(fmt.Sprintf("/tv/%d", tmdbID), func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(matching.TMDBTVDetails{
			ID:   tmdbID,
			Name: resolvedTitle,
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := matching.NewTMDBClient("test-key")
	client.SetBaseURL(server.URL)
	return client
}

// newGeminiHighConfidenceMock returns an httptest-backed GeminiClient that
// always responds with confirmed=true, confidence=high so the pipeline sets
// MatchStatusConfirmed for a tmdb_search result.
func newGeminiHighConfidenceMock(t *testing.T) *matching.GeminiClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Gemini wraps the JSON answer inside its "candidates" envelope.
		payload := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": `{"confirmed": true, "confidence": "high", "reason": "exact match"}`},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(server.Close)

	client := matching.NewGeminiClient([]string{"test-key"})
	client.SetBaseURL(server.URL)
	return client
}

// TestHandleEnrichment_SearchMatch_WritesAutoConfirmedEvent verifies that an
// enrichment whose pipeline result is MatchStatusConfirmed via a search source
// (tmdb_search) produces exactly one match_events row with kind=auto_confirmed
// whose detail contains both the original and resolved names.
func TestHandleEnrichment_SearchMatch_WritesAutoConfirmedEvent(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	const originalName = "Alien Movie Original"
	const resolvedTitle = "Alien: Romulus"
	const tmdbID = int64(945961)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: originalName, Language: "en", IsPrimary: true}})

	tmdb := newTMDBSearchMock(t, tmdbID, resolvedTitle)
	gemini := newGeminiHighConfidenceMock(t)
	pipeline := matching.NewPipeline(tmdb, nil, gemini, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, tmdb, nil, nil, nil, t.TempDir(), titleSvc, db)

	raw, err := json.Marshal(service.EnrichmentPayload{
		TitleID:   id,
		TitleName: originalName,
		Year:      2024,
		TitleType: model.TitleTypeMovie,
		// No TMDB/IMDB pre-set — forces the search path.
	})
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	// Verify the title was confirmed via TMDB search.
	got, err := titles.GetByID(id)
	require.NoError(t, err)
	require.Equal(t, model.MatchStatusConfirmed, got.MatchStatus)
	require.NotNil(t, got.MatchSource)
	assert.Equal(t, matching.MatchSourceTMDBSearch, *got.MatchSource)

	// Verify exactly one auto_confirmed event with correct detail.
	events := matchEventsForTitle(t, db, id, model.MatchEventAutoConfirmed)
	require.Len(t, events, 1, "search match must produce exactly one auto_confirmed event")
	assert.Contains(t, events[0].Detail, originalName,
		"event detail must contain the original payload name")
	assert.Contains(t, events[0].Detail, resolvedTitle,
		"event detail must contain the resolved primary name")
}

// TestHandleEnrichment_PlexIDsMatch_NoEvent ensures that an enrichment
// confirmed via plex_ids (ID-based, no search decision) does NOT write a
// match_events row.
func TestHandleEnrichment_PlexIDsMatch_NoEvent(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Known Movie", Language: "en", IsPrimary: true}})

	// Supplying TMDBID makes the pipeline take the plex_ids branch (MatchSourcePlexIDs).
	pipeline := matching.NewPipeline(nil, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, nil, nil, nil, nil, t.TempDir(), titleSvc, db)

	raw, err := json.Marshal(service.EnrichmentPayload{
		TitleID:   id,
		TitleName: "Known Movie",
		Year:      2024,
		TitleType: model.TitleTypeMovie,
		TMDBID:    42, // pre-supplied ID → plex_ids branch, MatchSourcePlexIDs
	})
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	got, err := titles.GetByID(id)
	require.NoError(t, err)
	require.Equal(t, model.MatchStatusConfirmed, got.MatchStatus)
	require.NotNil(t, got.MatchSource)
	assert.Equal(t, matching.MatchSourcePlexIDs, *got.MatchSource)

	events := matchEventsForTitle(t, db, id, model.MatchEventAutoConfirmed)
	assert.Empty(t, events, "plex_ids match must not write an auto_confirmed event")
}

// TestHandleEnrichment_PreserveMatch_NoEvent ensures that an enrichment run
// with PreserveMatch=true (manual-edit path) does not write a match_events row.
func TestHandleEnrichment_PreserveMatch_NoEvent(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	const tmdbID = int64(945961)
	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Manually Matched Movie", Language: "en", IsPrimary: true}})

	tmdb := newTMDBSearchMock(t, tmdbID, "Alien: Romulus")
	gemini := newGeminiHighConfidenceMock(t)
	pipeline := matching.NewPipeline(tmdb, nil, gemini, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, tmdb, nil, nil, nil, t.TempDir(), titleSvc, db)

	raw, err := json.Marshal(service.EnrichmentPayload{
		TitleID:       id,
		TitleName:     "Manually Matched Movie",
		Year:          2024,
		TitleType:     model.TitleTypeMovie,
		PreserveMatch: true, // manual-edit path — must never write an event
	})
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	events := matchEventsForTitle(t, db, id, model.MatchEventAutoConfirmed)
	assert.Empty(t, events, "PreserveMatch=true must not write an auto_confirmed event")
}

// ----------------------------------------------------------------------------
// decideSeasonAction — franchise-protection rule table (pure, no infra)
// ----------------------------------------------------------------------------

func TestDecideSeasonAction(t *testing.T) {
	seasonChain := func(season int, isRoot, rootIsSeries bool) *matching.SeasonChain {
		return &matching.SeasonChain{
			RootID:       1000,
			RootTitle:    "Root Show",
			SeasonNumber: season,
			IsRoot:       isRoot,
			RootIsSeries: rootIsSeries,
		}
	}
	titleAt := func(id int64) *model.Title { return &model.Title{ID: id} }

	tests := []struct {
		name         string
		chain        *matching.SeasonChain
		result       *matching.MatchResult
		parentByIDs  *model.Title
		parentByRoot *model.Title
		wantKind     int
		wantParent   int64
		wantOffset   int
	}{
		{
			name:     "nil chain → legacy",
			chain:    nil,
			result:   &matching.MatchResult{IMDBID: "tt1"},
			wantKind: service.SeasonActionLegacyForTest,
		},
		{
			name:       "IsRoot → legacyRoot offset 0",
			chain:      seasonChain(1, true, true),
			result:     &matching.MatchResult{IMDBID: "tt1"},
			wantKind:   service.SeasonActionLegacyRootForTest,
			wantOffset: 0,
		},
		{
			name:        "S2, parentByIDs found → mergeInto offset 1",
			chain:       seasonChain(2, false, true),
			result:      &matching.MatchResult{IMDBID: "tt-parent"},
			parentByIDs: titleAt(7),
			wantKind:    service.SeasonActionMergeIntoForTest,
			wantParent:  7,
			wantOffset:  1,
		},
		{
			name:         "S2, own imdb + only parentByRoot → none (id-conflict protection)",
			chain:        seasonChain(2, false, true),
			result:       &matching.MatchResult{IMDBID: "tt-own"},
			parentByRoot: titleAt(9),
			wantKind:     service.SeasonActionNoneForTest,
		},
		{
			name:         "S2, no ids + parentByRoot → mergeInto offset 1",
			chain:        seasonChain(2, false, true),
			result:       &matching.MatchResult{},
			parentByRoot: titleAt(9),
			wantKind:     service.SeasonActionMergeIntoForTest,
			wantParent:   9,
			wantOffset:   1,
		},
		{
			name:       "S2, no ids, no parent → createRoot offset 1",
			chain:      seasonChain(2, false, true),
			result:     &matching.MatchResult{},
			wantKind:   service.SeasonActionCreateRootForTest,
			wantOffset: 1,
		},
		{
			name:     "S3, own imdb, no parent → none",
			chain:    seasonChain(3, false, true),
			result:   &matching.MatchResult{IMDBID: "tt-own"},
			wantKind: service.SeasonActionNoneForTest,
		},
		{
			name:         "S2, RootIsSeries=false → none (movie root)",
			chain:        seasonChain(2, false, false),
			result:       &matching.MatchResult{},
			parentByRoot: titleAt(9),
			wantKind:     service.SeasonActionNoneForTest,
		},
		{
			// TVDB-only identity: result has TVDBID but no IMDb/TMDB id.
			// parentByIDs=nil (TVDB lookup returned nothing), parentByRoot has a
			// hit. Relations-only evidence must not override an entry's own
			// external identity — expect none.
			name:         "S2, TVDB-only identity + parentByRoot → none (id-conflict protection)",
			chain:        seasonChain(2, false, true),
			result:       &matching.MatchResult{TVDBID: 222},
			parentByRoot: titleAt(9),
			wantKind:     service.SeasonActionNoneForTest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := service.DecideSeasonActionForTest(tc.chain, tc.result, tc.parentByIDs, tc.parentByRoot)
			assert.Equal(t, tc.wantKind, got.Kind, "kind")
			assert.Equal(t, tc.wantParent, got.ParentID, "parent id")
			assert.Equal(t, tc.wantOffset, got.Offset, "offset")
		})
	}
}

// newAniListChainMock returns an AniListClient whose GraphQL endpoint answers
// BOTH the relations query (used by ResolveSeasonChain) and the anime-details
// query (used during enrichment). The two share the /graphql POST shape and an
// "id" variable; we branch on whether the query string mentions "relations".
// media maps an AniList id to its {format, english title, prequel id (0=none)}.
func newAniListChainMock(t *testing.T, media map[int64]struct {
	Format  string
	English string
	Prequel int64
}) *matching.AniListClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		idRaw, ok := req.Variables["id"].(float64)
		if !ok {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		m, found := media[int64(idRaw)]
		if !found {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		node := map[string]any{
			"id":     int64(idRaw),
			"format": m.Format,
			"title":  map[string]any{"english": m.English, "romaji": m.English},
		}
		if strings.Contains(req.Query, "relations") {
			edges := []any{}
			if m.Prequel != 0 {
				edges = append(edges, map[string]any{
					"relationType": "PREQUEL",
					"node": map[string]any{
						"id":     m.Prequel,
						"type":   "ANIME",
						"format": media[m.Prequel].Format,
						"title":  map[string]any{},
					},
				})
			}
			node["relations"] = map[string]any{"edges": edges}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"Media": node},
		})
	}))
	t.Cleanup(server.Close)
	return matching.NewAniListClientWithURL(server.URL)
}

// TestHandleEnrichment_AutoAttachesSeasonBySharedIMDb is the integration-flavored
// test for Task 7: an anime entry whose AniList id resolves to "season 2 of root
// X", with the parent series already present locally under the same (parent)
// IMDb id. After ProcessTask the source title is merged into the parent (seasons
// shifted by the season offset), and exactly one season_attached match_event is
// written on the parent with a detail mentioning "Season 2".
func TestHandleEnrichment_AutoAttachesSeasonBySharedIMDb(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	tasks := repository.NewTaskRepository(db)

	const sharedIMDB = "tt-root-show"
	const seasonAniList = int64(20)

	// Parent series already present, carrying the shared (parent) IMDb id.
	imdb := sharedIMDB
	parentID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2013,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		IMDBID:      &imdb,
	}, []model.TitleName{{Name: "Root Show", Language: "en", IsPrimary: true}})

	// Source = the season-2 entry, also carrying the shared (parent) IMDb id
	// (Simkl season entries inherit the parent show's id) plus its AniList id.
	srcIMDB := sharedIMDB
	srcAniList := seasonAniList
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2017,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		IMDBID:      &srcIMDB,
		AniListID:   &srcAniList,
	}, []model.TitleName{{Name: "Root Show Season 2", Language: "en", IsPrimary: true}})

	// Seed a season on the source so the merge has something to shift.
	// season_number=1 on the source will become season_number=2 on the parent
	// after the offset of 1 is applied.
	srcSeasonID := testutil.InsertSeason(t, db, sourceID, 1)
	testutil.GetOrCreateEpisode(t, db, srcSeasonID, 1)

	// AniList: 20 (TV) → PREQUEL 10 (TV, root). Resolving 20 yields season 2.
	anilist := newAniListChainMock(t, map[int64]struct {
		Format  string
		English string
		Prequel int64
	}{
		10: {Format: "TV", English: "Root Show", Prequel: 0},
		20: {Format: "TV", English: "Root Show Season 2", Prequel: 10},
	})

	pipeline := matching.NewPipeline(nil, anilist, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, pipeline, nil, anilist, nil, nil, t.TempDir(), titleSvc, db)

	raw, err := json.Marshal(service.EnrichmentPayload{
		TitleID:   sourceID,
		TitleName: "Root Show Season 2",
		Year:      2017,
		TitleType: model.TitleTypeSeries,
		IsAnime:   true,
		IMDBID:    sharedIMDB,
		AniListID: seasonAniList,
		// Lock IMDb so enrichment doesn't try to re-resolve it via TMDB (no client).
		LockedIDs: []string{service.LockIMDB},
	})
	require.NoError(t, err)
	testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, string(raw), nil)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	// Source title consumed by the merge.
	_, err = titles.GetByID(sourceID)
	assert.Error(t, err, "source season title must be deleted after merge into parent")

	// Exactly one season_attached event on the parent, mentioning Season 2.
	events := matchEventsForTitle(t, db, parentID, model.MatchEventSeasonAttached)
	require.Len(t, events, 1, "merge must write exactly one season_attached event on the parent")
	assert.Contains(t, events[0].Detail, "Season 2",
		"event detail must name the attached season ordinal")

	// No season-backfill refresh enqueued for the consumed source.
	assert.Nil(t, pendingRefreshFor(t, tasks, sourceID),
		"a merged-away source must not enqueue a season-backfill refresh")

	// Offset applied: the source's season 1 must now live as season 2 on the parent.
	var seasonCount int
	err = db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM seasons WHERE title_id = ? AND season_number = 2`,
		parentID,
	).Scan(&seasonCount)
	require.NoError(t, err)
	assert.Equal(t, 1, seasonCount,
		"source season 1 must be re-numbered to season 2 on the parent after merge with offset 1")
}
