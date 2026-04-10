package repository_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })
	return db
}

func TestTitleRepository_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	title := &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}
	names := []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}}

	id, err := repo.Create(title, names)
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, "Dune", got.PrimaryName())
	assert.Equal(t, model.TitleTypeMovie, got.Type)
	assert.Equal(t, 2024, got.Year)
}

func TestTitleRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	_, err := repo.GetByID(99999)
	assert.Error(t, err)
}

func TestTitleRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)

	// Create test data
	idA, _ := repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})
	idB, _ := repo.Create(&model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusPendingReview}, []model.TitleName{{Name: "Shogun", Language: "en", IsPrimary: true}})
	idC, _ := repo.Create(&model.Title{Type: model.TitleTypeSeries, IsAnime: true, Year: 2022, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Naruto", Language: "en", IsPrimary: true}})

	// Create episodes for "up to date" and "watching behind" tests
	// Naruto: 1 season, 2 episodes, all watched → "up to date"
	sN, _ := seasonRepo.GetOrCreate(idC, 1)
	eN1, _ := episodeRepo.GetOrCreate(sN.ID, 1)
	_, _ = episodeRepo.ToggleWatched(eN1.ID)
	eN2, _ := episodeRepo.GetOrCreate(sN.ID, 2)
	_, _ = episodeRepo.ToggleWatched(eN2.ID)

	// Add a series "watching behind": series with unwatched episodes
	idD, _ := repo.Create(&model.Title{Type: model.TitleTypeSeries, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "The Bear", Language: "en", IsPrimary: true}})
	sD, _ := seasonRepo.GetOrCreate(idD, 1)
	_, _ = episodeRepo.GetOrCreate(sD.ID, 1) // unwatched

	_ = idA
	_ = idB

	tests := []struct {
		name     string
		filter   repository.TitleFilter
		wantLen  int
		wantName string // first result name (if wantLen > 0)
	}{
		{
			name:    "filter by status watching",
			filter:  repository.TitleFilter{Status: ptr(model.TitleStatusWatching)},
			wantLen: 3,
		},
		{
			name:     "filter by status completed",
			filter:   repository.TitleFilter{Status: ptr(model.TitleStatusCompleted)},
			wantLen:  1,
			wantName: "Shogun",
		},
		{
			name:     "filter by type movie",
			filter:   repository.TitleFilter{Type: ptr(model.TitleTypeMovie)},
			wantLen:  1,
			wantName: "Dune",
		},
		{
			name:     "filter by match_status pending_review",
			filter:   repository.TitleFilter{MatchStatus: ptr(model.MatchStatusPendingReview)},
			wantLen:  1,
			wantName: "Shogun",
		},
		{
			name:    "pagination limit",
			filter:  repository.TitleFilter{Limit: 2},
			wantLen: 2,
		},
		{
			name:    "pagination offset past end",
			filter:  repository.TitleFilter{Offset: 100},
			wantLen: 0,
		},
		{
			name:     "up to date filter",
			filter:   repository.TitleFilter{UpToDate: true},
			wantLen:  1,
			wantName: "Naruto",
		},
		{
			name:    "watching behind filter",
			filter:  repository.TitleFilter{WatchingBehind: true},
			wantLen: 2, // Dune (movie) + The Bear (has unwatched)
		},
		{
			name:    "no results for dropped",
			filter:  repository.TitleFilter{Status: ptr(model.TitleStatusDropped)},
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := repo.List(tc.filter)
			require.NoError(t, err)
			assert.Len(t, result.Titles, tc.wantLen)
			if tc.wantLen > 0 && tc.wantName != "" {
				assert.Equal(t, tc.wantName, result.Titles[0].PrimaryName())
			}
		})
	}
}

func TestTitleRepository_ListBySearch(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Dune: Part Two", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Oppenheimer", Language: "en", IsPrimary: true}})

	result, err := repo.List(repository.TitleFilter{Search: ptr("dune")})
	require.NoError(t, err)
	assert.Len(t, result.Titles, 1)
	assert.Equal(t, "Dune: Part Two", result.Titles[0].PrimaryName())
}

func TestTitleRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	id, _ := repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	err := repo.Update(id, repository.TitleUpdate{Status: ptr(model.TitleStatusCompleted), MyRating: ptr(8)})
	require.NoError(t, err)

	got, _ := repo.GetByID(id)
	assert.Equal(t, model.TitleStatusCompleted, got.Status)
	assert.Equal(t, 8, *got.MyRating)
}

func TestTitleRepository_UpdateNoFields(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	id, _ := repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	// Update with no fields should be a no-op
	err := repo.Update(id, repository.TitleUpdate{})
	require.NoError(t, err)

	got, _ := repo.GetByID(id)
	assert.Equal(t, model.TitleStatusWatching, got.Status)
}

func TestTitleRepository_FindByExternalID(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	imdb := "tt1234567"
	tmdb := int64(12345)
	anilist := int64(67890)
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed, IMDBID: &imdb, TMDBID: &tmdb, AniListID: &anilist}, []model.TitleName{{Name: "Test Movie", Language: "en", IsPrimary: true}})

	tests := []struct {
		name      string
		imdbID    *string
		tmdbID    *int64
		anilistID *int64
		plexKey   *string
		wantErr   bool
		wantTitle string
	}{
		{
			name:      "find by IMDB",
			imdbID:    &imdb,
			wantTitle: "Test Movie",
		},
		{
			name:      "find by TMDB",
			tmdbID:    &tmdb,
			wantTitle: "Test Movie",
		},
		{
			name:      "find by AniList",
			anilistID: &anilist,
			wantTitle: "Test Movie",
		},
		{
			name:    "not found",
			imdbID:  ptr("tt9999999"),
			wantErr: true,
		},
		{
			name:    "no IDs provided",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.FindByExternalID(tc.imdbID, tc.tmdbID, tc.plexKey, tc.anilistID, nil)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantTitle, got.PrimaryName())
		})
	}
}

