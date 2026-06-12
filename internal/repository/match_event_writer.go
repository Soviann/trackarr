package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/model"
)

type MatchEventWriter struct{ tx *sql.Tx }

func NewMatchEventWriter(tx *sql.Tx) *MatchEventWriter { return &MatchEventWriter{tx: tx} }

func (w *MatchEventWriter) Create(ctx context.Context, titleID int64, kind model.MatchEventKind, detail string) error {
	if _, err := w.tx.ExecContext(ctx,
		`INSERT INTO match_events (title_id, kind, detail) VALUES (?, ?, ?)`,
		titleID, kind, detail); err != nil {
		return fmt.Errorf("insert match event: %w", err)
	}
	return nil
}
