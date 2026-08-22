package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	scoreByID    map[int64]int
	errByID      map[int64]error
	detailsByID  map[int64]*matching.AniListDetails
	searchResult map[string][]matching.AniListSearchResult
	calls        []int64
}

func (f *fakeAniListSeasonScoreClient) GetAnimeDetails(_ context.Context, id int64) (*matching.AniListDetails, error) {
	f.calls = append(f.calls, id)
	if err, ok := f.errByID[id]; ok {
		return nil, err
	}
	if d, ok := f.detailsByID[id]; ok {
		return d, nil
	}
	if score, ok := f.scoreByID[id]; ok {
		s := score
		return &matching.AniListDetails{ID: id, AverageScore: &s}, nil
	}
	return &matching.AniListDetails{ID: id}, nil
}

func (f *fakeAniListSeasonScoreClient) SearchAnime(_ context.Context, query string) ([]matching.AniListSearchResult, error) {
	if res, ok := f.searchResult[query]; ok {
		return res, nil
	}
	return nil, nil
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

	_ = testutil.SeedEpisode(t, db, season.ID, 1, "2024-01-01", true)
	_ = testutil.SeedEpisode(t, db, season.ID, 2, "2024-01-08", false) // ep2 unwatched

	results := svc.RefreshTitles(context.Background())

	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusWatching, title.Status)
	assert.Len(t, results, 1)
	assert.False(t, results[0].AutoCompleted)
}

func TestBackgroundService_PlanToWatchWithWatchedEpisodesReconcilesToWatching(t *testing.T) {
	svc, db, titleRepo, _, _ := setupBackgroundService(t)

	returning := model.SeriesStatusReturning
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:         model.TitleTypeSeries,
		Year:         2023,
		Status:       model.TitleStatusPlanToWatch,
		MatchStatus:  model.MatchStatusConfirmed,
		SeriesStatus: &returning,
	}, []model.TitleName{{Name: "Returning Anime", Language: "en", IsPrimary: true}})

	s1 := testutil.GetOrCreateSeason(t, db, titleID, 1)
	_ = testutil.SeedEpisode(t, db, s1.ID, 1, "2024-01-01", true)
	s2 := testutil.GetOrCreateSeason(t, db, titleID, 2)
	_ = testutil.SeedEpisode(t, db, s2.ID, 1, "2099-01-01", false)

	svc.RefreshTitles(context.Background())

	title, err := titleRepo.GetByID(titleID)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusWatching, title.Status)
}

func TestBackgroundService_PlanToWatchCompletedEndedSeriesReconcilesToCompleted(t *testing.T) {
	svc, db, titleRepo, _, _ := setupBackgroundService(t)

	ended := model.SeriesStatusEnded
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:         model.TitleTypeSeries,
		Year:         2023,
		Status:       model.TitleStatusPlanToWatch,
		MatchStatus:  model.MatchStatusConfirmed,
		SeriesStatus: &ended,
	}, []model.TitleName{{Name: "Ended Show", Language: "en", IsPrimary: true}})

	s1 := testutil.GetOrCreateSeason(t, db, titleID, 1)
	_ = testutil.SeedEpisode(t, db, s1.ID, 1, "2024-01-01", true)

	results := svc.RefreshTitles(context.Background())

	title, err := titleRepo.GetByID(titleID)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusCompleted, title.Status)
	assert.Len(t, results, 1)
	assert.True(t, results[0].AutoCompleted)
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

// readPartScore returns the anilist_average_score persisted in
// season_external_ids for the primary (first-sorted) part of (seasonID,
// "anilist"). Returns nil if no row exists or the score is NULL.
func readPartScore(t *testing.T, db *sql.DB, seasonID int64, externalID string) *int {
	t.Helper()
	var score sql.NullInt64
	err := db.QueryRow(`
		SELECT anilist_average_score FROM season_external_ids
		WHERE season_id = ? AND provider = 'anilist' AND external_id = ?`,
		seasonID, externalID).Scan(&score)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	require.NoError(t, err)
	if !score.Valid {
		return nil
	}
	v := int(score.Int64)
	return &v
}

