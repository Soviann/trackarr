package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePushNotifier records SendNotification calls so refresh tests can
// assert the series-ended notification was triggered exactly when expected.
// The other PushNotifier methods are no-ops — the worker never invokes them
// during a refresh.
type fakePushNotifier struct {
	calls []pushCall
}

type pushCall struct{ Title, Body, URL string }

func (f *fakePushNotifier) Subscribe(_ context.Context, _ string) error { return nil }
func (f *fakePushNotifier) Unsubscribe(_ context.Context) error         { return nil }
func (f *fakePushNotifier) HasSubscription() bool                       { return true }
func (f *fakePushNotifier) SendNotification(_ context.Context, t, b, u string) error {
	f.calls = append(f.calls, pushCall{t, b, u})
	return nil
}

// newRefreshTMDBMock spins a httptest server returning canned TMDB responses
// for one TV title. Status drives the series-ended detection branch; seasons
// is the list mocked under /tv/{id}/season/{n}. Returns a TMDB client already
// pointed at the server (cleanup auto-registered).
func newRefreshTMDBMock(t *testing.T, tmdbID int64, status string) *matching.TMDBClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/tv/%d", tmdbID), func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(matching.TMDBTVDetails{
			ID:     tmdbID,
			Name:   "Test Series",
			Status: status,
			// No PosterPath → skips DownloadCover branch (kept out of the
			// status-change tests; cover persistence is exercised in SESSION-12).
			Seasons: nil,
		})
	})
	// Catch-all returns 404 so any unexpected fetch surfaces as a clear failure
	// rather than a hang or zero-value decode.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := matching.NewTMDBClient("test-key")
	client.SetBaseURL(server.URL)
	return client
}

// newBackgroundServiceWithTMDB mirrors setupBackgroundService but plugs in a
// TMDB client + a recording PushNotifier so the SESSION-11 path can be
// exercised without HTTP outside the test process.
func newBackgroundServiceWithTMDB(t *testing.T, tmdb *matching.TMDBClient) (*service.BackgroundService, *sql.DB, *repository.TitleRepository, *fakePushNotifier) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	push := &fakePushNotifier{}
	coverSvc := service.NewCoverService(db, titleRepo, nil, nil, t.TempDir())
	svc := service.NewBackgroundService(db, titleRepo, settingRepo, tmdb, coverSvc, push)
	return svc, db, titleRepo, push
}

// fakeAniListSeasonScoreClient stubs the GetAnimeDetails call so the
// per-season score refresh can be exercised without a real HTTP server.
// scoreByID maps an AniList media id to the averageScore the fake should
// return; errByID overrides the response with an error for that id.
type fakeAniListSeasonScoreClient struct {
	scoreByID map[int64]int
	errByID   map[int64]error
	calls     []int64
}

func (f *fakeAniListSeasonScoreClient) GetAnimeDetails(_ context.Context, id int64) (*matching.AniListDetails, error) {
	f.calls = append(f.calls, id)
	if err, ok := f.errByID[id]; ok {
		return nil, err
	}
	if score, ok := f.scoreByID[id]; ok {
		s := score
		return &matching.AniListDetails{ID: id, AverageScore: &s}, nil
	}
	return &matching.AniListDetails{ID: id}, nil
}

func setupBackgroundService(t *testing.T) (*service.BackgroundService, *sql.DB, *repository.TitleRepository, *repository.SeasonRepository, *repository.EpisodeRepository) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	settingRepo := repository.NewSettingRepository(db)

	pushSvc := service.NewPushService(db, settingRepo, "pub", "priv", "mailto:test@test.com")
	// No external API clients in tests — nil TMDB/AniList
	coverSvc := service.NewCoverService(db, titleRepo, nil, nil, t.TempDir())
	svc := service.NewBackgroundService(db, titleRepo, settingRepo, nil, coverSvc, pushSvc)
	return svc, db, titleRepo, seasonRepo, episodeRepo
}

