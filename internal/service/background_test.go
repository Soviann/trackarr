package service_test

import (
	"context"
	"database/sql"
	"fmt"
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

func TestBackgroundService_RefreshAniListScores_On401FlagsTokenAndAborts(t *testing.T) {
	svc, db, _, _, _ := setupBackgroundService(t)

	titleID := testutil.InsertTitle(t, db, "Anime", true)
	s1 := testutil.InsertSeason(t, db, titleID, 1)
	s2 := testutil.InsertSeason(t, db, titleID, 2)
	testutil.InsertSeasonExternalID(t, db, s1, "anilist", "1")
	testutil.InsertSeasonExternalID(t, db, s2, "anilist", "2")

	fake := &fakeAniListSeasonScoreClient{
		errByID:   map[int64]error{1: matching.TokenInvalidError{}, 2: matching.TokenInvalidError{}},
		scoreByID: map[int64]int{},
	}
	svc.SetAniList(fake)

	_ = svc.RefreshTitles(context.Background())

	got, _ := testutil.GetSetting(t, db, "anilist_token_invalid")
	assert.Equal(t, "true", got, "401 must flag the token invalid")
	assert.Nil(t, readSeasonAniListScore(t, db, s1))
	assert.Nil(t, readSeasonAniListScore(t, db, s2))
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
