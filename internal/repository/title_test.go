package repository_test

import (
	"database/sql"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(":memory:")
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
	idC, _ := repo.Create(&model.Title{Type: model.TitleTypeAnime, Year: 2022, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Naruto", Language: "en", IsPrimary: true}})

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
			name:   "filter by status watching",
			filter: repository.TitleFilter{Status: ptr(model.TitleStatusWatching)},
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
			name:   "pagination limit",
			filter: repository.TitleFilter{Limit: 2},
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
			name:     "watching behind filter",
			filter:   repository.TitleFilter{WatchingBehind: true},
			wantLen:  2, // Dune (movie) + The Bear (has unwatched)
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
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed, IMDBID: &imdb, TMDBID: &tmdb}, []model.TitleName{{Name: "Test Movie", Language: "en", IsPrimary: true}})

	tests := []struct {
		name      string
		imdbID    *string
		tmdbID    *int64
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
			got, err := repo.FindByExternalID(tc.imdbID, tc.tmdbID, tc.plexKey)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantTitle, got.PrimaryName())
		})
	}
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

func ptr[T any](v T) *T { return &v }