func TestTitleRepository_FindByExternalID_TypeFilter(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	// Same TMDB ID for a movie and a series (real-world: TMDB 1891 = "The Empire Strikes Back" movie + "Rome" series)
	tmdb := int64(1891)
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 1980, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, TMDBID: &tmdb}, []model.TitleName{{Name: "The Empire Strikes Back", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeSeries, Year: 2005, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, TMDBID: &tmdb}, []model.TitleName{{Name: "Rome", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeSeries, IsAnime: true, Year: 2002, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Naruto", Language: "en", IsPrimary: true}})

	movieType := model.TitleTypeMovie
	seriesType := model.TitleTypeSeries

	t.Run("without type filter returns first match", func(t *testing.T) {
		got, err := repo.FindByExternalID(nil, &tmdb, nil, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, []string{"The Empire Strikes Back", "Rome"}, got.PrimaryName())
	})

	t.Run("filter by movie type", func(t *testing.T) {
		got, err := repo.FindByExternalID(nil, &tmdb, nil, nil, &movieType)
		require.NoError(t, err)
		assert.Equal(t, "The Empire Strikes Back", got.PrimaryName())
	})

	t.Run("filter by series type", func(t *testing.T) {
		got, err := repo.FindByExternalID(nil, &tmdb, nil, nil, &seriesType)
		require.NoError(t, err)
		assert.Equal(t, "Rome", got.PrimaryName())
	})

	t.Run("filter by anime via is_anime", func(t *testing.T) {
		// Naruto was created with IsAnime: true
		isAnime := true
		result, err := repo.List(repository.TitleFilter{IsAnime: &isAnime})
		require.NoError(t, err)
		assert.Len(t, result.Titles, 1)
		assert.Equal(t, "Naruto", result.Titles[0].PrimaryName())
	})
}

func TestTitleRepository_GetStatusCounts(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "A", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusPendingReview}, []model.TitleName{{Name: "B", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusUnconfirmed}, []model.TitleName{{Name: "C", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusUnconfirmed}, []model.TitleName{{Name: "D", Language: "en", IsPrimary: true}})

	counts, err := repo.GetStatusCounts()
	require.NoError(t, err)
	assert.Equal(t, 1, counts.PendingReview)
	assert.Equal(t, 2, counts.Unconfirmed)
}

func TestTitleRepository_List_SortByYear(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2020, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Old", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "New", Language: "en", IsPrimary: true}})

	result, err := repo.List(repository.TitleFilter{
		Status: ptr(model.TitleStatusCompleted),
		Sort:   "year",
		Order:  "asc",
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 2)
	assert.Equal(t, "Old", result.Titles[0].PrimaryName())
	assert.Equal(t, "New", result.Titles[1].PrimaryName())
}

func TestTitleRepository_List_SortByRating_NullsLast(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	rating := 8
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, MyRating: &rating}, []model.TitleName{{Name: "Rated", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Unrated", Language: "en", IsPrimary: true}})

	result, err := repo.List(repository.TitleFilter{
		Status: ptr(model.TitleStatusCompleted),
		Sort:   "my_rating",
		Order:  "desc",
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 2)
	assert.Equal(t, "Rated", result.Titles[0].PrimaryName())
	assert.Equal(t, "Unrated", result.Titles[1].PrimaryName())
}

func TestTitleRepository_List_SortByReleaseDate(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	rd1 := "2020-03-15"
	rd2 := "2024-11-01"
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2020, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd1}, []model.TitleName{{Name: "Old Movie", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd2}, []model.TitleName{{Name: "New Movie", Language: "en", IsPrimary: true}})

	result, err := repo.List(repository.TitleFilter{
		Status: ptr(model.TitleStatusCompleted),
		Sort:   "release_date",
		Order:  "asc",
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 2)
	assert.Equal(t, "Old Movie", result.Titles[0].PrimaryName())
	assert.Equal(t, "New Movie", result.Titles[1].PrimaryName())
}

func TestTitleRepository_List_SortByReleaseDate_NullsLast(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	rd := "2024-06-01"
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd}, []model.TitleName{{Name: "With Date", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "No Date", Language: "en", IsPrimary: true}})

	result, err := repo.List(repository.TitleFilter{
		Status: ptr(model.TitleStatusCompleted),
		Sort:   "release_date",
		Order:  "desc",
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 2)
	assert.Equal(t, "With Date", result.Titles[0].PrimaryName())
	assert.Equal(t, "No Date", result.Titles[1].PrimaryName())
}

func TestTitleRepository_List_FilterByDecade(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2015, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "2010s Movie", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2022, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "2020s Movie", Language: "en", IsPrimary: true}})

	decade := 2020
	result, err := repo.List(repository.TitleFilter{
		Status: ptr(model.TitleStatusCompleted),
		Decade: &decade,
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 1)
	assert.Equal(t, "2020s Movie", result.Titles[0].PrimaryName())
}

func TestTitleRepository_List_FilterByDateRange(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	rd1 := "2023-01-15"
	rd2 := "2024-06-20"
	rd3 := "2025-01-01"
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd1}, []model.TitleName{{Name: "Early", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd2}, []model.TitleName{{Name: "Mid", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2025, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd3}, []model.TitleName{{Name: "Late", Language: "en", IsPrimary: true}})

	from := "2024-01-01"
	to := "2024-12-31"
	result, err := repo.List(repository.TitleFilter{
		Status:      ptr(model.TitleStatusCompleted),
		ReleaseFrom: &from,
		ReleaseTo:   &to,
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 1)
	assert.Equal(t, "Mid", result.Titles[0].PrimaryName())
}

func TestTitleRepository_List_FilterByDateRange_ExcludeNoRelease(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	rd := "2024-06-20"
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd}, []model.TitleName{{Name: "With Date", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "No Date", Language: "en", IsPrimary: true}})

	from := "2024-01-01"
	// IncludeNoRelease = true → include NULL release_date
	result, err := repo.List(repository.TitleFilter{
		Status:           ptr(model.TitleStatusCompleted),
		ReleaseFrom:      &from,
		IncludeNoRelease: true,
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 2)

	// IncludeNoRelease = false → exclude NULL release_date
	result2, err := repo.List(repository.TitleFilter{
		Status:           ptr(model.TitleStatusCompleted),
		ReleaseFrom:      &from,
		IncludeNoRelease: false,
	})
	require.NoError(t, err)
	require.Len(t, result2.Titles, 1)
	assert.Equal(t, "With Date", result2.Titles[0].PrimaryName())
}

func TestTitleRepository_MetadataRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	genres := `["Science Fiction","Drama"]`
	credits := `[{"name":"Jack Arnold","role":"Director"},{"name":"Michel Ray","role":"Bud"}]`
	overview := "A mysterious brain from space..."
	runtime := 69
	tmdbRating := 5.2

	title := &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        1958,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		Overview:    &overview,
		Genres:      &genres,
		Runtime:     &runtime,
		TMDBRating:  &tmdbRating,
		Credits:     &credits,
	}

	id, err := repo.Create(title, []model.TitleName{{Name: "The Space Children", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, &overview, got.Overview)
	assert.Equal(t, &genres, got.Genres)
	assert.Equal(t, &runtime, got.Runtime)
	assert.Equal(t, &tmdbRating, got.TMDBRating)
	assert.Equal(t, &credits, got.Credits)
}

func TestTitleRepository_UpdateMetadata(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	title := &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        1958,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
	}

	id, err := repo.Create(title, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	overview := "Updated overview"
	genres := `["Action"]`
	runtime := 120
	tmdbRating := 7.5
	credits := `[{"name":"Director","role":"Director"}]`

	err = repo.Update(id, repository.TitleUpdate{
		Overview:   &overview,
		Genres:     &genres,
		Runtime:    &runtime,
		TMDBRating: &tmdbRating,
		Credits:    &credits,
	})
	require.NoError(t, err)

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, &overview, got.Overview)
	assert.Equal(t, &genres, got.Genres)
	assert.Equal(t, &runtime, got.Runtime)
	assert.Equal(t, &tmdbRating, got.TMDBRating)
	assert.Equal(t, &credits, got.Credits)
}

func TestTitleRepository_List_SortByLastWatched(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	idA, _ := repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "A", Language: "en", IsPrimary: true}})
	idB, _ := repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "B", Language: "en", IsPrimary: true}})
	idC, _ := repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "C", Language: "en", IsPrimary: true}})

	// Set last_watched_at in specific order: C (older), then A (newer), then B (not watched)
	dateC, _ := time.Parse(time.RFC3339, "2024-01-01T10:00:00Z")
	dateA, _ := time.Parse(time.RFC3339, "2024-01-02T10:00:00Z")
	_ = repo.UpdateLastWatchedAt(idC, dateC)
	_ = repo.UpdateLastWatchedAt(idA, dateA)
	_ = idB // explicitly ignore idB as it has no events

	// Sort DESC: A (newest), then C (older), then B (null)
	result, err := repo.List(repository.TitleFilter{
		Sort:  "last_watched_at",
		Order: "desc",
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 3)
	assert.Equal(t, "A", result.Titles[0].PrimaryName())
	assert.Equal(t, "C", result.Titles[1].PrimaryName())
	assert.Equal(t, "B", result.Titles[2].PrimaryName())
	assert.NotNil(t, result.Titles[0].LastWatchedAt)
	assert.NotNil(t, result.Titles[1].LastWatchedAt)
	assert.Nil(t, result.Titles[2].LastWatchedAt)

	// Sort ASC: C (older), then A (newer), then B (null last)
	result2, err := repo.List(repository.TitleFilter{
		Sort:  "last_watched_at",
		Order: "asc",
	})
	require.NoError(t, err)
	require.Len(t, result2.Titles, 3)
	assert.Equal(t, "C", result2.Titles[0].PrimaryName())
	assert.Equal(t, "A", result2.Titles[1].PrimaryName())
	assert.Equal(t, "B", result2.Titles[2].PrimaryName())
}

func ptr[T any](v T) *T { return &v }
