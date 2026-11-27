package repository_test

import (
	"context"
	"database/sql"
	"fmt"
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

	// Dune: Watched movie, directed by Denis Villeneuve, written by Frank Herbert, music by Hans Zimmer, starring Timothée Chalamet & Zendaya
	duneCredits := `[{"name":"Denis Villeneuve","role":"Director"},{"name":"Frank Herbert","role":"Writer"},{"name":"Hans Zimmer","role":"Composer"},{"name":"Timothée Chalamet","role":"Paul Atreides"},{"name":"Zendaya","role":"Chani"}]`
	id1 := createTitle(t, db, "Dune", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(9))
	_, err := db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, duneCredits, id1)
	require.NoError(t, err)

	// Dune 2: Watched movie, directed by Denis Villeneuve, music by Hans Zimmer, starring Timothée Chalamet & Florence Pugh
	dune2Credits := `[{"name":"Denis Villeneuve","role":"Director"},{"name":"Hans Zimmer","role":"Composer"},{"name":"Timothée Chalamet","role":"Paul Atreides"},{"name":"Florence Pugh","role":"Princess Irulan"}]`
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
	assert.NotContains(t, actorNames, "Frank Herbert", "writers should not be in top actors")
	assert.NotContains(t, actorNames, "Hans Zimmer", "composers should not be in top actors")

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

func TestStatsRepository_GetWrappedData(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	duneID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2026, Runtime: ptr(166),
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
		MyRating: ptr(9),
	}, []model.TitleName{{Name: "Dune: Part Two", Language: "en", IsPrimary: true}})

	shogunID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, Year: 2026, Runtime: ptr(60),
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
		MyRating: ptr(10),
	}, []model.TitleName{{Name: "Shōgun", Language: "en", IsPrimary: true}})

	solID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, IsAnime: true, Year: 2026, Runtime: ptr(24),
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
		MyRating: ptr(8),
	}, []model.TitleName{{Name: "Solo Leveling", Language: "en", IsPrimary: true}})

	// Add watch events in 2026
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: duneID, Source: model.WatchEventSourcePlex, CreatedAt: time.Date(2026, 3, 10, 21, 0, 0, 0, time.UTC)})
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: duneID, Source: model.WatchEventSourcePlex, CreatedAt: time.Date(2026, 4, 15, 22, 0, 0, 0, time.UTC)})
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: shogunID, Source: model.WatchEventSourcePlex, CreatedAt: time.Date(2026, 5, 1, 20, 0, 0, 0, time.UTC)})
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: solID, Source: model.WatchEventSourcePlex, CreatedAt: time.Date(2026, 6, 1, 14, 0, 0, 0, time.UTC)})

	raw, resp, err := repo.GetWrappedData(context.Background(), 2026)
	require.NoError(t, err)
	require.NotNil(t, raw)
	require.NotNil(t, resp)

	assert.Equal(t, 2026, resp.Year)
	assert.Equal(t, 3, resp.Overview.TotalTitles)
	assert.Equal(t, 1, resp.Overview.TotalMovies)
	assert.Equal(t, 1, resp.Overview.TotalSeries)
	assert.Equal(t, 1, resp.Overview.TotalAnime)
	assert.NotEmpty(t, resp.TopFavorites.Movies)
	assert.Equal(t, "Dune: Part Two", resp.TopFavorites.Movies[0].Title)
	assert.NotEmpty(t, resp.TopFavorites.Series)
	assert.Equal(t, "Shōgun", resp.TopFavorites.Series[0].Title)
	assert.NotEmpty(t, resp.TopFavorites.Anime)
	assert.Equal(t, "Solo Leveling", resp.TopFavorites.Anime[0].Title)

	require.NotNil(t, resp.RewatchChampion)
	assert.Equal(t, "Dune: Part Two", resp.RewatchChampion.Title.Title)
	assert.Equal(t, 2, resp.RewatchChampion.TotalPlays)
}