func TestBackgroundService_DetectCompletedSeries(t *testing.T) {
	svc, db, titleRepo, _, _ := setupBackgroundService(t)

	// Create a series with status=watching, series_status=ended
	ended := model.SeriesStatusEnded
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:         model.TitleTypeSeries,
		Year:         2023,
		Status:       model.TitleStatusWatching,
		MatchStatus:  model.MatchStatusConfirmed,
		SeriesStatus: &ended,
	}, []model.TitleName{{Name: "Shogun", Language: "en", IsPrimary: true}})

	// Create 1 season with 2 episodes, all watched
	season := testutil.GetOrCreateSeason(t, db, titleID, 1)
	testutil.UpdateSeasonTotalEpisodes(t, db, season.ID, 2)

	ep1 := testutil.GetOrCreateEpisode(t, db, season.ID, 1)
	ep2 := testutil.GetOrCreateEpisode(t, db, season.ID, 2)
	now := time.Now().UTC()
	testutil.MarkEpisodeWatched(t, db, ep1.ID, now)
	testutil.MarkEpisodeWatched(t, db, ep2.ID, now)

	// Run refresh
	results := svc.RefreshTitles(context.Background())

	// Title should be auto-completed
	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusCompleted, title.Status)
	assert.Len(t, results, 1)
	assert.True(t, results[0].AutoCompleted)
}

func TestBackgroundService_SkipCompletedTitles(t *testing.T) {
	svc, db, _, _, _ := setupBackgroundService(t)

	// Create a completed title
	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})

	results := svc.RefreshTitles(context.Background())
	assert.Empty(t, results)
}

func TestBackgroundService_NoAutoCompleteIfUnwatchedEpisodes(t *testing.T) {
	svc, db, titleRepo, _, _ := setupBackgroundService(t)

	ended := model.SeriesStatusEnded
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:         model.TitleTypeSeries,
		Year:         2023,
		Status:       model.TitleStatusWatching,
		MatchStatus:  model.MatchStatusConfirmed,
		SeriesStatus: &ended,
	}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	season := testutil.GetOrCreateSeason(t, db, titleID, 1)
	testutil.UpdateSeasonTotalEpisodes(t, db, season.ID, 2)

	ep1 := testutil.GetOrCreateEpisode(t, db, season.ID, 1)
	_ = testutil.GetOrCreateEpisode(t, db, season.ID, 2) // ep2 unwatched
	testutil.MarkEpisodeWatched(t, db, ep1.ID, time.Now().UTC())

	results := svc.RefreshTitles(context.Background())

	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusWatching, title.Status)
	assert.Len(t, results, 1)
	assert.False(t, results[0].AutoCompleted)
}

func TestBackgroundService_NilSafe(t *testing.T) {
	var svc *service.BackgroundService
	results := svc.RefreshTitles(context.Background())
	assert.Nil(t, results)
}

func TestBackgroundService_RefreshCancelledContextStopsLoop(t *testing.T) {
	svc, db, _, _, _ := setupBackgroundService(t)

	for i := 0; i < 10; i++ {
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2023,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
		}, []model.TitleName{{Name: fmt.Sprintf("Movie %d", i), Language: "en", IsPrimary: true}})
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	results := svc.RefreshTitles(ctx)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, time.Second, "refresh should stop within 1s after cancel")
	assert.Less(t, len(results), 10, "not all titles should be processed after cancel")
}

// readSeasonAniListScore reads the persisted anilist_average_score column
// directly so the test asserts on DB state, not method return values.
func readSeasonAniListScore(t *testing.T, db *sql.DB, seasonID int64) *int {
	t.Helper()
	var score sql.NullInt64
	require.NoError(t, db.QueryRow(`SELECT anilist_average_score FROM seasons WHERE id = ?`, seasonID).Scan(&score))
	if !score.Valid {
		return nil
	}
	v := int(score.Int64)
	return &v
}

