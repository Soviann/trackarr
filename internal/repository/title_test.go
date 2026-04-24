package repository_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
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

// createTestTitle inserts a minimal title and returns it with its assigned ID.
func createTestTitle(t *testing.T, db *sql.DB, titleType string, runtime int) *model.Title {
	t.Helper()
	title := &model.Title{
		Type:        model.TitleType(titleType),
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		Runtime:     &runtime,
	}
	id := testutil.CreateTitle(t, db, title, []model.TitleName{{Name: "Test Title", Language: "en", IsPrimary: true}})
	title.ID = id
	return title
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

	id := testutil.CreateTitle(t, db, title, names)
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

	// Create test data
	idA := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})
	idB := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusPendingReview}, []model.TitleName{{Name: "Shogun", Language: "en", IsPrimary: true}})
	idC := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, IsAnime: true, Year: 2022, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Naruto", Language: "en", IsPrimary: true}})

	// Create episodes for "up to date" and "watching behind" tests
	// Naruto: 1 season, 2 episodes, all watched → "up to date"
	sN := testutil.GetOrCreateSeason(t, db, idC, 1)
	eN1 := testutil.GetOrCreateEpisode(t, db, sN.ID, 1)
	_ = testutil.ToggleEpisodeWatched(t, db, eN1.ID)
	eN2 := testutil.GetOrCreateEpisode(t, db, sN.ID, 2)
	_ = testutil.ToggleEpisodeWatched(t, db, eN2.ID)

	// Add a series "watching behind": series with unwatched episodes
	idD := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "The Bear", Language: "en", IsPrimary: true}})
	sD := testutil.GetOrCreateSeason(t, db, idD, 1)
	_ = testutil.GetOrCreateEpisode(t, db, sD.ID, 1) // unwatched

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

	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Dune: Part Two", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Oppenheimer", Language: "en", IsPrimary: true}})

	result, err := repo.List(repository.TitleFilter{Search: ptr("dune")})
	require.NoError(t, err)
	assert.Len(t, result.Titles, 1)
	assert.Equal(t, "Dune: Part Two", result.Titles[0].PrimaryName())
}

func TestTitleRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	testutil.UpdateTitle(t, db, id, repository.TitleUpdate{Status: ptr(model.TitleStatusCompleted), MyRating: ptr(8)})

	got, _ := repo.GetByID(id)
	assert.Equal(t, model.TitleStatusCompleted, got.Status)
	assert.Equal(t, 8, *got.MyRating)
}

func TestTitleRepository_UpdateNoFields(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	// Update with no fields should be a no-op
	testutil.UpdateTitle(t, db, id, repository.TitleUpdate{})

	got, _ := repo.GetByID(id)
	assert.Equal(t, model.TitleStatusWatching, got.Status)
}

