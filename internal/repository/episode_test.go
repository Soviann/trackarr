package repository_test

import (
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeasonRepository_GetOrCreate(t *testing.T) {
	db := setupTestDB(t)
	seasonRepo := repository.NewSeasonRepository(db)

	titleID := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	s1, err := seasonRepo.GetOrCreate(titleID, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, s1.SeasonNumber)

	// Second call returns same season
	s2, err := seasonRepo.GetOrCreate(titleID, 1)
	require.NoError(t, err)
	assert.Equal(t, s1.ID, s2.ID)
}

func TestEpisodeRepository_ToggleWatched(t *testing.T) {
	db := setupTestDB(t)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)

	titleID := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})
	season, _ := seasonRepo.GetOrCreate(titleID, 1)
	ep := testutil.GetOrCreateEpisode(t, db, season.ID, 1)

	// Toggle on
	toggled := testutil.ToggleEpisodeWatched(t, db, ep.ID)
	assert.True(t, toggled.Watched)
	assert.NotNil(t, toggled.FirstWatchedAt)
	assert.NotNil(t, toggled.LastWatchedAt)

	// Toggle off
	toggled = testutil.ToggleEpisodeWatched(t, db, ep.ID)
	assert.False(t, toggled.Watched)
	assert.Nil(t, toggled.FirstWatchedAt)
	assert.Nil(t, toggled.LastWatchedAt)

	// Reader still returns the stored state.
	got, err := episodeRepo.GetBySeasonID(season.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.False(t, got[0].Watched)
}

func TestEpisodeRepository_BatchMarkWatched(t *testing.T) {
	db := setupTestDB(t)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)

	titleID := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})
	season, _ := seasonRepo.GetOrCreate(titleID, 1)
	ep1 := testutil.GetOrCreateEpisode(t, db, season.ID, 1)
	ep2 := testutil.GetOrCreateEpisode(t, db, season.ID, 2)

	now := time.Now().UTC()
	testutil.BatchMarkEpisodesWatched(t, db, []int64{ep1.ID, ep2.ID}, now)

	episodes, _ := episodeRepo.GetBySeasonID(season.ID)
	assert.Len(t, episodes, 2)
	assert.True(t, episodes[0].Watched)
	assert.True(t, episodes[1].Watched)
}

func TestEpisodeRepository_BatchMarkWatched_PreservesFirstWatchedAt(t *testing.T) {
	db := setupTestDB(t)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)

	titleID := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})
	season, _ := seasonRepo.GetOrCreate(titleID, 1)
	ep := testutil.GetOrCreateEpisode(t, db, season.ID, 1)

	// First watch
	first := time.Now().UTC().Add(-24 * time.Hour)
	testutil.BatchMarkEpisodesWatched(t, db, []int64{ep.ID}, first)

	episodes, _ := episodeRepo.GetBySeasonID(season.ID)
	require.Len(t, episodes, 1)
	require.NotNil(t, episodes[0].FirstWatchedAt)
	assert.WithinDuration(t, first, *episodes[0].FirstWatchedAt, time.Second)
	assert.WithinDuration(t, first, *episodes[0].LastWatchedAt, time.Second)

	// Rewatch — first_watched_at must be preserved, last_watched_at must update
	rewatch := time.Now().UTC()
	testutil.BatchMarkEpisodesWatched(t, db, []int64{ep.ID}, rewatch)

	episodes, _ = episodeRepo.GetBySeasonID(season.ID)
	require.NotNil(t, episodes[0].FirstWatchedAt)
	assert.WithinDuration(t, first, *episodes[0].FirstWatchedAt, time.Second, "first_watched_at must not change on rewatch")
	assert.WithinDuration(t, rewatch, *episodes[0].LastWatchedAt, time.Second, "last_watched_at must update on rewatch")
}

func TestSettingRepository_SetAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewSettingRepository(db)

	err := repo.Set("test_key", "test_value")
	require.NoError(t, err)

	val, err := repo.Get("test_key")
	require.NoError(t, err)
	assert.Equal(t, "test_value", val)

	// Update
	err = repo.Set("test_key", "new_value")
	require.NoError(t, err)

	val, _ = repo.Get("test_key")
	assert.Equal(t, "new_value", val)

	// Delete
	err = repo.Delete("test_key")
	require.NoError(t, err)
	_, err = repo.Get("test_key")
	assert.Error(t, err)
}
