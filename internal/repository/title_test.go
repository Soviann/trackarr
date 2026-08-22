package repository_test

import (
	"context"
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

func TestTitleRepository_CaughtUp(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	// caughtUp: anime series, both aired episodes watched → caught up
	caughtUp := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, IsAnime: true, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "CaughtUp", Language: "en", IsPrimary: true}})
	sCU := testutil.GetOrCreateSeason(t, db, caughtUp, 1)
	testutil.SeedEpisode(t, db, sCU.ID, 1, "2020-01-01", true)
	testutil.SeedEpisode(t, db, sCU.ID, 2, "2020-01-08", true)

	// behind: series with an aired, unwatched episode → not caught up
	behind := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Behind", Language: "en", IsPrimary: true}})
	sB := testutil.GetOrCreateSeason(t, db, behind, 1)
	testutil.SeedEpisode(t, db, sB.ID, 1, "2020-01-01", false)

	// future: all aired watched + one not-yet-aired unwatched → caught up (future ignored)
	future := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Future", Language: "en", IsPrimary: true}})
	sF := testutil.GetOrCreateSeason(t, db, future, 1)
	testutil.SeedEpisode(t, db, sF.ID, 1, "2020-01-01", true)
	testutil.SeedEpisode(t, db, sF.ID, 2, "2099-01-01", false)

	// unknownDate: aired watched + unwatched with empty air_date → caught up (unknown ignored)
	unknownDate := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Unknown", Language: "en", IsPrimary: true}})
	sU := testutil.GetOrCreateSeason(t, db, unknownDate, 1)
	testutil.SeedEpisode(t, db, sU.ID, 1, "2020-01-01", true)
	testutil.SeedEpisode(t, db, sU.ID, 2, "", false)

	// movie: watching movie → never caught up
	_ = testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Movie", Language: "en", IsPrimary: true}})

	want := map[string]bool{"CaughtUp": true, "Behind": false, "Future": true, "Unknown": true, "Movie": false}

	res, err := repo.List(repository.TitleFilter{})
	require.NoError(t, err)
	got := map[string]bool{}
	for _, tt := range res.Titles {
		got[tt.PrimaryName()] = tt.CaughtUp
	}
	for name, exp := range want {
		assert.Equal(t, exp, got[name], "CaughtUp for %s", name)
	}

	// up_to_date filter == titles whose CaughtUp is true
	utd, err := repo.List(repository.TitleFilter{UpToDate: true})
	require.NoError(t, err)
	utdNames := map[string]bool{}
	for _, tt := range utd.Titles {
		utdNames[tt.PrimaryName()] = true
		assert.True(t, tt.CaughtUp, "up_to_date result %s should be CaughtUp", tt.PrimaryName())
	}
	assert.True(t, utdNames["CaughtUp"] && utdNames["Future"] && utdNames["Unknown"])
	assert.False(t, utdNames["Behind"] || utdNames["Movie"])

	// watching_behind is the complement over watching titles
	wb, err := repo.List(repository.TitleFilter{WatchingBehind: true})
	require.NoError(t, err)
	wbNames := map[string]bool{}
	for _, tt := range wb.Titles {
		wbNames[tt.PrimaryName()] = true
	}
	assert.True(t, wbNames["Behind"] && wbNames["Movie"])
}

func TestTitleRepository_HasWatchedAndUnwatchedEpisodes(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Test Series", Language: "en", IsPrimary: true}})

	season := testutil.GetOrCreateSeason(t, db, titleID, 1)

	// No episodes
	hasWatched, err := repo.HasWatchedEpisodes(titleID)
	require.NoError(t, err)
	assert.False(t, hasWatched)
	hasUnwatched, err := repo.HasUnwatchedEpisodes(titleID)
	require.NoError(t, err)
	assert.False(t, hasUnwatched)

	// Add aired unwatched episode
	ep1 := testutil.SeedEpisode(t, db, season.ID, 1, "2020-01-01", false)
	hasWatched, err = repo.HasWatchedEpisodes(titleID)
	require.NoError(t, err)
	assert.False(t, hasWatched)
	hasUnwatched, err = repo.HasUnwatchedEpisodes(titleID)
	require.NoError(t, err)
	assert.True(t, hasUnwatched)

	// Add future unwatched episode
	_ = testutil.SeedEpisode(t, db, season.ID, 2, "2099-01-01", false)

	// Mark ep1 watched
	_, err = db.Exec(`UPDATE episodes SET watched = 1 WHERE id = ?`, ep1.ID)
	require.NoError(t, err)

	hasWatched, err = repo.HasWatchedEpisodes(titleID)
	require.NoError(t, err)
	assert.True(t, hasWatched)
	hasUnwatched, err = repo.HasUnwatchedEpisodes(titleID)
	require.NoError(t, err)
	assert.True(t, hasUnwatched, "future episode is unwatched")
}

func TestTitleRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	// Create test data
	idA := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})
	idB := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusPendingReview}, []model.TitleName{{Name: "Shogun", Language: "en", IsPrimary: true}})
	idC := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, IsAnime: true, Year: 2022, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Naruto", Language: "en", IsPrimary: true}})

	// Create episodes for "up to date" and "watching behind" tests
	// Naruto: 1 season, 2 aired episodes, all watched → "up to date"
	sN := testutil.GetOrCreateSeason(t, db, idC, 1)
	testutil.SeedEpisode(t, db, sN.ID, 1, "2020-01-01", true)
	testutil.SeedEpisode(t, db, sN.ID, 2, "2020-01-08", true)

	// Add a series "watching behind": series with an aired, unwatched episode
	idD := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "The Bear", Language: "en", IsPrimary: true}})
	sD := testutil.GetOrCreateSeason(t, db, idD, 1)
	testutil.SeedEpisode(t, db, sD.ID, 1, "2020-01-01", false) // aired, unwatched

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

func TestTitleRepository_Update_SetsOriginCountry(t *testing.T) {
	db := setupTestDB(t)

	id := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	kr := "KR"
	testutil.UpdateTitle(t, db, id, repository.TitleUpdate{OriginCountry: &kr})

	var got *string
	require.NoError(t, db.QueryRow(`SELECT origin_country FROM titles WHERE id = ?`, id).Scan(&got))
	require.NotNil(t, got)
	assert.Equal(t, "KR", *got)
}

func TestTitleRepository_AddMissingNames(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{
			{Name: "DanMachi", Language: "en", IsPrimary: true},
			{Name: "Dungeon ni Deai", Language: "x-romaji"},
		})

	// Backfill: one new (fr), one already present case-insensitively (en),
	// and a duplicate within the same batch.
	err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).AddMissingNames(context.Background(), id, []model.TitleName{
			{Name: "La légende des Familias", Language: "fr"},
			{Name: "danmachi", Language: "en"},                // dup of primary (different case) → skipped
			{Name: "La légende des Familias", Language: "fr"}, // dup within batch → inserted once
		})
	})
	require.NoError(t, err)

	got, err := repo.GetByID(id)
	require.NoError(t, err)

	// romaji preserved, fr added once, en not duplicated → 3 total.
	assert.Len(t, got.Names, 3)

	var fr []model.TitleName
	var primaries int
	for _, n := range got.Names {
		if n.Language == "fr" {
			fr = append(fr, n)
		}
		if n.IsPrimary {
			primaries++
		}
	}
	require.Len(t, fr, 1)
	assert.Equal(t, "La légende des Familias", fr[0].Name)
	assert.False(t, fr[0].IsPrimary, "backfilled names must not be primary")
	assert.Equal(t, 1, primaries, "the original primary stays the only primary")
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

func TestTitleRepository_Update_ClearsExternalIDsToNull(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	imdb, tmdb, anilist, tvdb := "tt1234567", int64(550), int64(21), int64(81189)
	id := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
		IMDBID: &imdb, TMDBID: &tmdb, AniListID: &anilist, TVDBID: &tvdb,
	}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	// Clear TVDB + IMDB, keep TMDB, reassign AniList — the three states in one Update.
	newAnilist := int64(99)
	testutil.UpdateTitle(t, db, id, repository.TitleUpdate{
		ClearTVDBID: true,
		ClearIMDBID: true,
		AniListID:   &newAnilist,
	})

	got, _ := repo.GetByID(id)
	assert.Nil(t, got.TVDBID, "TVDB cleared to NULL")
	assert.Nil(t, got.IMDBID, "IMDB cleared to NULL")
	require.NotNil(t, got.TMDBID)
	assert.Equal(t, int64(550), *got.TMDBID, "untouched TMDB preserved")
	require.NotNil(t, got.AniListID)
	assert.Equal(t, int64(99), *got.AniListID, "AniList reassigned")
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