func TestBackgroundService_RefreshAniListScores_PersistsPerSeason(t *testing.T) {
	svc, db, _, _, _ := setupBackgroundService(t)

	titleID := testutil.InsertTitle(t, db, "Jujutsu Kaisen", true)
	s1 := testutil.InsertSeason(t, db, titleID, 1)
	s2 := testutil.InsertSeason(t, db, titleID, 2)
	testutil.InsertSeasonExternalID(t, db, s1, "anilist", "113415")
	testutil.InsertSeasonExternalID(t, db, s2, "anilist", "145064")

	fake := &fakeAniListSeasonScoreClient{
		scoreByID: map[int64]int{113415: 86, 145064: 88},
	}
	svc.SetAniList(fake)

	results := svc.RefreshTitles(context.Background())
	require.Len(t, results, 1)

	assert.ElementsMatch(t, []int64{113415, 145064}, fake.calls)
	got1 := readSeasonAniListScore(t, db, s1)
	got2 := readSeasonAniListScore(t, db, s2)
	require.NotNil(t, got1)
	require.NotNil(t, got2)
	assert.Equal(t, 86, *got1)
	assert.Equal(t, 88, *got2)
}

func TestBackgroundService_RefreshAniListScores_SkipsWhenTokenInvalid(t *testing.T) {
	svc, db, _, _, _ := setupBackgroundService(t)

	titleID := testutil.InsertTitle(t, db, "Anime", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "100")
	testutil.SetSetting(t, db, "anilist_token_invalid", "true")

	fake := &fakeAniListSeasonScoreClient{scoreByID: map[int64]int{100: 90}}
	svc.SetAniList(fake)

	_ = svc.RefreshTitles(context.Background())
	assert.Empty(t, fake.calls, "no AniList call may be issued while the token is flagged invalid")
	assert.Nil(t, readSeasonAniListScore(t, db, seasonID))
}

func TestBackgroundService_RefreshAniListScores_ContinuesOnTransientError(t *testing.T) {
	svc, db, _, _, _ := setupBackgroundService(t)

	titleID := testutil.InsertTitle(t, db, "Anime", true)
	s1 := testutil.InsertSeason(t, db, titleID, 1)
	s2 := testutil.InsertSeason(t, db, titleID, 2)
	testutil.InsertSeasonExternalID(t, db, s1, "anilist", "1")
	testutil.InsertSeasonExternalID(t, db, s2, "anilist", "2")

	fake := &fakeAniListSeasonScoreClient{
		errByID:   map[int64]error{1: fmt.Errorf("network glitch")},
		scoreByID: map[int64]int{2: 77},
	}
	svc.SetAniList(fake)

	_ = svc.RefreshTitles(context.Background())

	// Token must NOT be flagged invalid for non-401 errors.
	got, _ := testutil.GetSetting(t, db, "anilist_token_invalid")
	assert.NotEqual(t, "true", got)

	// S1 score stays nil, S2 was still persisted despite S1's failure.
	assert.Nil(t, readSeasonAniListScore(t, db, s1))
	require.NotNil(t, readSeasonAniListScore(t, db, s2))
	assert.Equal(t, 77, *readSeasonAniListScore(t, db, s2))
}

func TestBackgroundService_RefreshAniListScores_SkipsNonAnimeTitles(t *testing.T) {
	svc, db, _, _, _ := setupBackgroundService(t)

	titleID := testutil.InsertTitle(t, db, "Live action", false)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	// Stale mapping (shouldn't normally happen on a non-anime title) — proves
	// the IsAnime guard is what keeps us from calling AniList here.
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "999")

	fake := &fakeAniListSeasonScoreClient{}
	svc.SetAniList(fake)

	_ = svc.RefreshTitles(context.Background())
	assert.Empty(t, fake.calls, "non-anime titles must not trigger AniList per-season fetches")
}

