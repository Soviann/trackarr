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

// Stamp appends an AniList part for the season. ON CONFLICT on the full
// (season_id, provider, external_id) key dedupes the same entry; a *different*
// incoming id coexists as a new part. This is what makes split-season merges
// keep both parts instead of dropping the second.
func (w *SeasonExternalIDWriter) Stamp(ctx context.Context, seasonID int64, provider, externalID string) error {
	if _, err := w.tx.ExecContext(ctx, `
		INSERT INTO season_external_ids (season_id, provider, external_id)
		VALUES (?, ?, ?)
		ON CONFLICT(season_id, provider, external_id) DO NOTHING`, seasonID, provider, externalID); err != nil {
		return fmt.Errorf("season_external_ids stamp: %w", err)
	}
	return nil
}

// Add is a semantic alias of Stamp for call sites that "add a part" rather than
// "stamp during merge". Same insert-if-absent behaviour.
func (w *SeasonExternalIDWriter) Add(ctx context.Context, seasonID int64, provider, externalID string) error {
	return w.Stamp(ctx, seasonID, provider, externalID)
}

// Delete removes all (seasonID, provider) parts inside the caller's transaction.
func (w *SeasonExternalIDWriter) Delete(ctx context.Context, seasonID int64, provider string) error {
	if _, err := w.tx.ExecContext(ctx,
		`DELETE FROM season_external_ids WHERE season_id = ? AND provider = ?`,
		seasonID, provider,
	); err != nil {
		return fmt.Errorf("season_external_ids delete: %w", err)
	}
	return nil
}

// DeletePart removes a single (seasonID, provider, externalID) part inside the
// caller's transaction.
func (w *SeasonExternalIDWriter) DeletePart(ctx context.Context, seasonID int64, provider, externalID string) error {
	if _, err := w.tx.ExecContext(ctx,
		`DELETE FROM season_external_ids WHERE season_id = ? AND provider = ? AND external_id = ?`,
		seasonID, provider, externalID); err != nil {
		return fmt.Errorf("season_external_ids delete part: %w", err)
	}
	return nil
}

// Reorder sets explicit sort_order values for parts inside a caller-owned transaction.
func (w *SeasonExternalIDWriter) Reorder(ctx context.Context, seasonID int64, provider string, orderedIDs []string) error {
	for i, id := range orderedIDs {
		if _, err := w.tx.ExecContext(ctx, `
			UPDATE season_external_ids SET sort_order = ?, updated_at = CURRENT_TIMESTAMP
			WHERE season_id = ? AND provider = ? AND external_id = ?`, i, seasonID, provider, id); err != nil {
			return fmt.Errorf("season_external_ids reorder: %w", err)
		}
	}
	return nil
}

// UpdatePartMeta updates enrichment fields for a specific part inside the caller's transaction.
func (w *SeasonExternalIDWriter) UpdatePartMeta(ctx context.Context, seasonID int64, provider, externalID string, score, episodeCount *int, startDate *string) error {
	_, err := w.tx.ExecContext(ctx, `
		UPDATE season_external_ids
		SET anilist_average_score = ?, anilist_episode_count = ?, anilist_start_date = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE season_id = ? AND provider = ? AND external_id = ?`,
		score, episodeCount, startDate, seasonID, provider, externalID)
	if err != nil {
		return fmt.Errorf("season_external_ids update meta: %w", err)
	}
	return nil
}

