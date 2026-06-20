package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// ProviderAniList is the canonical provider key for AniList media IDs in
// season_external_ids.provider. Use this constant from any new SQL or repo
// call so a future provider rename touches one place.
const ProviderAniList = "anilist"

// partOrderClause is the single source of truth for part ordering: explicit
// sort_order first (NULLs last), then AniList start date ascending (NULLs
// last), then external_id as tiebreak.
const partOrderClause = `ORDER BY (sort_order IS NULL), sort_order, (anilist_start_date IS NULL), anilist_start_date, external_id`

// SeasonExternalIDRepository maps a season to provider-specific IDs (currently
// AniList). The repository serves single-statement upserts on the connection
// pool (handler-driven fix-match flows). For writes that must commit alongside
// other rows in the same transaction, use SeasonExternalIDWriter instead.
type SeasonExternalIDRepository struct {
	db *sql.DB
}

func NewSeasonExternalIDRepository(db *sql.DB) *SeasonExternalIDRepository {
	return &SeasonExternalIDRepository{db: db}
}

// Get returns the primary (first-sorted) external_id for (seasonID, provider),
// or "" if none.
func (r *SeasonExternalIDRepository) Get(ctx context.Context, seasonID int64, provider string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		SELECT external_id FROM season_external_ids
		WHERE season_id = ? AND provider = ? `+partOrderClause+` LIMIT 1`, seasonID, provider).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("season_external_ids get: %w", err)
	}
	return id, nil
}

// Add inserts a (season, provider, externalID) mapping. Silently deduplicates
// if the same triple already exists; a different externalID for the same
// (season, provider) is added as a new part (split-cour support).
func (r *SeasonExternalIDRepository) Add(ctx context.Context, seasonID int64, provider, externalID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO season_external_ids (season_id, provider, external_id)
		VALUES (?, ?, ?)
		ON CONFLICT(season_id, provider, external_id) DO NOTHING`, seasonID, provider, externalID)
	if err != nil {
		return fmt.Errorf("season_external_ids add: %w", err)
	}
	return nil
}

// DeletePart removes a single (season, provider, externalID) part.
func (r *SeasonExternalIDRepository) DeletePart(ctx context.Context, seasonID int64, provider, externalID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM season_external_ids WHERE season_id = ? AND provider = ? AND external_id = ?`,
		seasonID, provider, externalID)
	if err != nil {
		return fmt.Errorf("season_external_ids delete part: %w", err)
	}
	return nil
}

// Delete removes all parts for (seasonID, provider).
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

// ListParts returns all parts for (seasonID, provider) in display order.
func (r *SeasonExternalIDRepository) ListParts(ctx context.Context, seasonID int64, provider string) ([]model.AniListPart, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT external_id, anilist_average_score, anilist_episode_count, anilist_start_date, sort_order
		FROM season_external_ids
		WHERE season_id = ? AND provider = ? `+partOrderClause, seasonID, provider)
	if err != nil {
		return nil, fmt.Errorf("season_external_ids list parts: %w", err)
	}
	defer rows.Close()
	var out []model.AniListPart
	for rows.Next() {
		var p model.AniListPart
		if err := rows.Scan(&p.ExternalID, &p.Score, &p.EpisodeCount, &p.StartDate, &p.SortOrder); err != nil {
			return nil, fmt.Errorf("season_external_ids scan part: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListPartsForTitle returns (season_id → []AniListPart) for a title + provider.
func (r *SeasonExternalIDRepository) ListPartsForTitle(ctx context.Context, titleID int64, provider string) (map[int64][]model.AniListPart, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sei.season_id, sei.external_id, sei.anilist_average_score,
		       sei.anilist_episode_count, sei.anilist_start_date, sei.sort_order
		FROM season_external_ids sei
		JOIN seasons s ON s.id = sei.season_id
		WHERE s.title_id = ? AND sei.provider = ?
		`+partOrderClause, titleID, provider)
	if err != nil {
		return nil, fmt.Errorf("season_external_ids list parts for title: %w", err)
	}
	defer rows.Close()
	out := make(map[int64][]model.AniListPart)
	for rows.Next() {
		var sid int64
		var p model.AniListPart
		if err := rows.Scan(&sid, &p.ExternalID, &p.Score, &p.EpisodeCount, &p.StartDate, &p.SortOrder); err != nil {
			return nil, fmt.Errorf("season_external_ids scan part row: %w", err)
		}
		out[sid] = append(out[sid], p)
	}
	return out, rows.Err()
}

// Reorder sets explicit sort_order values so that orderedIDs[0] sorts first.
func (r *SeasonExternalIDRepository) Reorder(ctx context.Context, seasonID int64, provider string, orderedIDs []string) error {
	for i, id := range orderedIDs {
		if _, err := r.db.ExecContext(ctx, `
			UPDATE season_external_ids SET sort_order = ?, updated_at = CURRENT_TIMESTAMP
			WHERE season_id = ? AND provider = ? AND external_id = ?`, i, seasonID, provider, id); err != nil {
			return fmt.Errorf("season_external_ids reorder: %w", err)
		}
	}
	return nil
}

// UpdatePartMeta updates enrichment fields for a specific part.
func (r *SeasonExternalIDRepository) UpdatePartMeta(ctx context.Context, seasonID int64, provider, externalID string, score, episodeCount *int, startDate *string) error {
	_, err := r.db.ExecContext(ctx, `
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

// ListForTitle returns (season_id → primary external_id) for a title + provider.
// Background service still uses this single-value form until a later task
// migrates it to ListPartsForTitle.
func (r *SeasonExternalIDRepository) ListForTitle(ctx context.Context, titleID int64, provider string) (map[int64]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sei.season_id, sei.external_id
		FROM season_external_ids sei
		JOIN seasons s ON s.id = sei.season_id
		WHERE s.title_id = ? AND sei.provider = ?
		`+partOrderClause, titleID, provider)
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
		// Keep only the first (primary) part per season — later rows are
		// additional split-cour parts that background.go ignores for now.
		if _, exists := out[sid]; !exists {
			out[sid] = eid
		}
	}
	return out, rows.Err()
}