// createSeriesWithTMDBID is a tiny shim around testutil.CreateTitle that bakes
// in the fields every refreshSeriesFromTMDB test needs: confirmed match,
// watching status, TMDB ID set, and an initial SeriesStatus.
func createSeriesWithTMDBID(t *testing.T, db *sql.DB, name string, tmdbID int64, initial model.SeriesStatus) int64 {
	t.Helper()
	return testutil.CreateTitle(t, db, &model.Title{
		Type:         model.TitleTypeSeries,
		Year:         2008,
		Status:       model.TitleStatusWatching,
		MatchStatus:  model.MatchStatusConfirmed,
		TMDBID:       &tmdbID,
		SeriesStatus: &initial,
	}, []model.TitleName{{Name: name, Language: "en", IsPrimary: true}})
}

// TestBackgroundService_RefreshSeries_StatusChangeFiresPush is the canonical
// SESSION-11 test: a series that flips from "Returning Series" to "Ended" in
// TMDB must trigger exactly one push notification with the title-detail URL.
// The auto-complete branch also runs in the same iteration (no episodes →
// nothing unwatched), giving us a free regression on that branch too.
func TestBackgroundService_RefreshSeries_StatusChangeFiresPush(t *testing.T) {
	tmdb := newRefreshTMDBMock(t, 1399, "Ended")
	svc, db, titleRepo, push := newBackgroundServiceWithTMDB(t, tmdb)

	titleID := createSeriesWithTMDBID(t, db, "Breaking Bad", 1399, model.SeriesStatusReturning)

	results := svc.RefreshTitles(context.Background())
	require.Len(t, results, 1)
	assert.True(t, results[0].StatusChanged)
	assert.Equal(t, model.SeriesStatusReturning, results[0].OldStatus)
	assert.Equal(t, model.SeriesStatusEnded, results[0].NewStatus)

	require.Len(t, push.calls, 1, "exactly one series-ended push expected")
	assert.Equal(t, "PlexTracker", push.calls[0].Title)
	assert.Contains(t, push.calls[0].Body, "Breaking Bad")
	assert.Contains(t, push.calls[0].Body, "Series ended")
	assert.Equal(t, fmt.Sprintf("/title/%d", titleID), push.calls[0].URL)

	got, err := titleRepo.GetByID(titleID)
	require.NoError(t, err)
	require.NotNil(t, got.SeriesStatus)
	assert.Equal(t, model.SeriesStatusEnded, *got.SeriesStatus)
	assert.Equal(t, model.TitleStatusCompleted, got.Status, "auto-complete must run after status flips to Ended with no unwatched episodes")
}

// TestBackgroundService_RefreshSeries_NotificationToggleSilencesPush proves
// the notif_series_ended preference is honored — the toggle is the user's
// only escape hatch from a stream of "X ended" pings, so a regression here
// is user-visible and silent.
func TestBackgroundService_RefreshSeries_NotificationToggleSilencesPush(t *testing.T) {
	tmdb := newRefreshTMDBMock(t, 1399, "Ended")
	svc, db, _, push := newBackgroundServiceWithTMDB(t, tmdb)
	testutil.SetSetting(t, db, service.NotifSeriesEnded, "false")

	createSeriesWithTMDBID(t, db, "Breaking Bad", 1399, model.SeriesStatusReturning)

	_ = svc.RefreshTitles(context.Background())
	assert.Empty(t, push.calls, "notif_series_ended=false must suppress the push")
}

// TestBackgroundService_RefreshSeries_NoStatusChangeNoPush guards against the
// other regression direction — a steady-state refresh where TMDB still says
// "Ended" must NOT fire a push. Otherwise every daily refresh would re-page
// the user about already-known endings.
func TestBackgroundService_RefreshSeries_NoStatusChangeNoPush(t *testing.T) {
	tmdb := newRefreshTMDBMock(t, 1399, "Ended")
	svc, db, _, push := newBackgroundServiceWithTMDB(t, tmdb)

	createSeriesWithTMDBID(t, db, "Breaking Bad", 1399, model.SeriesStatusEnded)

	results := svc.RefreshTitles(context.Background())
	require.Len(t, results, 1)
	assert.False(t, results[0].StatusChanged, "no status flip → no StatusChanged flag")
	assert.Empty(t, push.calls, "no status flip → no push")
}
