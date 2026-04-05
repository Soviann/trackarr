package service_test

import (
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBackgroundService(t *testing.T) (*service.BackgroundService, *repository.TitleRepository, *repository.SeasonRepository, *repository.EpisodeRepository) {
	t.Helper()
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	settingRepo := repository.NewSettingRepository(db)

	pushSvc := service.NewPushService(settingRepo, "pub", "priv", "mailto:test@test.com")
	// No external API clients in tests — nil TMDB/AniList
	svc := service.NewBackgroundService(titleRepo, seasonRepo, episodeRepo, nil, settingRepo, nil, nil, pushSvc, t.TempDir())
	return svc, titleRepo, seasonRepo, episodeRepo
}

func TestBackgroundService_DetectCompletedSeries(t *testing.T) {
	svc, titleRepo, seasonRepo, episodeRepo := setupBackgroundService(t)

	// Create a series with status=watching, series_status=ended
	ended := model.SeriesStatusEnded
	titleID, err := titleRepo.Create(&model.Title{
		Type:         model.TitleTypeSeries,
		Year:         2023,
		Status:       model.TitleStatusWatching,
		MatchStatus:  model.MatchStatusConfirmed,
		SeriesStatus: &ended,
	}, []model.TitleName{{Name: "Shogun", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	// Create 1 season with 2 episodes, all watched
	season, err := seasonRepo.GetOrCreate(titleID, 1)
	require.NoError(t, err)
	require.NoError(t, seasonRepo.UpdateTotalEpisodes(season.ID, 2))

	ep1, err := episodeRepo.GetOrCreate(season.ID, 1)
	require.NoError(t, err)
	ep2, err := episodeRepo.GetOrCreate(season.ID, 2)
	require.NoError(t, err)
	now := time.Now().UTC()
	require.NoError(t, episodeRepo.MarkWatched(ep1.ID, now))
	require.NoError(t, episodeRepo.MarkWatched(ep2.ID, now))

	// Run refresh
	results := svc.RefreshTitles()

	// Title should be auto-completed
	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusCompleted, title.Status)
	assert.Len(t, results, 1)
	assert.True(t, results[0].AutoCompleted)
}

func TestBackgroundService_SkipCompletedTitles(t *testing.T) {
	svc, titleRepo, _, _ := setupBackgroundService(t)

	// Create a completed title
	_, err := titleRepo.Create(&model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	results := svc.RefreshTitles()
	assert.Empty(t, results)
}

func TestBackgroundService_NoAutoCompleteIfUnwatchedEpisodes(t *testing.T) {
	svc, titleRepo, seasonRepo, episodeRepo := setupBackgroundService(t)

	ended := model.SeriesStatusEnded
	titleID, err := titleRepo.Create(&model.Title{
		Type:         model.TitleTypeSeries,
		Year:         2023,
		Status:       model.TitleStatusWatching,
		MatchStatus:  model.MatchStatusConfirmed,
		SeriesStatus: &ended,
	}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	season, err := seasonRepo.GetOrCreate(titleID, 1)
	require.NoError(t, err)
	require.NoError(t, seasonRepo.UpdateTotalEpisodes(season.ID, 2))

	ep1, err := episodeRepo.GetOrCreate(season.ID, 1)
	require.NoError(t, err)
	_, err = episodeRepo.GetOrCreate(season.ID, 2) // ep2 unwatched
	require.NoError(t, err)
	require.NoError(t, episodeRepo.MarkWatched(ep1.ID, time.Now().UTC()))

	results := svc.RefreshTitles()

	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusWatching, title.Status)
	assert.Len(t, results, 1)
	assert.False(t, results[0].AutoCompleted)
}

func TestBackgroundService_NilSafe(t *testing.T) {
	var svc *service.BackgroundService
	results := svc.RefreshTitles()
	assert.Nil(t, results)
}
