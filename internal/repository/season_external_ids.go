package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SeasonExternalIDRepository maps a season to provider-specific IDs (currently
// AniList). Unlike SeasonWriter, writes here are single-statement upserts so
// they run outside a caller-owned tx — batching multiple (season, provider)
// pairs is not a use case we have.
type SeasonExternalIDRepository struct {
	db *sql.DB
}

func NewSeasonExternalIDRepository(db *sql.DB) *SeasonExternalIDRepository {
	return &SeasonExternalIDRepository{db: db}
}

// Get returns the external_id for (seasonID, provider), or "" if none.
func (r *SeasonExternalIDRepository) Get(ctx context.Context, seasonID int64, provider string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx,
		`SELECT external_id FROM season_external_ids WHERE season_id = ? AND provider = ?`,
		seasonID, provider,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("season_external_ids get: %w", err)
	}
	return id, nil
}

// Set upserts the (season, provider) → external_id mapping.
func (r *SeasonExternalIDRepository) Set(ctx context.Context, seasonID int64, provider, externalID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO season_external_ids (season_id, provider, external_id)
		VALUES (?, ?, ?)
		ON CONFLICT(season_id, provider) DO UPDATE SET
		    external_id = excluded.external_id,
		    updated_at  = CURRENT_TIMESTAMP
	`, seasonID, provider, externalID)
	if err != nil {
		return fmt.Errorf("season_external_ids set: %w", err)
	}
	return nil
}

// Delete removes the mapping; used by the upcoming per-season fix-match flow
// (Phase 7) to let the user clear an incorrect AniList link.
func (r *SeasonExternalIDRepository) Delete(ctx context.Context, seasonID int64, provider string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM season_external_ids WHERE season_id = ? AND provider = ?`,
		seasonID, provider,
	)
	if err != nil {
		return fmt.Errorf("season_external_ids delete: %w", err)
	}
	return nil
}

// ListForTitle returns (season_id → external_id) for a title + provider.
func (r *SeasonExternalIDRepository) ListForTitle(ctx context.Context, titleID int64, provider string) (map[int64]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sei.season_id, sei.external_id
		FROM season_external_ids sei
		JOIN seasons s ON s.id = sei.season_id
		WHERE s.title_id = ? AND sei.provider = ?
	`, titleID, provider)
	if err != nil {
		return nil, fmt.Errorf("season_external_ids list: %w", err)
	}
	defer rows.Close()

	out := make(map[int64]string)
	for rows.Next() {
		var sid int64
		var eid string
		if err := rows.Scan(&sid, &eid); err != nil {
			return nil, fmt.Errorf("season_external_ids scan: %w", err)
		}
		out[sid] = eid
	}
	return out, rows.Err()
}