func TestTitleRepository_GetByID_HydratesAniListSeasonFields(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)
	seiRepo := repository.NewSeasonExternalIDRepository(db)
	ctx := t.Context()

	titleID := testutil.InsertTitle(t, db, "Jujutsu Kaisen", true)

	// S1 has an AniList mapping AND a community score → both fields populated.
	s1 := testutil.InsertSeason(t, db, titleID, 1)
	testutil.InsertSeasonExternalID(t, db, s1, "anilist", "113415")
	score := 86
	require.NoError(t, seiRepo.UpdatePartMeta(ctx, s1, "anilist", "113415", &score, nil, nil))

	// S2 has no mapping → both fields nil.
	s2 := testutil.InsertSeason(t, db, titleID, 2)
	_ = s2

	got, err := repo.GetByID(titleID)
	require.NoError(t, err)
	require.Len(t, got.Seasons, 2)

	// Season 1 — mapped + scored; primary aliases derived from the single part.
	require.NotNil(t, got.Seasons[0].AniListID)
	assert.Equal(t, "113415", *got.Seasons[0].AniListID)
	require.NotNil(t, got.Seasons[0].AniListAverageScore)
	assert.Equal(t, 86, *got.Seasons[0].AniListAverageScore)

	// Season 2 — unmapped, both fields stay nil
	assert.Nil(t, got.Seasons[1].AniListID)
	assert.Nil(t, got.Seasons[1].AniListAverageScore)
}

func TestTitleRepository_GetByID_HydratesMultiPartAniListSeason(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)
	seiRepo := repository.NewSeasonExternalIDRepository(db)
	ctx := t.Context()

	titleID := testutil.InsertTitle(t, db, "Overlord", true)
	s1 := testutil.InsertSeason(t, db, titleID, 1)

	// Two parts: part B has an earlier start date than part A (to verify ordering).
	testutil.InsertSeasonExternalID(t, db, s1, "anilist", "29722") // part A
	testutil.InsertSeasonExternalID(t, db, s1, "anilist", "20000") // part B — earlier date

	scoreA, epA := 85, 13
	dateA := "2015-07-07"
	require.NoError(t, seiRepo.UpdatePartMeta(ctx, s1, "anilist", "29722", &scoreA, &epA, &dateA))

	scoreB, epB := 80, 13
	dateB := "2014-01-07"
	require.NoError(t, seiRepo.UpdatePartMeta(ctx, s1, "anilist", "20000", &scoreB, &epB, &dateB))

	got, err := repo.GetByID(titleID)
	require.NoError(t, err)
	require.Len(t, got.Seasons, 1)

	parts := got.Seasons[0].AniListParts
	require.Len(t, parts, 2, "season must expose two AniList parts")

	// Ordered by start_date asc: part B (2014) before part A (2015).
	assert.Equal(t, "20000", parts[0].ExternalID)
	assert.Equal(t, "29722", parts[1].ExternalID)

	// Primary aliases derived from first part.
	require.NotNil(t, got.Seasons[0].AniListID)
	assert.Equal(t, "20000", *got.Seasons[0].AniListID)
	require.NotNil(t, got.Seasons[0].AniListAverageScore)
	assert.Equal(t, scoreB, *got.Seasons[0].AniListAverageScore)
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

