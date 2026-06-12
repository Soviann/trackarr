package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
