package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsRepository_Overview(t *testing.T) {
	db := setupTestDB(t)
	statsRepo := repository.NewStatsRepository(db)

	// Create titles of different types and statuses
	createTitle(t, db, "Dune", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(8))
	createTitle(t, db, "Breaking Bad", model.TitleTypeSeries, false, model.TitleStatusCompleted, ptr(9))
	createTitle(t, db, "Naruto", model.TitleTypeSeries, true, model.TitleStatusWatching, nil)

	resp, err := statsRepo.GetAll(context.Background())
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
	statsRepo := repository.NewStatsRepository(db)

	createTitle(t, db, "A", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(7))
	createTitle(t, db, "B", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(7))
	createTitle(t, db, "C", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(5))

	resp, err := statsRepo.GetAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, resp.Ratings.Distribution[6]) // rating 7 at index 6
	assert.Equal(t, 1, resp.Ratings.Distribution[4]) // rating 5 at index 4
	assert.Contains(t, resp.Ratings.Insight, "67%")
}

func TestStatsRepository_Breakdown(t *testing.T) {
	db := setupTestDB(t)
	statsRepo := repository.NewStatsRepository(db)

	createTitle(t, db, "A", model.TitleTypeMovie, false, model.TitleStatusWatching, nil)
	createTitle(t, db, "B", model.TitleTypeSeries, false, model.TitleStatusCompleted, nil)
	createTitle(t, db, "C", model.TitleTypeSeries, true, model.TitleStatusCompleted, nil)

	resp, err := statsRepo.GetAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Breakdown.ByStatus["watching"])
	assert.Equal(t, 2, resp.Breakdown.ByStatus["completed"])
	assert.Equal(t, 1, resp.Breakdown.ByType["movie"])
	assert.Equal(t, 2, resp.Breakdown.ByType["series"])
}

func TestStatsRepository_FunStats_Graveyard(t *testing.T) {
	db := setupTestDB(t)
	statsRepo := repository.NewStatsRepository(db)

	createTitle(t, db, "Dropped Show", model.TitleTypeSeries, false, model.TitleStatusDropped, nil)

	resp, err := statsRepo.GetAll(context.Background())
	require.NoError(t, err)

	found := false
	for _, s := range resp.FunStats {
		if s.ID == "graveyard" {
			found = true
			assert.Equal(t, "1 titles", s.Value)
		}
	}
	assert.True(t, found, "graveyard stat should be present")
}

func TestStatsRepository_FunStats_PlexVsManual(t *testing.T) {
	db := setupTestDB(t)
	statsRepo := repository.NewStatsRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2024,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: id, Source: model.WatchEventSourcePlex})
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: id, Source: model.WatchEventSourcePlex})
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: id, Source: model.WatchEventSourceManual})

	resp, err := statsRepo.GetAll(context.Background())
	require.NoError(t, err)

	found := false
	for _, s := range resp.FunStats {
		if s.ID == "plex_vs_manual" {
			found = true
			assert.Equal(t, "67% Plex, 33% manual", s.Value)
		}
	}
	assert.True(t, found, "plex_vs_manual stat should be present")
}

func TestStatsRepository_YearSummary(t *testing.T) {
	db := setupTestDB(t)
	statsRepo := repository.NewStatsRepository(db)

	// Titles created_at defaults to CURRENT_TIMESTAMP, so they count for the current year
	createTitle(t, db, "A", model.TitleTypeMovie, false, model.TitleStatusCompleted, nil)
	createTitle(t, db, "B", model.TitleTypeSeries, false, model.TitleStatusWatching, nil)

	resp, err := statsRepo.GetAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, resp.Year.TitlesAdded)
	assert.Equal(t, 1, resp.Year.Completions)
}

