package repository_test

import (
	"context"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
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
