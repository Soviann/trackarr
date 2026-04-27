package repository_test

import (
	"context"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTitleSearch_GenreFilterOR(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	t1 := createTestTitle(t, db, "movie", 120)
	t2 := createTestTitle(t, db, "series", 45)
	t3 := createTestTitle(t, db, "movie", 90)
	_, err := db.Exec(`INSERT INTO title_genres VALUES (?, 'Drama')`, t1.ID)
	assert.NoError(t, err)
	_, err = db.Exec(`INSERT INTO title_genres VALUES (?, 'Action')`, t2.ID)
	assert.NoError(t, err)
	_, err = db.Exec(`INSERT INTO title_genres VALUES (?, 'Thriller')`, t3.ID)
	assert.NoError(t, err)

	result, err := repo.List(repository.TitleFilter{
		Genres:  []string{"Drama", "Action"},
		GenreOp: "OR",
	})
	assert.NoError(t, err)
	assert.Len(t, result.Titles, 2) // t1 and t2
}

func TestTitleSearch_GenreFilterAND(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	t1 := createTestTitle(t, db, "movie", 120)
	t2 := createTestTitle(t, db, "series", 45)
	_, err := db.Exec(`INSERT INTO title_genres VALUES (?, 'Drama'), (?, 'Action')`, t1.ID, t1.ID)
	assert.NoError(t, err)
	_, err = db.Exec(`INSERT INTO title_genres VALUES (?, 'Drama')`, t2.ID)
	assert.NoError(t, err)

	result, err := repo.List(repository.TitleFilter{
		Genres:  []string{"Drama", "Action"},
		GenreOp: "AND",
	})
	assert.NoError(t, err)
	assert.Len(t, result.Titles, 1) // only t1 has both Drama AND Action
	assert.Equal(t, t1.ID, result.Titles[0].ID)
}

func TestGenreRepository_ListWithCounts_Context(t *testing.T) {
	db := setupTestDB(t)
	genreRepo := repository.NewGenreRepository(db)

	t1 := createTestTitle(t, db, "movie", 120)
	_, err := db.Exec(`INSERT INTO title_genres VALUES (?, 'Drama')`, t1.ID)
	assert.NoError(t, err)

	genres, err := genreRepo.ListWithCounts(context.Background())
	assert.NoError(t, err)
	assert.Len(t, genres, 1)
	assert.Equal(t, "Drama", genres[0].Genre)
}

// TestTitleSearch_FTSExactHitRanksFirst proves the FTS-driven primary path:
// a title whose name matches exactly must come back, with the exact-match
// relevance bucket pushing it ahead of any fuzzy-only contenders.
func TestTitleSearch_FTSExactHitRanksFirst(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	bbID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2008,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Breaking Bad", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2014,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Better Call Saul", Language: "en", IsPrimary: true}})

	q := "breaking bad"
	res, err := repo.List(repository.TitleFilter{Search: &q})
	require.NoError(t, err)
	require.NotEmpty(t, res.Titles)
	assert.Equal(t, bbID, res.Titles[0].ID, "exact-name FTS hit must rank first")
}

// TestTitleSearch_FuzzyFallbackOnTypo proves the Levenshtein fallback fires
// when FTS returns nothing — a typo of one character must still surface the
// title rather than vanishing into a no-results screen.
func TestTitleSearch_FuzzyFallbackOnTypo(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	bbID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2008,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Breaking Bad", Language: "en", IsPrimary: true}})

	// "brakin" is a typo of "breakin(g)" — Levenshtein distance ≤ 2 from
	// the word "Breaking" so the fuzzy branch must catch it.
	q := "brakin"
	res, err := repo.List(repository.TitleFilter{Search: &q})
	require.NoError(t, err)
	require.NotEmpty(t, res.Titles, "fuzzy fallback must surface a 1-char typo")
	assert.Equal(t, bbID, res.Titles[0].ID)
}

// TestTitleSearch_NoResult is the negative guard — a search term unlike any
// title's name must come back empty, not throw and not surface unrelated rows.
func TestTitleSearch_NoResult(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2008,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Breaking Bad", Language: "en", IsPrimary: true}})

	q := "zzzunmatchableqxv"
	res, err := repo.List(repository.TitleFilter{Search: &q})
	require.NoError(t, err)
	assert.Empty(t, res.Titles)
	assert.Equal(t, 0, res.Total)
}
