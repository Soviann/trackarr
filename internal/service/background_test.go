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
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
