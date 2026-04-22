package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/database"
)

// GenreCount holds a genre name and how many titles carry it.
type GenreCount struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

// GenreRepository handles genre persistence in the title_genres join table.
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

// ReplaceForTitle deletes all existing genres for a title and inserts the new ones atomically.
func (r *GenreRepository) ReplaceForTitle(ctx context.Context, titleID int64, genres []string) error {
	db, ok := r.db.(*sql.DB)
	if !ok {
		// Already inside a transaction — execute directly.
		if _, err := r.db.Exec(`DELETE FROM title_genres WHERE title_id = ?`, titleID); err != nil {
			return fmt.Errorf("genre: replace: delete: %w", err)
		}
		for _, g := range genres {
			if _, err := r.db.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (?, ?)`, titleID, g); err != nil {
				return fmt.Errorf("genre: replace: insert %q: %w", g, err)
			}
		}
		return nil
	}

	return database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM title_genres WHERE title_id = ?`, titleID); err != nil {
			return fmt.Errorf("genre: replace: delete: %w", err)
		}
		for _, g := range genres {
			if _, err := tx.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (?, ?)`, titleID, g); err != nil {
				return fmt.Errorf("genre: replace: insert %q: %w", g, err)
			}
		}
		return nil
	})
}