func TestStatsRepository_GetWrappedData_ManualVsAutomatedScrobbles(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	// Movie A: 5 manual events (should count as 1 play)
	manualMovieID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2026, Runtime: ptr(120),
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Manual Movie", Language: "en", IsPrimary: true}})

	for i := 0; i < 5; i++ {
		testutil.CreateWatchEvent(t, db, &model.WatchEvent{
			TitleID:   manualMovieID,
			Source:    model.WatchEventSourceManual,
			CreatedAt: time.Date(2026, 6, 20, 14, 48, 36, 0, time.UTC),
		})
	}

	// Movie B: 3 Plex/Jellyfin events (should count as 3 plays)
	plexMovieID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2026, Runtime: ptr(120),
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Plex Movie", Language: "en", IsPrimary: true}})

	for i := 0; i < 3; i++ {
		testutil.CreateWatchEvent(t, db, &model.WatchEvent{
			TitleID:   plexMovieID,
			Source:    model.WatchEventSourcePlex,
			CreatedAt: time.Date(2026, time.Month(1+i), 10, 20, 0, 0, 0, time.UTC),
		})
	}

	_, resp, err := repo.GetWrappedData(context.Background(), 2026)
	require.NoError(t, err)
	require.NotNil(t, resp.RewatchChampion)
	assert.Equal(t, "Plex Movie", resp.RewatchChampion.Title.Title)
	assert.Equal(t, 3, resp.RewatchChampion.TotalPlays)
}

func TestStatsRepository_GetFiltered_MediaType(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	createTitle(t, db, "Movie 1", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(8))
	createTitle(t, db, "Series 1", model.TitleTypeSeries, false, model.TitleStatusCompleted, ptr(9))
	createTitle(t, db, "Anime 1", model.TitleTypeSeries, true, model.TitleStatusCompleted, ptr(10))

	// All
	respAll, err := repo.GetFiltered(context.Background(), model.StatsFilter{Timeframe: "all", MediaType: "all"})
	require.NoError(t, err)
	assert.Equal(t, 3, respAll.Overview.TotalTitles)

	// Movies
	respMovie, err := repo.GetFiltered(context.Background(), model.StatsFilter{Timeframe: "all", MediaType: "movie"})
	require.NoError(t, err)
	assert.Equal(t, 1, respMovie.Overview.TotalTitles)
	assert.Equal(t, 1, respMovie.Overview.TotalMovies)
	assert.Equal(t, 0, respMovie.Overview.TotalSeries)

	// Series
	respSeries, err := repo.GetFiltered(context.Background(), model.StatsFilter{Timeframe: "all", MediaType: "series"})
	require.NoError(t, err)
	assert.Equal(t, 1, respSeries.Overview.TotalTitles)
	assert.Equal(t, 1, respSeries.Overview.TotalSeries)
	assert.Equal(t, 0, respSeries.Overview.TotalAnime)

	// Anime
	respAnime, err := repo.GetFiltered(context.Background(), model.StatsFilter{Timeframe: "all", MediaType: "anime"})
	require.NoError(t, err)
	assert.Equal(t, 1, respAnime.Overview.TotalTitles)
	assert.Equal(t, 1, respAnime.Overview.TotalAnime)
}

func TestStatsRepository_GetFiltered_Timeframe(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	mID := createTitle(t, db, "Dune", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(8))
	sID := createTitle(t, db, "Dark", model.TitleTypeSeries, false, model.TitleStatusCompleted, ptr(9))

	// Add 2024 watch event for Dune
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{
		TitleID:   mID,
		Source:    model.WatchEventSourcePlex,
		CreatedAt: time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC),
	})

	// Add recent watch event for Dark (today)
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{
		TitleID:   sID,
		Source:    model.WatchEventSourcePlex,
		CreatedAt: time.Now(),
	})

	// Filter by 2024
	resp2024, err := repo.GetFiltered(context.Background(), model.StatsFilter{Timeframe: "year", Year: 2024, MediaType: "all"})
	require.NoError(t, err)
	assert.Equal(t, 1, resp2024.Overview.TotalTitles)
	assert.Equal(t, 1, resp2024.Overview.TotalMovies)
	assert.Equal(t, 0, resp2024.Overview.TotalSeries)

	// Filter by 30d
	resp30d, err := repo.GetFiltered(context.Background(), model.StatsFilter{Timeframe: "30d", MediaType: "all"})
	require.NoError(t, err)
	assert.Equal(t, 1, resp30d.Overview.TotalTitles)
	assert.Equal(t, 1, resp30d.Overview.TotalSeries)
}

