package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Soviann/trackarr/internal/model"
)

// WatchEventWriter performs write operations on watch events within a
// caller-owned transaction. Accepting only *sql.Tx makes "write to the pool
// without a transaction" a compile-time error — the same class of bug that
// used to surface as SQLite BUSY deadlocks or partially-applied writes.
type WatchEventWriter struct {
	tx *sql.Tx
}

func NewWatchEventWriter(tx *sql.Tx) *WatchEventWriter {
	return &WatchEventWriter{tx: tx}
}

func (w *WatchEventWriter) Create(ctx context.Context, event *model.WatchEvent) (int64, error) {
	if !event.CreatedAt.IsZero() {
		res, err := w.tx.ExecContext(ctx,
			`INSERT INTO watch_events (title_id, episode_id, source, raw_payload, created_at) VALUES (?, ?, ?, ?, ?)`,
			event.TitleID, event.EpisodeID, event.Source, event.RawPayload, event.CreatedAt,
		)
		if err != nil {
			return 0, fmt.Errorf("create watch event: %w", err)
		}
		return res.LastInsertId()
	}

	res, err := w.tx.ExecContext(ctx,
		`INSERT INTO watch_events (title_id, episode_id, source, raw_payload) VALUES (?, ?, ?, ?)`,
		event.TitleID, event.EpisodeID, event.Source, event.RawPayload,
	)
	if err != nil {
		return 0, fmt.Errorf("create watch event: %w", err)
	}
	return res.LastInsertId()
}

// BatchCreate inserts multiple watch events in a single statement.
func (w *WatchEventWriter) BatchCreate(ctx context.Context, events []model.WatchEvent) error {
	if len(events) == 0 {
		return nil
	}

	placeholders := make([]string, len(events))
	args := make([]any, 0, len(events)*4)
	for i, e := range events {
		placeholders[i] = "(?, ?, ?, ?)"
		args = append(args, e.TitleID, e.EpisodeID, e.Source, e.RawPayload)
	}

	query := fmt.Sprintf("INSERT INTO watch_events (title_id, episode_id, source, raw_payload) VALUES %s",
		strings.Join(placeholders, ", "))
	if _, err := w.tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("batch create watch events: %w", err)
	}
	return nil
}
