package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Soviann/trackarr/internal/model"
)

// WrappedWriter handles all mutations for wrapped_snapshots inside a caller-owned transaction.
type WrappedWriter struct {
	tx *sql.Tx
}

func NewWrappedWriter(tx *sql.Tx) *WrappedWriter {
	return &WrappedWriter{tx: tx}
}

// SaveSnapshot saves or replaces an immutable Wrapped snapshot.
func (w *WrappedWriter) SaveSnapshot(ctx context.Context, year int, resp *model.WrappedResponse) error {
	dataJSON, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal wrapped snapshot %d: %w", year, err)
	}

	query := `
		INSERT INTO wrapped_snapshots (year, data_json, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(year) DO UPDATE SET
			data_json = excluded.data_json,
			created_at = excluded.created_at
	`
	_, err = w.tx.ExecContext(ctx, query, year, string(dataJSON))
	if err != nil {
		return fmt.Errorf("save wrapped snapshot %d: %w", year, err)
	}
	return nil
}

// DeleteSnapshot removes a snapshot for a given year.
func (w *WrappedWriter) DeleteSnapshot(ctx context.Context, year int) error {
	query := `DELETE FROM wrapped_snapshots WHERE year = ?`
	_, err := w.tx.ExecContext(ctx, query, year)
	if err != nil {
		return fmt.Errorf("delete wrapped snapshot %d: %w", year, err)
	}
	return nil
}
