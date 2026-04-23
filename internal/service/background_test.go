package service_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
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
	taskRepo := repository.NewTaskRepository(db)

	pushSvc := service.NewPushService(settingRepo, "pub", "priv", "mailto:test@test.com")
	// No external API clients in tests — nil TMDB/AniList
	svc := service.NewBackgroundService(db, titleRepo, nil, seasonRepo, episodeRepo, taskRepo, settingRepo, nil, nil, pushSvc, t.TempDir())
	return svc, db, titleRepo, seasonRepo, episodeRepo
}

func TestBackgroundService_DetectCompletedSeries(t *testing.T) {
	svc, db, titleRepo, seasonRepo, _ := setupBackgroundService(t)

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
	season, err := seasonRepo.GetOrCreate(titleID, 1)
	require.NoError(t, err)
	require.NoError(t, seasonRepo.UpdateTotalEpisodes(season.ID, 2))

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
	svc, db, titleRepo, seasonRepo, _ := setupBackgroundService(t)

	ended := model.SeriesStatusEnded
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:         model.TitleTypeSeries,
		Year:         2023,
		Status:       model.TitleStatusWatching,
		MatchStatus:  model.MatchStatusConfirmed,
		SeriesStatus: &ended,
	}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	season, err := seasonRepo.GetOrCreate(titleID, 1)
	require.NoError(t, err)
	require.NoError(t, seasonRepo.UpdateTotalEpisodes(season.ID, 2))

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

func TestBackgroundService_CleanupUnusedCovers(t *testing.T) {
	dataDir := t.TempDir()

	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	pushSvc := service.NewPushService(settingRepo, "pub", "priv", "mailto:test@test.com")
	svc := service.NewBackgroundService(db, titleRepo, nil, seasonRepo, episodeRepo, taskRepo, settingRepo, nil, nil, pushSvc, dataDir)

	coversDir := filepath.Join(dataDir, "covers")
	err = os.MkdirAll(coversDir, 0755)
	require.NoError(t, err)

	// Create some dummy files
	files := []string{
		"a123.jpg", // Sunday prefix 'a'
		"b456.jpg", // Sunday prefix 'b'
		"j789.jpg", // Monday prefix 'j'
		"z012.jpg", // Tuesday prefix 'z'
	}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(coversDir, f), []byte("dummy"), 0644)
		require.NoError(t, err)
	}

	// Reference a123.jpg and z012.jpg in the DB
	coverA := "a123.jpg"
	coverZ := "z012.jpg"
	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2023,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
		CoverURL:    &coverA,
	}, []model.TitleName{{Name: "Movie A", Language: "en", IsPrimary: true}})

	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2023,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
		CoverURL:    &coverZ,
	}, []model.TitleName{{Name: "Movie Z", Language: "en", IsPrimary: true}})

	// Run cleanup for Sunday
	svc.CleanupUnusedCovers(context.Background(), time.Sunday)

	// After Sunday cleanup, 'b456.jpg' should be deleted (starts with 'b', unused)
	// 'a123.jpg' remains (starts with 'a', used)
	// 'j789.jpg' remains (starts with 'j', not checked on Sunday)
	// 'z012.jpg' remains (starts with 'z', not checked on Sunday)

	_, err = os.Stat(filepath.Join(coversDir, "b456.jpg"))
	assert.True(t, os.IsNotExist(err), "b456.jpg should be deleted")

	_, err = os.Stat(filepath.Join(coversDir, "a123.jpg"))
	assert.NoError(t, err, "a123.jpg should remain")

	_, err = os.Stat(filepath.Join(coversDir, "j789.jpg"))
	assert.NoError(t, err, "j789.jpg should remain")

	_, err = os.Stat(filepath.Join(coversDir, "z012.jpg"))
	assert.NoError(t, err, "z012.jpg should remain")
}