func TestRefreshByID_AniListOnly_SourcesMetadata(t *testing.T) {
	svc, db, titleRepo, _, _ := setupBackgroundService(t)

	anilistID := int64(555)
	id := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, IsAnime: true, Year: 2024,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
		AniListID: &anilistID,
	}, []model.TitleName{{Name: "Wrong TMDB Name", Language: "en", IsPrimary: true}})

	score, dur := 80, 24
	svc.SetAniList(&fakeAniListSeasonScoreClient{
		detailsByID: map[int64]*matching.AniListDetails{
			anilistID: {
				ID: anilistID, EnglishTitle: "Correct Anime", RomajiTitle: "Tadashii",
				Description: "The real synopsis.", Genres: []string{"Action", "Drama"},
				AverageScore: &score, Duration: &dur,
			},
		},
	})

	require.NoError(t, svc.RefreshByID(context.Background(), id))

	got, err := titleRepo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, "Correct Anime", got.PrimaryName(), "name sourced from AniList, overwriting the stale TMDB name")
	require.NotNil(t, got.Overview)
	assert.Equal(t, "The real synopsis.", *got.Overview)
	require.NotNil(t, got.AniListRating)
	assert.Equal(t, 80, *got.AniListRating)
	require.NotNil(t, got.Runtime)
	assert.Equal(t, 24, *got.Runtime)
}