func TestTitleRepository_FindByExternalID(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	imdb := "tt1234567"
	tmdb := int64(12345)
	anilist := int64(67890)
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed, IMDBID: &imdb, TMDBID: &tmdb, AniListID: &anilist}, []model.TitleName{{Name: "Test Movie", Language: "en", IsPrimary: true}})

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
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 1980, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, TMDBID: &tmdb}, []model.TitleName{{Name: "The Empire Strikes Back", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Year: 2005, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, TMDBID: &tmdb}, []model.TitleName{{Name: "Rome", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, IsAnime: true, Year: 2002, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Naruto", Language: "en", IsPrimary: true}})

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

	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "A", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusPendingReview}, []model.TitleName{{Name: "B", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusUnconfirmed}, []model.TitleName{{Name: "C", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusUnconfirmed}, []model.TitleName{{Name: "D", Language: "en", IsPrimary: true}})

	counts, err := repo.GetStatusCounts()
	require.NoError(t, err)
	assert.Equal(t, 1, counts.PendingReview)
	assert.Equal(t, 2, counts.Unconfirmed)
}

func TestTitleRepository_List_SortByYear(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2020, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Old", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "New", Language: "en", IsPrimary: true}})

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
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, MyRating: &rating}, []model.TitleName{{Name: "Rated", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Unrated", Language: "en", IsPrimary: true}})

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
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2020, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd1}, []model.TitleName{{Name: "Old Movie", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd2}, []model.TitleName{{Name: "New Movie", Language: "en", IsPrimary: true}})

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
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd}, []model.TitleName{{Name: "With Date", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "No Date", Language: "en", IsPrimary: true}})

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

	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2015, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "2010s Movie", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2022, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "2020s Movie", Language: "en", IsPrimary: true}})

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
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd1}, []model.TitleName{{Name: "Early", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd2}, []model.TitleName{{Name: "Mid", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2025, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd3}, []model.TitleName{{Name: "Late", Language: "en", IsPrimary: true}})

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
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd}, []model.TitleName{{Name: "With Date", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "No Date", Language: "en", IsPrimary: true}})

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
		Runtime:     &runtime,
		TMDBRating:  &tmdbRating,
		Credits:     &credits,
	}

	id := testutil.CreateTitle(t, db, title, []model.TitleName{{Name: "The Space Children", Language: "en", IsPrimary: true}})

	// Insert genres via title_genres table
	_, _ = db.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (?, 'Science Fiction'), (?, 'Drama')`, id, id)

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, &overview, got.Overview)
	assert.Equal(t, []string{"Drama", "Science Fiction"}, got.Genres)
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

	id := testutil.CreateTitle(t, db, title, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	overview := "Updated overview"
	runtime := 120
	tmdbRating := 7.5
	credits := `[{"name":"Director","role":"Director"}]`

	testutil.UpdateTitle(t, db, id, repository.TitleUpdate{
		Overview:   &overview,
		Runtime:    &runtime,
		TMDBRating: &tmdbRating,
		Credits:    &credits,
	})

	// Genres now stored in title_genres
	testutil.ReplaceGenres(t, db, id, []string{"Action"})

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, &overview, got.Overview)
	assert.Equal(t, []string{"Action"}, got.Genres)
	assert.Equal(t, &runtime, got.Runtime)
	assert.Equal(t, &tmdbRating, got.TMDBRating)
	assert.Equal(t, &credits, got.Credits)
}

func TestTitleRepository_List_SortByLastWatched(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	idA := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "A", Language: "en", IsPrimary: true}})
	idB := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "B", Language: "en", IsPrimary: true}})
	idC := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "C", Language: "en", IsPrimary: true}})

	// Set last_watched_at in specific order: C (older), then A (newer), then B (not watched)
	dateC, _ := time.Parse(time.RFC3339, "2024-01-01T10:00:00Z")
	dateA, _ := time.Parse(time.RFC3339, "2024-01-02T10:00:00Z")
	testutil.UpdateTitleLastWatchedAt(t, db, idC, dateC)
	testutil.UpdateTitleLastWatchedAt(t, db, idA, dateA)
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

func TestTitleRepository_GetByID_EpisodesMultiSeason(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2020,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "MultiSeason Show", Language: "en", IsPrimary: true}})

	for sn := 1; sn <= 3; sn++ {
		s := testutil.GetOrCreateSeason(t, db, id, sn)
		for ep := 1; ep <= 5; ep++ {
			_ = testutil.GetOrCreateEpisode(t, db, s.ID, ep)
		}
	}

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	require.Len(t, got.Seasons, 3)
	for i, s := range got.Seasons {
		assert.Len(t, s.Episodes, 5, "season %d should have 5 episodes", i+1)
	}
}

func TestTitleRepository_List_PersonFilter(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	credits := `[{"name":"John Doe","role":"Director"}]`
	idA := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		Credits:     &credits,
	}, []model.TitleName{{Name: "Film A", Language: "en", IsPrimary: true}})

	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2023,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Film B", Language: "en", IsPrimary: true}})

	person := "John Doe"
	result, err := repo.List(repository.TitleFilter{Person: &person})
	require.NoError(t, err)
	require.Len(t, result.Titles, 1)
	assert.Equal(t, idA, result.Titles[0].ID)
}

func TestTitleRepository_Merge_TransfersMissingExternalIDs(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        1989,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Poirot (Simkl)", Language: "en", IsPrimary: true}})

	imdb := "tt0094525"
	var tmdb int64 = 790
	var tvdb int64 = 77623
	var anilist int64 = 9999
	plexKey := "19908"
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:          model.TitleTypeSeries,
		Year:          1989,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		IMDBID:        &imdb,
		TMDBID:        &tmdb,
		TVDBID:        &tvdb,
		AniListID:     &anilist,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "Poirot (Plex)", Language: "en", IsPrimary: true}})

	testutil.MergeTitles(t, db, destID, sourceID, 0)

	got, err := repo.GetByID(destID)
	require.NoError(t, err)
	require.NotNil(t, got.IMDBID)
	assert.Equal(t, imdb, *got.IMDBID)
	require.NotNil(t, got.TMDBID)
	assert.Equal(t, tmdb, *got.TMDBID)
	require.NotNil(t, got.TVDBID)
	assert.Equal(t, tvdb, *got.TVDBID)
	require.NotNil(t, got.AniListID)
	assert.Equal(t, anilist, *got.AniListID)
	require.NotNil(t, got.PlexRatingKey)
	assert.Equal(t, plexKey, *got.PlexRatingKey)
}

func TestTitleRepository_Merge_PreservesExistingExternalIDs(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	destImdb := "tt-dest"
	var destTmdb int64 = 111
	destPlexKey := "dest-key"
	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:          model.TitleTypeSeries,
		Year:          1989,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		IMDBID:        &destImdb,
		TMDBID:        &destTmdb,
		PlexRatingKey: &destPlexKey,
	}, []model.TitleName{{Name: "Dest", Language: "en", IsPrimary: true}})

	srcImdb := "tt-src"
	var srcTmdb int64 = 222
	var srcTvdb int64 = 333
	srcPlexKey := "src-key"
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:          model.TitleTypeSeries,
		Year:          1989,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		IMDBID:        &srcImdb,
		TMDBID:        &srcTmdb,
		TVDBID:        &srcTvdb,
		PlexRatingKey: &srcPlexKey,
	}, []model.TitleName{{Name: "Source", Language: "en", IsPrimary: true}})

	testutil.MergeTitles(t, db, destID, sourceID, 0)

	got, err := repo.GetByID(destID)
	require.NoError(t, err)
	// Non-NULL dest values must be preserved.
	require.NotNil(t, got.IMDBID)
	assert.Equal(t, destImdb, *got.IMDBID)
	require.NotNil(t, got.TMDBID)
	assert.Equal(t, destTmdb, *got.TMDBID)
	require.NotNil(t, got.PlexRatingKey)
	assert.Equal(t, destPlexKey, *got.PlexRatingKey)
	// NULL dest value must be filled from source.
	require.NotNil(t, got.TVDBID)
	assert.Equal(t, srcTvdb, *got.TVDBID)
}

func TestTitleRepository_Merge_StampsAniListOnMovedSeason(t *testing.T) {
	db := setupTestDB(t)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Jujutsu Kaisen", Language: "en", IsPrimary: true}})

	// Dest S1 pre-exists with its own anilist mapping (backfill from migration 020).
	destS1 := testutil.InsertSeason(t, db, destID, 1)
	testutil.InsertSeasonExternalID(t, db, destS1, "anilist", "113415")

	// Source is an S2 sequel carrying AniList ID 145064.
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Jujutsu Kaisen S2", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, sourceID, 1) // source S1 will shift to dest S2

	// seasonOffset = 1 → source S1 becomes dest S2; anilist ID 145064 is stamped on it.
	testutil.MergeTitlesWithAniList(t, db, destID, sourceID, 1, 145064)

	// Dest S1 mapping untouched.
	got, err := testutil.GetSeasonExternalID(t, db, destS1, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "113415", got)

	// Dest S2 now exists and carries 145064.
	var destS2 int64
	require.NoError(t, db.QueryRow(`SELECT id FROM seasons WHERE title_id = ? AND season_number = 2`, destID).Scan(&destS2))
	got2, err := testutil.GetSeasonExternalID(t, db, destS2, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "145064", got2)
}

func TestTitleRepository_Merge_KeepsExistingSeasonAniList(t *testing.T) {
	db := setupTestDB(t)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Jujutsu Kaisen", Language: "en", IsPrimary: true}})

	// Dest already has S2 with a user-confirmed mapping we must not overwrite.
	destS2 := testutil.InsertSeason(t, db, destID, 2)
	testutil.InsertSeasonExternalID(t, db, destS2, "anilist", "111")

	// Source S1 would collide with dest S2 after offset 1.
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Source", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, sourceID, 1)

	testutil.MergeTitlesWithAniList(t, db, destID, sourceID, 1, 999)

	got, err := testutil.GetSeasonExternalID(t, db, destS2, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "111", got, "first writer wins — dest mapping preserved")
}

func TestTitleRepository_Merge_NoAniListSkipsStamp(t *testing.T) {
	db := setupTestDB(t)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dest", Language: "en", IsPrimary: true}})

	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Source", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, sourceID, 1)

	// aniListID = 0 → nothing to stamp.
	testutil.MergeTitlesWithAniList(t, db, destID, sourceID, 0, 0)

	var destS1 int64
	require.NoError(t, db.QueryRow(`SELECT id FROM seasons WHERE title_id = ? AND season_number = 1`, destID).Scan(&destS1))
	got, err := testutil.GetSeasonExternalID(t, db, destS1, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestTitleRepository_Merge_DeletesSource(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dest", Language: "en", IsPrimary: true}})

	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Source", Language: "en", IsPrimary: true}})

	testutil.MergeTitles(t, db, destID, sourceID, 0)

	_, err := repo.GetByID(sourceID)
	assert.Error(t, err)
}

func ptr[T any](v T) *T { return &v }