func TestStatsRepository_AvailableYears(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 1982,
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Blade Runner", Language: "en", IsPrimary: true}})

	currentYear := time.Now().Year()

	// Without watch events, only current year is returned (not 1982)
	years, err := repo.AvailableYears(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int{currentYear}, years)

	// Add watch event in 2024
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{
		TitleID:   titleID,
		Source:    model.WatchEventSourcePlex,
		CreatedAt: time.Date(2024, 6, 15, 20, 0, 0, 0, time.UTC),
	})

	yearsWithWatch, err := repo.AvailableYears(context.Background())
	require.NoError(t, err)
	assert.Contains(t, yearsWithWatch, currentYear)
	assert.Contains(t, yearsWithWatch, 2024)
	assert.NotContains(t, yearsWithWatch, 1982)
}

func TestStatsRepo_TopWriters_And_Composers(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	// Movie 1 (completed): Multi-role person (Director + Writer + Cast), plus Writer & Composer
	c1 := `[{"name":"Quentin Tarantino","role":"Director"},{"name":"Quentin Tarantino","role":"Writer"},{"name":"Quentin Tarantino","role":"Jimmie"},{"name":"Roger Avary","role":"Writer"},{"name":"Hans Zimmer","role":"Composer"},{"name":"Uma Thurman","role":"Mia"}]`
	id1 := createTitle(t, db, "Movie One", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(9))
	_, err := db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, c1, id1)
	require.NoError(t, err)

	// Series 1 (watching): Duplicate Writer entry on same title should still count once
	c2 := `[{"name":"Craig Mazin","role":"Writer"},{"name":"Craig Mazin","role":"Writer"},{"name":"Hans Zimmer","role":"Composer"},{"name":"Pedro Pascal","role":"Joel"}]`
	id2 := createTitle(t, db, "Series One", model.TitleTypeSeries, false, model.TitleStatusWatching, ptr(9))
	_, err = db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, c2, id2)
	require.NoError(t, err)

	// Anime 1 (dropped WITH last_watched_at set): should count because it was watched
	c3 := `[{"name":"Craig Mazin","role":"Writer"},{"name":"Yoko Kanno","role":"Composer"},{"name":"Spike Actor","role":"Spike"}]`
	id3 := createTitle(t, db, "Anime Watched Dropped", model.TitleTypeSeries, true, model.TitleStatusDropped, ptr(8))
	_, err = db.Exec(`UPDATE titles SET credits = ?, last_watched_at = CURRENT_TIMESTAMP WHERE id = ?`, c3, id3)
	require.NoError(t, err)

	// Unwatched plan_to_watch and unwatched dropped (last_watched_at IS NULL): must NOT count
	c4 := `[{"name":"Unwatched Writer","role":"Writer"},{"name":"Unwatched Composer","role":"Composer"},{"name":"Unwatched Actor","role":"Lead"}]`
	id4 := createTitle(t, db, "Plan Movie", model.TitleTypeMovie, false, model.TitleStatusPlanToWatch, nil)
	_, err = db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, c4, id4)
	require.NoError(t, err)

	id5 := createTitle(t, db, "Never Watched Dropped", model.TitleTypeMovie, false, model.TitleStatusDropped, nil)
	_, err = db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, c4, id5)
	require.NoError(t, err)

	// Direct repository convenience methods
	writers, err := repo.TopWriters(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, []model.PersonStat{
		{Name: "Craig Mazin", Count: 2},
		{Name: "Quentin Tarantino", Count: 1},
		{Name: "Roger Avary", Count: 1},
	}, writers)

	composers, err := repo.TopComposers(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, []model.PersonStat{
		{Name: "Hans Zimmer", Count: 2},
		{Name: "Yoko Kanno", Count: 1},
	}, composers)

	// GetAll integrates top_writers, top_composers, top_directors, top_actors
	resp, err := repo.GetAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, writers, resp.TopWriters)
	assert.Equal(t, composers, resp.TopComposers)
	assert.Equal(t, []model.PersonStat{{Name: "Quentin Tarantino", Count: 1}}, resp.TopDirectors)

	actorCounts := make(map[string]int)
	for _, a := range resp.TopActors {
		actorCounts[a.Name] = a.Count
	}
	assert.Equal(t, 1, actorCounts["Uma Thurman"])
	assert.Equal(t, 1, actorCounts["Pedro Pascal"])
	assert.Equal(t, 1, actorCounts["Spike Actor"])
	// Quentin Tarantino has a cast entry ("Jimmie"), so that cast entry counts; pure crew roles must not.
	assert.Equal(t, 1, actorCounts["Quentin Tarantino"])
	assert.NotContains(t, actorCounts, "Craig Mazin")
	assert.NotContains(t, actorCounts, "Roger Avary")
	assert.NotContains(t, actorCounts, "Hans Zimmer")
	assert.NotContains(t, actorCounts, "Yoko Kanno")
	assert.NotContains(t, actorCounts, "Unwatched Actor")
}