func TestBackgroundService_AutoBackfillsAniListIDForAnime(t *testing.T) {
	svc, db, titleRepo, _, _ := setupBackgroundService(t)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, IsAnime: true, Year: 2024,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Frieren", Language: "en", IsPrimary: true}})
	s1 := testutil.InsertSeason(t, db, id, 1)
	_ = s1

	svc.SetAniList(&fakeAniListSeasonScoreClient{
		searchResult: map[string][]matching.AniListSearchResult{
			"Frieren": {{ID: 154587, EnglishTitle: "Sousou no Frieren"}},
		},
	})

	require.NoError(t, svc.RefreshByID(context.Background(), id))

	got, err := titleRepo.GetByID(id)
	require.NoError(t, err)
	require.NotNil(t, got.AniListID)
	assert.Equal(t, int64(154587), *got.AniListID)
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
	got1 := readPartScore(t, db, s1, "113415")
	got2 := readPartScore(t, db, s2, "145064")
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
	assert.Nil(t, readPartScore(t, db, seasonID, "100"))
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
	assert.Nil(t, readPartScore(t, db, s1, "1"))
	require.NotNil(t, readPartScore(t, db, s2, "2"))
	assert.Equal(t, 77, *readPartScore(t, db, s2, "2"))
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

// TestBackgroundService_RefreshAniListScores_PersistsPerPartMeta asserts that
// when a season has two AniList parts, the refresh job persists
// score, episode count, and start date for each part independently.
func TestBackgroundService_RefreshAniListScores_PersistsPerPartMeta(t *testing.T) {
	svc, db, _, _, _ := setupBackgroundService(t)

	titleID := testutil.InsertTitle(t, db, "Sword Art Online", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	// Two AniList parts for the same season (split-cour).
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "11757")
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "14833")

	eps1, eps2 := 25, 12
	date1, date2 := "2012-07-07", "2012-10-06"
	score1, score2 := 72, 68
	fake := &fakeAniListSeasonScoreClient{
		detailsByID: map[int64]*matching.AniListDetails{
			11757: {ID: 11757, AverageScore: &score1, Episodes: &eps1, StartDate: &date1},
			14833: {ID: 14833, AverageScore: &score2, Episodes: &eps2, StartDate: &date2},
		},
	}
	svc.SetAniList(fake)

	results := svc.RefreshTitles(context.Background())
	require.Len(t, results, 1)
	assert.ElementsMatch(t, []int64{11757, 14833}, fake.calls)

	extIDRepo := repository.NewSeasonExternalIDRepository(db)
	parts, err := extIDRepo.ListParts(context.Background(), seasonID, "anilist")
	require.NoError(t, err)
	require.Len(t, parts, 2)

	byID := make(map[string]model.AniListPart, len(parts))
	for _, p := range parts {
		byID[p.ExternalID] = p
	}

	p1 := byID["11757"]
	require.NotNil(t, p1.Score)
	assert.Equal(t, score1, *p1.Score)
	require.NotNil(t, p1.EpisodeCount)
	assert.Equal(t, eps1, *p1.EpisodeCount)
	require.NotNil(t, p1.StartDate)
	assert.Equal(t, date1, *p1.StartDate)

	p2 := byID["14833"]
	require.NotNil(t, p2.Score)
	assert.Equal(t, score2, *p2.Score)
	require.NotNil(t, p2.EpisodeCount)
	assert.Equal(t, eps2, *p2.EpisodeCount)
	require.NotNil(t, p2.StartDate)
	assert.Equal(t, date2, *p2.StartDate)
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

// --- Episode-list backfill for completed/dropped titles (heal) ---

// A completed series that was never TMDB-synced (no total_episodes) must NOT be
// skipped by the daily refresh — its episode list has to be fetched. And once
// processed, the "completed ⟹ every episode watched" invariant is enforced.
// This is the Goldorak case: imported completed with no episodes, then a couple
// of scrobbles created sparse rows.
func TestBackgroundService_HealsCompletedSeriesMissingEpisodeList(t *testing.T) {
	svc, db, _, _, episodeRepo := setupBackgroundService(t)

	tmdbID := int64(46252)
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, Year: 1975,
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
		TMDBID: &tmdbID,
	}, []model.TitleName{{Name: "Grendizer", Language: "en", IsPrimary: true}})

	season := testutil.GetOrCreateSeason(t, db, titleID, 1) // total_episodes stays NULL
	ep4 := testutil.GetOrCreateEpisode(t, db, season.ID, 4)
	_ = testutil.GetOrCreateEpisode(t, db, season.ID, 5)
	testutil.MarkEpisodeWatched(t, db, ep4.ID, time.Now().UTC())

	results := svc.RefreshTitles(context.Background())
	require.Len(t, results, 1, "completed series lacking a synced episode list must NOT be skipped")

	eps, _ := episodeRepo.GetBySeasonID(season.ID)
	require.Len(t, eps, 2)
	for _, e := range eps {
		assert.Truef(t, e.Watched, "completed ⟹ every episode watched (E%d)", e.Episode)
	}
}

// A completed series whose seasons already carry total_episodes (TMDB-synced)
// stays skipped — no wasted re-fetch, and its episodes are left untouched.
func TestBackgroundService_SkipsCompletedSeriesWithSyncedList(t *testing.T) {
	svc, db, _, _, episodeRepo := setupBackgroundService(t)

	tmdbID := int64(1399)
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, Year: 2011,
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
		TMDBID: &tmdbID,
	}, []model.TitleName{{Name: "GoT", Language: "en", IsPrimary: true}})

	season := testutil.GetOrCreateSeason(t, db, titleID, 1)
	testutil.UpdateSeasonTotalEpisodes(t, db, season.ID, 2) // already synced
	_ = testutil.GetOrCreateEpisode(t, db, season.ID, 1)    // unwatched

	results := svc.RefreshTitles(context.Background())
	assert.Empty(t, results, "already-synced completed series stays skipped")

	eps, _ := episodeRepo.GetBySeasonID(season.ID)
	require.Len(t, eps, 1)
	assert.False(t, eps[0].Watched, "skipped title is left untouched")
}

