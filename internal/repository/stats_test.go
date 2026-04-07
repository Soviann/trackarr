package repository_test

import (
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsRepository_Overview(t *testing.T) {
	db := setupTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	// Create titles of different types and statuses
	createTitle(t, titleRepo, "Dune", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(8))
	createTitle(t, titleRepo, "Breaking Bad", model.TitleTypeSeries, false, model.TitleStatusCompleted, ptr(9))
	createTitle(t, titleRepo, "Naruto", model.TitleTypeSeries, true, model.TitleStatusWatching, nil)

	resp, err := statsRepo.GetAll()
	require.NoError(t, err)

	assert.Equal(t, 3, resp.Overview.TotalTitles)
	assert.Equal(t, 1, resp.Overview.TotalMovies)
	assert.Equal(t, 2, resp.Overview.TotalSeries)
	assert.Equal(t, 1, resp.Overview.TotalAnime)
	assert.InDelta(t, 0.67, resp.Overview.CompletionRate, 0.01)
	assert.InDelta(t, 8.5, resp.Overview.AverageRating, 0.1)
}

func TestStatsRepository_RatingDistribution(t *testing.T) {
	db := setupTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	createTitle(t, titleRepo, "A", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(7))
	createTitle(t, titleRepo, "B", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(7))
	createTitle(t, titleRepo, "C", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(5))

	resp, err := statsRepo.GetAll()
	require.NoError(t, err)

	assert.Equal(t, 2, resp.Ratings.Distribution[6]) // rating 7 at index 6
	assert.Equal(t, 1, resp.Ratings.Distribution[4]) // rating 5 at index 4
	assert.Contains(t, resp.Ratings.Insight, "67%")
}

func TestStatsRepository_Breakdown(t *testing.T) {
	db := setupTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	createTitle(t, titleRepo, "A", model.TitleTypeMovie, false, model.TitleStatusWatching, nil)
	createTitle(t, titleRepo, "B", model.TitleTypeSeries, false, model.TitleStatusCompleted, nil)
	createTitle(t, titleRepo, "C", model.TitleTypeSeries, true, model.TitleStatusCompleted, nil)

	resp, err := statsRepo.GetAll()
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Breakdown.ByStatus["watching"])
	assert.Equal(t, 2, resp.Breakdown.ByStatus["completed"])
	assert.Equal(t, 1, resp.Breakdown.ByType["movie"])
	assert.Equal(t, 2, resp.Breakdown.ByType["series"])
}

func TestStatsRepository_FunStats_Graveyard(t *testing.T) {
	db := setupTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	createTitle(t, titleRepo, "Dropped Show", model.TitleTypeSeries, false, model.TitleStatusDropped, nil)

	resp, err := statsRepo.GetAll()
	require.NoError(t, err)

	found := false
	for _, s := range resp.FunStats {
		if s.ID == "graveyard" {
			found = true
			assert.Equal(t, "1 titres", s.Value)
		}
	}
	assert.True(t, found, "graveyard stat should be present")
}

func TestStatsRepository_FunStats_PlexVsManual(t *testing.T) {
	db := setupTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	eventRepo := repository.NewWatchEventRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	id, _ := titleRepo.Create(&model.Title{
		Type: model.TitleTypeMovie, Year: 2024,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	_, _ = eventRepo.Create(&model.WatchEvent{TitleID: id, Source: model.WatchEventSourcePlex})
	_, _ = eventRepo.Create(&model.WatchEvent{TitleID: id, Source: model.WatchEventSourcePlex})
	_, _ = eventRepo.Create(&model.WatchEvent{TitleID: id, Source: model.WatchEventSourceManual})

	resp, err := statsRepo.GetAll()
	require.NoError(t, err)

	found := false
	for _, s := range resp.FunStats {
		if s.ID == "plex_vs_manual" {
			found = true
			assert.Equal(t, "67% Plex, 33% manuels", s.Value)
		}
	}
	assert.True(t, found, "plex_vs_manual stat should be present")
}

func TestStatsRepository_YearSummary(t *testing.T) {
	db := setupTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	// Titles created_at defaults to CURRENT_TIMESTAMP, so they count for the current year
	createTitle(t, titleRepo, "A", model.TitleTypeMovie, false, model.TitleStatusCompleted, nil)
	createTitle(t, titleRepo, "B", model.TitleTypeSeries, false, model.TitleStatusWatching, nil)

	resp, err := statsRepo.GetAll()
	require.NoError(t, err)

	assert.Equal(t, 2, resp.Year.TitlesAdded)
	assert.Equal(t, 1, resp.Year.Completions)
}

func TestStatsRepository_EmptyDatabase(t *testing.T) {
	db := setupTestDB(t)
	statsRepo := repository.NewStatsRepository(db)

	resp, err := statsRepo.GetAll()
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Overview.TotalTitles)
	assert.Equal(t, 0, resp.Overview.EpisodesWatched)
	assert.Equal(t, 0.0, resp.Overview.CompletionRate)
	assert.Equal(t, 0.0, resp.Overview.AverageRating)
	assert.Empty(t, resp.FunStats)
}

func TestStatsRepository_FunStats_LongestBinge(t *testing.T) {
	db := setupTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	id, _ := titleRepo.Create(&model.Title{
		Type: model.TitleTypeSeries, Year: 2024,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Attack on Titan", Language: "en", IsPrimary: true}})

	// Create a season with episodes
	_, err := db.Exec(`INSERT INTO seasons (title_id, season_number, total_episodes) VALUES (?, 1, 5)`, id)
	require.NoError(t, err)

	var seasonID int64
	err = db.QueryRow(`SELECT id FROM seasons WHERE title_id = ?`, id).Scan(&seasonID)
	require.NoError(t, err)

	// Insert 4 episodes watched on the same day
	watchDate := time.Now().Format("2006-01-02") + " 20:00:00"
	for i := 1; i <= 4; i++ {
		_, err := db.Exec(`INSERT INTO episodes (season_id, episode, watched, watched_at) VALUES (?, ?, 1, ?)`, seasonID, i, watchDate)
		require.NoError(t, err)
	}

	resp, err := statsRepo.GetAll()
	require.NoError(t, err)

	found := false
	for _, s := range resp.FunStats {
		if s.ID == "longest_binge" {
			found = true
			assert.Equal(t, "4 épisodes", s.Value)
			assert.Contains(t, s.Detail, "Attack on Titan")
		}
	}
	assert.True(t, found, "longest_binge stat should be present with 4 episodes on same day")
}

// Helper to create a title with a rating
func createTitle(t *testing.T, repo *repository.TitleRepository, name string, titleType model.TitleType, isAnime bool, status model.TitleStatus, rating *int) int64 {
	t.Helper()
	title := &model.Title{
		Type:        titleType,
		IsAnime:     isAnime,
		Year:        2024,
		Status:      status,
		MatchStatus: model.MatchStatusConfirmed,
		MyRating:    rating,
	}
	names := []model.TitleName{{Name: name, Language: "en", IsPrimary: true}}
	id, err := repo.Create(title, names)
	require.NoError(t, err)
	return id
}