func TestStatsRepository_EmptyDatabase(t *testing.T) {
	db := setupTestDB(t)
	statsRepo := repository.NewStatsRepository(db)

	resp, err := statsRepo.GetAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Overview.TotalTitles)
	assert.Equal(t, 0, resp.Overview.EpisodesWatched)
	assert.Equal(t, 0.0, resp.Overview.CompletionRate)
	assert.Equal(t, 0.0, resp.Overview.AverageRating)
	assert.Empty(t, resp.FunStats)
}

func TestStatsRepository_FunStats_LongestBinge(t *testing.T) {
	db := setupTestDB(t)
	statsRepo := repository.NewStatsRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{
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
		_, err := db.Exec(`INSERT INTO episodes (season_id, episode, watched, first_watched_at) VALUES (?, ?, 1, ?)`, seasonID, i, watchDate)
		require.NoError(t, err)
	}

	resp, err := statsRepo.GetAll(context.Background())
	require.NoError(t, err)

	found := false
	for _, s := range resp.FunStats {
		if s.ID == "longest_binge" {
			found = true
			assert.Equal(t, "4 episodes", s.Value)
			assert.Contains(t, s.Detail, "Attack on Titan")
		}
	}
	assert.True(t, found, "longest_binge stat should be present with 4 episodes on same day")
}

func TestStatsRepo_TotalWatchMinutes_NoColumn(t *testing.T) {
	// Without the total_watch_minutes column (soft dependency not yet deployed), must return 0 gracefully.
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	total, err := repo.TotalWatchMinutes(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, total)
}

func TestStatsRepo_TopGenres_TableMissing(t *testing.T) {
	// Without title_genres table, must return empty slice gracefully.
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	genres, err := repo.TopGenres(context.Background(), 10)
	assert.NoError(t, err)
	assert.Empty(t, genres)
}

func TestStatsRepo_CurrentStreak_Empty(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	streak, err := repo.CurrentStreak(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, streak)
}

func TestStatsRepo_CurrentStreak_Consecutive(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	// Need a title first to satisfy FK (titles table exists, but watch_events.title_id references it)
	id := createTitle(t, db, "TestTitle", model.TitleTypeMovie, false, model.TitleStatusWatching, nil)

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysAgo := time.Now().AddDate(0, 0, -2).Format("2006-01-02")

	_, err := db.Exec(`INSERT INTO watch_events (title_id, source, created_at) VALUES (?, 'manual', ?), (?, 'manual', ?), (?, 'manual', ?)`,
		id, today+" 21:00:00", id, yesterday+" 20:00:00", id, twoDaysAgo+" 19:00:00")
	require.NoError(t, err)

	streak, err := repo.CurrentStreak(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 3, streak)
}

func TestStatsRepo_BestStreak(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	id := createTitle(t, db, "TestTitle", model.TitleTypeMovie, false, model.TitleStatusWatching, nil)

	// 5 consecutive days (10..6 days ago), then gap, then 2 days (2..1 days ago)
	for i := 10; i >= 6; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		_, err := db.Exec(`INSERT INTO watch_events (title_id, source, created_at) VALUES (?, 'manual', ?)`, id, d+" 20:00:00")
		require.NoError(t, err)
	}
	for i := 2; i >= 1; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		_, err := db.Exec(`INSERT INTO watch_events (title_id, source, created_at) VALUES (?, 'manual', ?)`, id, d+" 20:00:00")
		require.NoError(t, err)
	}

	best, err := repo.BestStreak(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 5, best)
}