// A completed series with no TMDB id can't have its episode list fetched from
// anywhere, so it stays skipped (avoids re-processing it on every cron pass).
func TestBackgroundService_SkipsCompletedSeriesWithoutTMDB(t *testing.T) {
	svc, db, _, _, _ := setupBackgroundService(t)

	testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, Year: 2000,
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "NoTMDB", Language: "en", IsPrimary: true}})

	results := svc.RefreshTitles(context.Background())
	assert.Empty(t, results, "no TMDB id → nothing can supply the episode list → skip")
}

// A dropped series lacking a synced list is also un-skipped (so its episode list
// gets fetched), but the completed-only invariant must NOT mark its episodes
// watched.
func TestBackgroundService_DroppedSeriesNotForceWatched(t *testing.T) {
	svc, db, _, _, episodeRepo := setupBackgroundService(t)

	tmdbID := int64(999)
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, Year: 2018,
		Status: model.TitleStatusDropped, MatchStatus: model.MatchStatusConfirmed,
		TMDBID: &tmdbID,
	}, []model.TitleName{{Name: "Dropped", Language: "en", IsPrimary: true}})

	season := testutil.GetOrCreateSeason(t, db, titleID, 1)
	_ = testutil.GetOrCreateEpisode(t, db, season.ID, 1) // unwatched

	results := svc.RefreshTitles(context.Background())
	require.Len(t, results, 1, "dropped series lacking a synced list is still processed (list backfill)")

	eps, _ := episodeRepo.GetBySeasonID(season.ID)
	require.Len(t, eps, 1)
	assert.False(t, eps[0].Watched, "dropped ⟹ watched flags untouched")
}

