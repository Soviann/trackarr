package repository_test

import (
	"context"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
)

func TestGenreRepository_ListWithCounts(t *testing.T) {
	db := setupTestDB(t)
	genreRepo := repository.NewGenreRepository(db)

	t1 := createTestTitle(t, db, "movie", 120)
	t2 := createTestTitle(t, db, "series", 45)
	db.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (?, 'Drama'), (?, 'Thriller')`, t1.ID, t1.ID)
	db.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (?, 'Drama')`, t2.ID)

	genres, err := genreRepo.ListWithCounts(context.Background())
	assert.NoError(t, err)
	assert.Len(t, genres, 2)
	assert.Equal(t, "Drama", genres[0].Genre)
	assert.Equal(t, 2, genres[0].Count)
	assert.Equal(t, "Thriller", genres[1].Genre)
	assert.Equal(t, 1, genres[1].Count)
}

func TestGenreRepository_ReplaceForTitle(t *testing.T) {
	db := setupTestDB(t)
	genreRepo := repository.NewGenreRepository(db)

	t1 := createTestTitle(t, db, "movie", 120)
	db.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (?, 'Drama'), (?, 'Action')`, t1.ID, t1.ID)

	// Replace with a different set
	err := genreRepo.ReplaceForTitle(context.Background(), t1.ID, []string{"Thriller", "Comedy"})
	assert.NoError(t, err)

	genres, err := genreRepo.ListWithCounts(context.Background())
	assert.NoError(t, err)
	assert.Len(t, genres, 2)
	assert.Equal(t, "Comedy", genres[0].Genre)
	assert.Equal(t, "Thriller", genres[1].Genre)
}