func TestStatsRepo_TopActors_And_Directors(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	// Dune: Watched movie, directed by Denis Villeneuve, starring Timothée Chalamet & Zendaya
	duneCredits := `[{"name":"Denis Villeneuve","role":"Director"},{"name":"Timothée Chalamet","role":"Paul Atreides"},{"name":"Zendaya","role":"Chani"}]`
	id1 := createTitle(t, db, "Dune", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(9))
	_, err := db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, duneCredits, id1)
	require.NoError(t, err)

	// Dune 2: Watched movie, directed by Denis Villeneuve, starring Timothée Chalamet & Florence Pugh
	dune2Credits := `[{"name":"Denis Villeneuve","role":"Director"},{"name":"Timothée Chalamet","role":"Paul Atreides"},{"name":"Florence Pugh","role":"Princess Irulan"}]`
	id2 := createTitle(t, db, "Dune Part Two", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(10))
	_, err = db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, dune2Credits, id2)
	require.NoError(t, err)

	// Wonka: Watching movie, starring Timothée Chalamet, directed by Paul King
	wonkaCredits := `[{"name":"Paul King","role":"Director"},{"name":"Timothée Chalamet","role":"Willy Wonka"}]`
	id3 := createTitle(t, db, "Wonka", model.TitleTypeMovie, false, model.TitleStatusWatching, ptr(8))
	_, err = db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, wonkaCredits, id3)
	require.NoError(t, err)

	// Unwatched movie in plan_to_watch should NOT count
	oppenheimerCredits := `[{"name":"Christopher Nolan","role":"Director"},{"name":"Cillian Murphy","role":"J. Robert Oppenheimer"},{"name":"Florence Pugh","role":"Jean Tatlock"}]`
	id4 := createTitle(t, db, "Oppenheimer", model.TitleTypeMovie, false, model.TitleStatusPlanToWatch, nil)
	_, err = db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, oppenheimerCredits, id4)
	require.NoError(t, err)

	// Test TopActors
	actors, err := repo.TopActors(context.Background(), 10)
	require.NoError(t, err)
	require.NotEmpty(t, actors)

	// Timothée Chalamet: 3 watched titles (Dune, Dune 2, Wonka)
	assert.Equal(t, "Timothée Chalamet", actors[0].Name)
	assert.Equal(t, 3, actors[0].Count)

	// Florence Pugh has 1 (Dune 2, since Oppenheimer is in plan_to_watch)
	// Zendaya has 1 (Dune)
	actorNames := make(map[string]int)
	for _, a := range actors {
		actorNames[a.Name] = a.Count
	}
	assert.Equal(t, 1, actorNames["Florence Pugh"])
	assert.Equal(t, 1, actorNames["Zendaya"])
	assert.NotContains(t, actorNames, "Cillian Murphy", "unwatched titles should not be included")
	assert.NotContains(t, actorNames, "Denis Villeneuve", "directors should not be in top actors")

	// Test TopDirectors
	directors, err := repo.TopDirectors(context.Background(), 10)
	require.NoError(t, err)
	require.NotEmpty(t, directors)

	// Denis Villeneuve: 2 watched titles (Dune, Dune 2)
	assert.Equal(t, "Denis Villeneuve", directors[0].Name)
	assert.Equal(t, 2, directors[0].Count)

	// Paul King: 1 watched title (Wonka)
	assert.Equal(t, "Paul King", directors[1].Name)
	assert.Equal(t, 1, directors[1].Count)

	dirNames := make(map[string]int)
	for _, d := range directors {
		dirNames[d.Name] = d.Count
	}
	assert.NotContains(t, dirNames, "Christopher Nolan", "unwatched titles director should not be included")
	assert.NotContains(t, dirNames, "Timothée Chalamet", "actors should not be in top directors")

	// Test GetAll integrates top_actors and top_directors
	resp, err := repo.GetAll(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, resp.TopActors)
	assert.NotEmpty(t, resp.TopDirectors)
	assert.Equal(t, "Timothée Chalamet", resp.TopActors[0].Name)
	assert.Equal(t, "Denis Villeneuve", resp.TopDirectors[0].Name)
}

// Helper to create a title with a rating
func createTitle(t *testing.T, db *sql.DB, name string, titleType model.TitleType, isAnime bool, status model.TitleStatus, rating *int) int64 {
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
	return testutil.CreateTitle(t, db, title, names)
}
