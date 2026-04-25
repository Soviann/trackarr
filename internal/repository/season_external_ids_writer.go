package repository

import (
	"context"
	"database/sql"
	"fmt"
)

// SeasonExternalIDWriter writes (season, provider) → external_id mappings
// inside a caller-owned transaction. Sibling of SeasonExternalIDRepository:
// the repository handles single-statement upserts on the pool, the writer
// participates in larger atomic units (merge consolidation, episode backfill).
type SeasonExternalIDWriter struct {
	tx *sql.Tx
}

func NewSeasonExternalIDWriter(tx *sql.Tx) *SeasonExternalIDWriter {
	return &SeasonExternalIDWriter{tx: tx}
}

// Stamp inserts the mapping only when no row exists for (seasonID, provider).
// First writer wins — a user-confirmed link survives a later automatic stamp
// from the merge or backfill flows.
func (w *SeasonExternalIDWriter) Stamp(ctx context.Context, seasonID int64, provider, externalID string) error {
	if _, err := w.tx.ExecContext(ctx, `
		INSERT INTO season_external_ids (season_id, provider, external_id)
		VALUES (?, ?, ?)
		ON CONFLICT(season_id, provider) DO NOTHING
	`, seasonID, provider, externalID); err != nil {
		return fmt.Errorf("season_external_ids stamp: %w", err)
	}
	return nil
}

// Upsert inserts or replaces the (seasonID, provider) → externalID mapping.
// Last writer wins — use for user-driven fix-match flows where the new value
// must always take effect regardless of an existing row.
func (w *SeasonExternalIDWriter) Upsert(ctx context.Context, seasonID int64, provider, externalID string) error {
	if _, err := w.tx.ExecContext(ctx, `
		INSERT INTO season_external_ids (season_id, provider, external_id)
		VALUES (?, ?, ?)
		ON CONFLICT(season_id, provider) DO UPDATE SET
		    external_id = excluded.external_id,
		    updated_at  = CURRENT_TIMESTAMP
	`, seasonID, provider, externalID); err != nil {
		return fmt.Errorf("season_external_ids upsert: %w", err)
	}
	return nil
}
