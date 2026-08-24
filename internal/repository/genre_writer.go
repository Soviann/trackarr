package repository

import (
	"context"
	"database/sql"
	"fmt"
)

// GenreWriter performs write operations on title_genres within a caller-owned
// transaction. Accepting only *sql.Tx makes "write to the pool without a
// transaction" a compile-time error — the same class of bug that used to
// surface as SQLite BUSY deadlocks or partially-applied multi-statement writes.
type GenreWriter struct {
	tx *sql.Tx
}

func NewGenreWriter(tx *sql.Tx) *GenreWriter {
	return &GenreWriter{tx: tx}
}

// ReplaceForTitle deletes all existing genres for a title and inserts the new
// ones in the caller's transaction. Delete + inserts share one tx so readers
// never observe an empty set between the two halves.
func (w *GenreWriter) ReplaceForTitle(ctx context.Context, titleID int64, genres []string) error {
	if _, err := w.tx.ExecContext(ctx, `DELETE FROM title_genres WHERE title_id = ?`, titleID); err != nil {
		return fmt.Errorf("genre: replace: delete: %w", err)
	}
	for _, g := range genres {
		if _, err := w.tx.ExecContext(ctx, `INSERT INTO title_genres (title_id, genre) VALUES (?, ?)`, titleID, g); err != nil {
			return fmt.Errorf("genre: replace: insert %q: %w", g, err)
		}
	}
	return nil
}