func TestTitleRepository_Merge_DoesNotCopySourceNames(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2009,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Show", Language: "en", IsPrimary: true}})

	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2010,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Show Season 2", Language: "en", IsPrimary: true}})

	testutil.MergeTitles(t, db, destID, sourceID, 1)

	got, err := repo.GetByID(destID)
	require.NoError(t, err)
	for _, n := range got.Names {
		assert.NotEqual(t, "Show Season 2", n.Name, "source season name must not be copied onto the merged title")
	}
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

func TestTitleRepository_Merge_AppendsAniListPartOnCollision(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Dest has S1 already mapped to AniList part "100".
	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dest", Language: "en", IsPrimary: true}})
	destS1 := testutil.InsertSeason(t, db, destID, 1)
	testutil.InsertSeasonExternalID(t, db, destS1, "anilist", "100")

	// Source carries a single S1 with AniList "200"; offset 0 → collides onto dest S1.
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Source", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, sourceID, 1)

	testutil.MergeTitlesWithAniList(t, db, destID, sourceID, 0, 200)

	parts, err := repository.NewSeasonExternalIDRepository(db).ListParts(ctx, destS1, repository.ProviderAniList)
	require.NoError(t, err)
	require.Len(t, parts, 2, "both AniList parts must be present after merge")
	ids := []string{parts[0].ExternalID, parts[1].ExternalID}
	assert.ElementsMatch(t, []string{"100", "200"}, ids)
}

func TestTitleRepository_Merge_AnimeEpisodeOffsetOnCollision(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Dest is an anime series with S1 having 13 episodes (1..13).
	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dest Anime", Language: "en", IsPrimary: true}})
	destS1 := testutil.InsertSeason(t, db, destID, 1)
	for ep := 1; ep <= 13; ep++ {
		_, err := db.ExecContext(ctx, `INSERT INTO episodes (season_id, episode) VALUES (?, ?)`, destS1, ep)
		require.NoError(t, err)
	}

	// Source is an anime series with S1 having 13 episodes (1..13).
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Source Anime", Language: "en", IsPrimary: true}})
	sourceS1 := testutil.InsertSeason(t, db, sourceID, 1)
	for ep := 1; ep <= 13; ep++ {
		_, err := db.ExecContext(ctx, `INSERT INTO episodes (season_id, episode) VALUES (?, ?)`, sourceS1, ep)
		require.NoError(t, err)
	}

	// Merge source into dest at offset 0 (colliding on S1).
	testutil.MergeTitlesWithAniList(t, db, destID, sourceID, 0, 159322)

	// Dest S1 should now have 26 episodes (1..26).
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM episodes WHERE season_id = ?`, destS1).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 26, count, "episodes from source should be offset and appended to dest season")

	var maxEp int
	err = db.QueryRowContext(ctx, `SELECT MAX(episode) FROM episodes WHERE season_id = ?`, destS1).Scan(&maxEp)
	require.NoError(t, err)
	assert.Equal(t, 26, maxEp)
}

func TestTitleRepository_Merge_DeduplicatesAniListPartOnCollision(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Dest has S1 already mapped to AniList part "100".
	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dest", Language: "en", IsPrimary: true}})
	destS1 := testutil.InsertSeason(t, db, destID, 1)
	testutil.InsertSeasonExternalID(t, db, destS1, "anilist", "100")

	// Source also carries "100" → merging must deduplicate.
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Source", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, sourceID, 1)

	testutil.MergeTitlesWithAniList(t, db, destID, sourceID, 0, 100)

	parts, err := repository.NewSeasonExternalIDRepository(db).ListParts(ctx, destS1, repository.ProviderAniList)
	require.NoError(t, err)
	assert.Len(t, parts, 1, "duplicate AniList id must not create a second part")
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

func TestTitleRepository_Merge_SourceHasNoSeasons_CreatesTargetSeason(t *testing.T) {
	db := setupTestDB(t)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dest", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, destID, 1)

	// sourceID has 0 season rows (e.g. an AniList-only title created without TMDB/TVDB episodes).
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Source (AniList Only)", Language: "en", IsPrimary: true}})

	// Merge with seasonOffset = 3 (target season 4) and sourceAniListID = 171110.
	testutil.MergeTitlesWithAniList(t, db, destID, sourceID, 3, 171110)

	// Season 4 should be synthesized on destID and stamped with 171110.
	var destS4 int64
	require.NoError(t, db.QueryRow(`SELECT id FROM seasons WHERE title_id = ? AND season_number = 4`, destID).Scan(&destS4))
	got, err := testutil.GetSeasonExternalID(t, db, destS4, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "171110", got)
}

// Reported bug: merging a dropped S2 (source, becomes the newest season) into a
// completed S1 (dest) must make the series dropped, not stay completed.
func TestTitleRepository_Merge_NewestDroppedWins(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Granblue Fantasy", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, destID, 1)

	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusDropped,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Granblue Fantasy S2", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, sourceID, 1) // shifts to dest S2 under offset 1

	testutil.MergeTitles(t, db, destID, sourceID, 1)

	got, err := repo.GetByID(destID)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusDropped, got.Status)
}

// Prequel merge: the source becomes the OLDER season (negative offset), so the
// dest is the newest block. Its watching status must win over the source's
// completed — proving "newest" is resolved by season number, not by source/dest.
func TestTitleRepository_Merge_NewestResolvedBySeasonNumber(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dest S2", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, destID, 2)

	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Source S2", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, sourceID, 2) // offset -1 → becomes S1 (older)

	testutil.MergeTitles(t, db, destID, sourceID, -1)

	got, err := repo.GetByID(destID)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusWatching, got.Status)
}

func TestTitleRepository_SimklIDRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	simklID := int64(123456)
	simklSlug := "breaking-bad-2008"

	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2008,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		SimklID:     &simklID,
		SimklSlug:   &simklSlug,
	}, []model.TitleName{{Name: "Breaking Bad", Language: "en", IsPrimary: true}})

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	require.NotNil(t, got.SimklID, "SimklID should round-trip through Create/GetByID")
	assert.Equal(t, simklID, *got.SimklID)
	require.NotNil(t, got.SimklSlug, "SimklSlug should round-trip through Create/GetByID")
	assert.Equal(t, simklSlug, *got.SimklSlug)

	// Update via TitleUpdate — both fields writable
	newID := int64(999)
	newSlug := "breaking-bad-updated"
	testutil.UpdateTitle(t, db, id, repository.TitleUpdate{
		SimklID:   &newID,
		SimklSlug: &newSlug,
	})

	got2, err := repo.GetByID(id)
	require.NoError(t, err)
	require.NotNil(t, got2.SimklID)
	assert.Equal(t, newID, *got2.SimklID)
	require.NotNil(t, got2.SimklSlug)
	assert.Equal(t, newSlug, *got2.SimklSlug)
}

func TestTitleRepository_SimklID_NilWhenAbsent(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2011,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		// SimklID and SimklSlug deliberately omitted
	}, []model.TitleName{{Name: "Game of Thrones", Language: "en", IsPrimary: true}})

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Nil(t, got.SimklID, "SimklID should be nil when not set")
	assert.Nil(t, got.SimklSlug, "SimklSlug should be nil when not set")
}

func ptr[T any](v T) *T { return &v }

func TestTitleRepo_WatchProvidersRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2016,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Test Series", Language: "en", IsPrimary: true}})

	providers := `[{"id":119,"name":"Amazon Prime Video"},{"id":8,"name":"Netflix"}]`
	err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(context.Background(), id, repository.TitleUpdate{WatchProviders: &providers})
	})
	require.NoError(t, err)

	got, err := repository.NewTitleRepository(db).GetByID(id)
	require.NoError(t, err)
	require.Len(t, got.WatchProviders, 2)
	assert.Equal(t, int64(119), got.WatchProviders[0].ID)
	assert.Equal(t, "Amazon Prime Video", got.WatchProviders[0].Name)
}

func TestTitleRepo_ContinueWatching_IncludesProviders(t *testing.T) {
	db := setupTestDB(t)
	sonarrID := int64(42)
	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2016,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		SonarrID:    &sonarrID,
	}, []model.TitleName{{Name: "Test Series", Language: "en", IsPrimary: true}})

	// One season, one unwatched episode so the title qualifies for continue-watching.
	s := testutil.GetOrCreateSeason(t, db, id, 1)
	testutil.SeedEpisode(t, db, s.ID, 1, "2020-01-01", false)

	providers := `[{"id":119,"name":"Amazon Prime Video"}]`
	err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(context.Background(), id, repository.TitleUpdate{WatchProviders: &providers})
	})
	require.NoError(t, err)

	items, err := repository.NewTitleRepository(db).ListContinueWatching()
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].WatchProviders, 1)
	assert.Equal(t, int64(119), items[0].WatchProviders[0].ID)
	assert.True(t, items[0].IsAnime)
	require.NotNil(t, items[0].SonarrID)
	assert.Equal(t, int64(42), *items[0].SonarrID)
}

func TestTitleRepo_Upcoming_IncludesProviders(t *testing.T) {
	db := setupTestDB(t)
	nextAirDate := "2099-01-01"
	sonarrID := int64(99)
	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		NextAirDate: &nextAirDate,
		SonarrID:    &sonarrID,
	}, []model.TitleName{{Name: "Upcoming Series", Language: "en", IsPrimary: true}})

	providers := `[{"id":8,"name":"Netflix"}]`
	err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(context.Background(), id, repository.TitleUpdate{WatchProviders: &providers})
	})
	require.NoError(t, err)

	items, err := repository.NewTitleRepository(db).ListUpcoming("2025-01-01")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].WatchProviders, 1)
	assert.Equal(t, int64(8), items[0].WatchProviders[0].ID)
	assert.Equal(t, "Netflix", items[0].WatchProviders[0].Name)
	assert.True(t, items[0].IsAnime)
	require.NotNil(t, items[0].SonarrID)
	assert.Equal(t, int64(99), *items[0].SonarrID)
}

func TestTitleRepository_List_FilterByOriginCountry(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	kr, jp, us := "KR", "JP", "US"

	idKR := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "K Drama", Language: "en", IsPrimary: true}})
	idJP := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "JP Film", Language: "en", IsPrimary: true}})
	idUS := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "US Show", Language: "en", IsPrimary: true}})

	testutil.UpdateTitle(t, db, idKR, repository.TitleUpdate{OriginCountry: &kr})
	testutil.UpdateTitle(t, db, idJP, repository.TitleUpdate{OriginCountry: &jp})
	testutil.UpdateTitle(t, db, idUS, repository.TitleUpdate{OriginCountry: &us})

	result, err := repo.List(repository.TitleFilter{OriginCountries: []string{"KR", "JP"}})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	names := []string{result.Titles[0].PrimaryName(), result.Titles[1].PrimaryName()}
	assert.ElementsMatch(t, []string{"K Drama", "JP Film"}, names)
}

func TestTitleRepository_List_FilterByRatingMinimums(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	great, mid := 9, 6
	tmdbRating := 8.5
	unratedTMDB := 9.0
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, MyRating: &great, TMDBRating: &tmdbRating}, []model.TitleName{{Name: "Great", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, MyRating: &mid, TMDBRating: &tmdbRating}, []model.TitleName{{Name: "Mid", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, TMDBRating: &unratedTMDB}, []model.TitleName{{Name: "Unrated", Language: "en", IsPrimary: true}})

	min := 8
	result, err := repo.List(repository.TitleFilter{MyRatingMin: &min})
	require.NoError(t, err)
	require.Len(t, result.Titles, 1) // only "Great"; NULL my_rating excluded
	assert.Equal(t, "Great", result.Titles[0].PrimaryName())
}

func TestTitleRepository_ArrQueue_RequiresValidIDs(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	tvdbValid := int64(12345)
	tmdbValid := int64(67890)
	zeroID := int64(0)

	// Series with valid TVDB ID -> included
	_ = testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
		TVDBID:      &tvdbValid,
	}, []model.TitleName{{Name: "Valid Series", Language: "en", IsPrimary: true}})

	// Series with no TVDB ID (nil) -> excluded
	_ = testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "No TVDB Series", Language: "en", IsPrimary: true}})

	// Series with 0 TVDB ID -> excluded
	_ = testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
		TVDBID:      &zeroID,
	}, []model.TitleName{{Name: "Zero TVDB Series", Language: "en", IsPrimary: true}})

	// Movie with valid TMDB ID -> included
	_ = testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
		TMDBID:      &tmdbValid,
	}, []model.TitleName{{Name: "Valid Movie", Language: "en", IsPrimary: true}})

	// Movie with no TMDB ID -> excluded
	_ = testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "No TMDB Movie", Language: "en", IsPrimary: true}})

	items, err := repo.ListArrQueue()
	require.NoError(t, err)
	require.Len(t, items, 2)

	names := []string{items[0].Name, items[1].Name}
	assert.Contains(t, names, "Valid Series")
	assert.Contains(t, names, "Valid Movie")
}