func TestStatsRepo_CrewRankings_Filters(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewStatsRepository(db)

	now := time.Now().UTC()
	recentTime := now.AddDate(0, 0, -5)
	pastYearTime := time.Date(2024, 6, 15, 20, 0, 0, 0, time.UTC)

	movieID := createTitle(t, db, "Movie Filter", model.TitleTypeMovie, false, model.TitleStatusCompleted, ptr(9))
	_, err := db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`,
		`[{"name":"Dir Movie","role":"Director"},{"name":"Writer Movie","role":"Writer"},{"name":"Comp Movie","role":"Composer"},{"name":"Actor Movie","role":"Hero"}]`, movieID)
	require.NoError(t, err)

	seriesID := createTitle(t, db, "Series Filter", model.TitleTypeSeries, false, model.TitleStatusCompleted, ptr(8))
	_, err = db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`,
		`[{"name":"Dir Series","role":"Director"},{"name":"Writer Series","role":"Writer"},{"name":"Comp Series","role":"Composer"},{"name":"Actor Series","role":"Detective"}]`, seriesID)
	require.NoError(t, err)

	animeID := createTitle(t, db, "Anime Filter", model.TitleTypeSeries, true, model.TitleStatusWatching, ptr(10))
	_, err = db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`,
		`[{"name":"Dir Anime","role":"Director"},{"name":"Writer Anime","role":"Writer"},{"name":"Comp Anime","role":"Composer"},{"name":"Actor Anime","role":"Protagonist"}]`, animeID)
	require.NoError(t, err)

	// Multiple watch events in 2024 for movie (must still count distinct title = 1)
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: movieID, Source: model.WatchEventSourcePlex, CreatedAt: pastYearTime})
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: movieID, Source: model.WatchEventSourcePlex, CreatedAt: pastYearTime.Add(2 * time.Hour)})
	// Series watched in 2024
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: seriesID, Source: model.WatchEventSourcePlex, CreatedAt: pastYearTime})
	// Anime watched in last 30 days
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: animeID, Source: model.WatchEventSourcePlex, CreatedAt: recentTime})

	cases := []struct {
		name          string
		filter        model.StatsFilter
		wantActors    []model.PersonStat
		wantDirectors []model.PersonStat
		wantWriters   []model.PersonStat
		wantComposers []model.PersonStat
	}{
		{
			name:          "media_type=movie",
			filter:        model.StatsFilter{Timeframe: "all", MediaType: "movie"},
			wantActors:    []model.PersonStat{{Name: "Actor Movie", Count: 1}},
			wantDirectors: []model.PersonStat{{Name: "Dir Movie", Count: 1}},
			wantWriters:   []model.PersonStat{{Name: "Writer Movie", Count: 1}},
			wantComposers: []model.PersonStat{{Name: "Comp Movie", Count: 1}},
		},
		{
			name:          "media_type=series excludes anime",
			filter:        model.StatsFilter{Timeframe: "all", MediaType: "series"},
			wantActors:    []model.PersonStat{{Name: "Actor Series", Count: 1}},
			wantDirectors: []model.PersonStat{{Name: "Dir Series", Count: 1}},
			wantWriters:   []model.PersonStat{{Name: "Writer Series", Count: 1}},
			wantComposers: []model.PersonStat{{Name: "Comp Series", Count: 1}},
		},
		{
			name:          "media_type=anime",
			filter:        model.StatsFilter{Timeframe: "all", MediaType: "anime"},
			wantActors:    []model.PersonStat{{Name: "Actor Anime", Count: 1}},
			wantDirectors: []model.PersonStat{{Name: "Dir Anime", Count: 1}},
			wantWriters:   []model.PersonStat{{Name: "Writer Anime", Count: 1}},
			wantComposers: []model.PersonStat{{Name: "Comp Anime", Count: 1}},
		},
		{
			name:          "timeframe=year year=2024 distinct title counts",
			filter:        model.StatsFilter{Timeframe: "year", Year: 2024, MediaType: "all"},
			wantActors:    []model.PersonStat{{Name: "Actor Movie", Count: 1}, {Name: "Actor Series", Count: 1}},
			wantDirectors: []model.PersonStat{{Name: "Dir Movie", Count: 1}, {Name: "Dir Series", Count: 1}},
			wantWriters:   []model.PersonStat{{Name: "Writer Movie", Count: 1}, {Name: "Writer Series", Count: 1}},
			wantComposers: []model.PersonStat{{Name: "Comp Movie", Count: 1}, {Name: "Comp Series", Count: 1}},
		},
		{
			name:          "timeframe=30d",
			filter:        model.StatsFilter{Timeframe: "30d", MediaType: "all"},
			wantActors:    []model.PersonStat{{Name: "Actor Anime", Count: 1}},
			wantDirectors: []model.PersonStat{{Name: "Dir Anime", Count: 1}},
			wantWriters:   []model.PersonStat{{Name: "Writer Anime", Count: 1}},
			wantComposers: []model.PersonStat{{Name: "Comp Anime", Count: 1}},
		},
		{
			name:          "timeframe=year year=2024 and media_type=anime yields non-nil empty",
			filter:        model.StatsFilter{Timeframe: "year", Year: 2024, MediaType: "anime"},
			wantActors:    []model.PersonStat{},
			wantDirectors: []model.PersonStat{},
			wantWriters:   []model.PersonStat{},
			wantComposers: []model.PersonStat{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := repo.GetFiltered(context.Background(), tc.filter)
			require.NoError(t, err)
			assert.NotNil(t, resp.TopActors)
			assert.NotNil(t, resp.TopDirectors)
			assert.NotNil(t, resp.TopWriters)
			assert.NotNil(t, resp.TopComposers)
			assert.Equal(t, tc.wantActors, resp.TopActors)
			assert.Equal(t, tc.wantDirectors, resp.TopDirectors)
			assert.Equal(t, tc.wantWriters, resp.TopWriters)
			assert.Equal(t, tc.wantComposers, resp.TopComposers)
		})
	}
}

func TestStatsRepo_CrewRankings_BoundaryAndError(t *testing.T) {
	t.Run("empty database returns non-nil empty slices", func(t *testing.T) {
		db := setupTestDB(t)
		repo := repository.NewStatsRepository(db)

		resp, err := repo.GetFiltered(context.Background(), model.StatsFilter{Timeframe: "all", MediaType: "all"})
		require.NoError(t, err)
		assert.NotNil(t, resp.TopActors)
		assert.NotNil(t, resp.TopDirectors)
		assert.NotNil(t, resp.TopWriters)
		assert.NotNil(t, resp.TopComposers)
		assert.Empty(t, resp.TopActors)
		assert.Empty(t, resp.TopDirectors)
		assert.Empty(t, resp.TopWriters)
		assert.Empty(t, resp.TopComposers)
	})

	t.Run("top 10 cap and deterministic tie-breaking", func(t *testing.T) {
		db := setupTestDB(t)
		repo := repository.NewStatsRepository(db)

		// Create 12 writers & composers: Writer 01 / Composer 01 get 2 titles, 02..12 get 1 title.
		for i := 1; i <= 12; i++ {
			wName := fmt.Sprintf("Writer %02d", i)
			cName := fmt.Sprintf("Composer %02d", i)
			credits := fmt.Sprintf(`[{"name":%q,"role":"Writer"},{"name":%q,"role":"Composer"}]`, wName, cName)
			id := createTitle(t, db, fmt.Sprintf("Cap Title %02d", i), model.TitleTypeMovie, false, model.TitleStatusCompleted, nil)
			_, err := db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, credits, id)
			require.NoError(t, err)
		}
		// Give Writer 01 and Composer 01 a second title so they sort first with count=2
		extraID := createTitle(t, db, "Cap Title Extra", model.TitleTypeMovie, false, model.TitleStatusCompleted, nil)
		_, err := db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`,
			`[{"name":"Writer 01","role":"Writer"},{"name":"Composer 01","role":"Composer"}]`, extraID)
		require.NoError(t, err)

		resp, err := repo.GetAll(context.Background())
		require.NoError(t, err)
		require.Len(t, resp.TopWriters, 10)
		require.Len(t, resp.TopComposers, 10)
		assert.Equal(t, model.PersonStat{Name: "Writer 01", Count: 2}, resp.TopWriters[0])
		assert.Equal(t, model.PersonStat{Name: "Writer 10", Count: 1}, resp.TopWriters[9])
		assert.Equal(t, model.PersonStat{Name: "Composer 01", Count: 2}, resp.TopComposers[0])
		assert.Equal(t, model.PersonStat{Name: "Composer 10", Count: 1}, resp.TopComposers[9])
	})

	t.Run("malformed, non-array, and blank credits do not fail queries", func(t *testing.T) {
		db := setupTestDB(t)
		repo := repository.NewStatsRepository(db)

		badRows := []string{`{malformed`, `{"name":"ObjectNotArray","role":"Writer"}`, `[123, null, "str"]`, `[{"name":"  ","role":"Writer"},{"name":"BlankRole","role":""}]`}
		for i, raw := range badRows {
			id := createTitle(t, db, fmt.Sprintf("Bad Credits %d", i), model.TitleTypeMovie, false, model.TitleStatusCompleted, nil)
			_, err := db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, raw, id)
			require.NoError(t, err)
		}
		validID := createTitle(t, db, "Valid Credits", model.TitleTypeMovie, false, model.TitleStatusCompleted, nil)
		_, err := db.Exec(`UPDATE titles SET credits = ? WHERE id = ?`, `[{"name":"Valid Writer","role":"Writer"}]`, validID)
		require.NoError(t, err)

		resp, err := repo.GetAll(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []model.PersonStat{{Name: "Valid Writer", Count: 1}}, resp.TopWriters)
		assert.Empty(t, resp.TopActors)
		assert.Empty(t, resp.TopDirectors)
		assert.Empty(t, resp.TopComposers)
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		db := setupTestDB(t)
		repo := repository.NewStatsRepository(db)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := repo.GetFiltered(ctx, model.StatsFilter{Timeframe: "all", MediaType: "all"})
		assert.Error(t, err)
		_, err = repo.TopWriters(ctx, 10)
		assert.Error(t, err)
		_, err = repo.TopComposers(ctx, 10)
		assert.Error(t, err)
	})
}
