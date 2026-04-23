package repository

import (
	"context"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/database"
)

// GenreCount holds a genre name and how many titles carry it.
type GenreCount struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

// GenreRepository handles read access to the title_genres join table. Writes
// live on GenreWriter, which requires a *sql.Tx.
type GenreRepository struct {
	db database.DBTX
}

func NewGenreRepository(db database.DBTX) *GenreRepository {
	return &GenreRepository{db: db}
}

// ListWithCounts returns all genres in the library ordered by count descending, then name ascending.
func (r *GenreRepository) ListWithCounts(_ context.Context) ([]GenreCount, error) {
	rows, err := r.db.Query(`
		SELECT genre, COUNT(*) AS count
		FROM title_genres
		GROUP BY genre
		ORDER BY count DESC, genre ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("genre: list with counts: %w", err)
	}
	defer rows.Close()

	var results []GenreCount
	for rows.Next() {
		var g GenreCount
		if err := rows.Scan(&g.Genre, &g.Count); err != nil {
			return nil, fmt.Errorf("genre: scan: %w", err)
		}
		results = append(results, g)
	}
	return results, rows.Err()
}