func TestBackgroundService_SeriesRefresh_PrunesSurplusEpisodes(t *testing.T) {
	tmdbID := int64(12345)
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/tv/%d", tmdbID), func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     tmdbID,
			"name":   "Anime Series",
			"status": "Ended",
			"seasons": []map[string]any{
				{"season_number": 1, "episode_count": 2},
			},
		})
	})
	mux.HandleFunc(fmt.Sprintf("/tv/%d/season/1", tmdbID), func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"season_number": 1,
			"episodes": []matching.TMDBEpisode{
				{EpisodeNumber: 1, Name: "Ep 1"},
				{EpisodeNumber: 2, Name: "Ep 2"},
			},
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := matching.NewTMDBClient("test-key")
	client.SetBaseURL(server.URL)

	svc, db, _, _ := newBackgroundServiceWithTMDB(t, client)
	episodeRepo := repository.NewEpisodeRepository(db)

	runtime := 20
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:              model.TitleTypeSeries,
		Year:              2024,
		Status:            model.TitleStatusWatching,
		MatchStatus:       model.MatchStatusConfirmed,
		TMDBID:            &tmdbID,
		Runtime:           &runtime,
		TotalWatchMinutes: 60,
	}, []model.TitleName{{Name: "Anime Series", Language: "en", IsPrimary: true}})

	season := testutil.GetOrCreateSeason(t, db, titleID, 1)
	ep1 := testutil.GetOrCreateEpisode(t, db, season.ID, 1)
	_ = testutil.GetOrCreateEpisode(t, db, season.ID, 2)
	ep3 := testutil.GetOrCreateEpisode(t, db, season.ID, 3)

	testutil.ToggleEpisodeWatched(t, db, ep1.ID)
	testutil.ToggleEpisodeWatched(t, db, ep3.ID)
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{
		TitleID:   titleID,
		EpisodeID: &ep3.ID,
		Source:    model.WatchEventSourceManual,
	})

	err := svc.RefreshByID(context.Background(), titleID)
	require.NoError(t, err)

	eps, err := episodeRepo.GetBySeasonID(season.ID)
	require.NoError(t, err)
	require.Len(t, eps, 2, "ep3 should be pruned")
	assert.Equal(t, 1, eps[0].Episode)
	assert.Equal(t, 2, eps[1].Episode)

	// Watch event for ep3 should be pruned
	var weCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM watch_events WHERE title_id = ?`, titleID).Scan(&weCount)
	require.NoError(t, err)
	assert.Equal(t, 0, weCount)

	// Total watch minutes should be decremented from 60 to 40 (pruned 1 watched episode with runtime 20)
	var totalMinutes int
	err = db.QueryRow(`SELECT total_watch_minutes FROM titles WHERE id = ?`, titleID).Scan(&totalMinutes)
	require.NoError(t, err)
	assert.Equal(t, 40, totalMinutes)
}

func TestBackgroundService_RefreshTitle_SyncsNamesAndPurgesStaleTranslations(t *testing.T) {
	tmdbID := int64(314554)
	anilistID := int64(208044)

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/tv/%d", tmdbID), func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":        tmdbID,
			"name":      "New English Title",
			"overview":  "Overview",
			"status":    "Returning Series",
			"seasons":   []any{},
			"genres":    []any{},
			"credits":   map[string]any{"cast": []any{}},
			"vote_avg":  6.0,
			"run_times": []int{24},
		})
	})
	mux.HandleFunc(fmt.Sprintf("/tv/%d/translations", tmdbID), func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"translations": []map[string]any{
				{
					"iso_639_1":  "en",
					"iso_3166_1": "US",
					"data":       map[string]any{"name": "New English Title"},
				},
				{
					"iso_639_1":  "fr",
					"iso_3166_1": "FR",
					"data":       map[string]any{"name": "Nouvelle Traduction FR"},
				},
			},
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := matching.NewTMDBClient("test-key")
	client.SetBaseURL(server.URL)

	svc, db, titleRepo, _ := newBackgroundServiceWithTMDB(t, client)
	svc.SetAniList(&fakeAniListSeasonScoreClient{
		detailsByID: map[int64]*matching.AniListDetails{
			anilistID: {
				ID:           anilistID,
				EnglishTitle: "New English Title",
				RomajiTitle:  "Rakudai Kenja Romaji",
			},
		},
	})

	// Initial state: old stale romaji saved as "fr", plus an old en title
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2026,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
		TMDBID:      &tmdbID,
		AniListID:   &anilistID,
	}, []model.TitleName{
		{Name: "Old English", Language: "en", IsPrimary: true},
		{Name: "Old Stale Romaji In FR", Language: "fr", IsPrimary: false},
	})

	err := svc.RefreshByID(context.Background(), titleID)
	require.NoError(t, err)

	got, err := titleRepo.GetByID(titleID)
	require.NoError(t, err)
	assert.Equal(t, "New English Title", got.PrimaryName())

	// Check title_names in DB
	rows, err := db.Query(`SELECT name, language, is_primary FROM title_names WHERE title_id = ? ORDER BY language`, titleID)
	require.NoError(t, err)
	defer rows.Close()

	namesByLang := make(map[string]model.TitleName)
	for rows.Next() {
		var n model.TitleName
		require.NoError(t, rows.Scan(&n.Name, &n.Language, &n.IsPrimary))
		namesByLang[n.Language] = n
	}

	assert.Len(t, namesByLang, 3, "should have en, fr, x-romaji and nothing else")

	enName, ok := namesByLang["en"]
	require.True(t, ok)
	assert.Equal(t, "New English Title", enName.Name)
	assert.True(t, enName.IsPrimary)

	frName, ok := namesByLang["fr"]
	require.True(t, ok)
	assert.Equal(t, "Nouvelle Traduction FR", frName.Name)
	assert.False(t, frName.IsPrimary)

	romajiName, ok := namesByLang["x-romaji"]
	require.True(t, ok)
	assert.Equal(t, "Rakudai Kenja Romaji", romajiName.Name)
	assert.False(t, romajiName.IsPrimary)
}

