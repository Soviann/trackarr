package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Soviann/trackarr/internal/model"
)

type TitleRelationWriter struct {
	tx *sql.Tx
}

func NewTitleRelationWriter(tx *sql.Tx) *TitleRelationWriter {
	return &TitleRelationWriter{tx: tx}
}

// UpsertBatch inserts or updates relations for a given title.
// It removes any existing relations for that title that are not in the new batch.
func (w *TitleRelationWriter) UpsertBatch(ctx context.Context, titleID int64, relations []model.TitleRelation) error {
	if len(relations) == 0 {
		return w.DeleteForTitle(ctx, titleID)
	}

	// Keep track of external IDs in the new batch to clean up removed relations
	keptIDs := make([]any, 0, len(relations)+1)
	keptIDs = append(keptIDs, titleID)

	placeholders := make([]string, 0, len(relations))

	for i, rel := range relations {
		provider := rel.Provider
		if provider == "" {
			provider = "anilist"
		}
		sortOrder := rel.SortOrder
		if sortOrder == 0 {
			sortOrder = i
		}

		_, err := w.tx.ExecContext(ctx, `
			INSERT INTO title_relations (
				title_id, season_id, provider, external_id,
				relation_type, format, title, romaji_title,
				cover_url, year, score, episode_count, duration,
				overview, sort_order, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(title_id, provider, external_id) DO UPDATE SET
				season_id = excluded.season_id,
				relation_type = excluded.relation_type,
				format = excluded.format,
				title = excluded.title,
				romaji_title = excluded.romaji_title,
				cover_url = excluded.cover_url,
				year = excluded.year,
				score = excluded.score,
				episode_count = excluded.episode_count,
				duration = excluded.duration,
				overview = excluded.overview,
				sort_order = excluded.sort_order,
				updated_at = CURRENT_TIMESTAMP
		`,
			titleID, rel.SeasonID, provider, rel.ExternalID,
			string(rel.RelationType), rel.Format, rel.Title, rel.RomajiTitle,
			rel.CoverURL, rel.Year, rel.Score, rel.EpisodeCount, rel.Duration,
			rel.Overview, sortOrder,
		)
		if err != nil {
			return fmt.Errorf("title_relations upsert: %w", err)
		}

		keptIDs = append(keptIDs, rel.ExternalID)
		placeholders = append(placeholders, "?")
	}

	// Delete relations no longer present
	deleteQuery := fmt.Sprintf(
		`DELETE FROM title_relations WHERE title_id = ? AND external_id NOT IN (%s)`,
		joinStrings(placeholders, ","),
	)
	if _, err := w.tx.ExecContext(ctx, deleteQuery, keptIDs...); err != nil {
		return fmt.Errorf("title_relations delete stale: %w", err)
	}

	return nil
}

// DeleteForTitle removes all relations for a title.
func (w *TitleRelationWriter) DeleteForTitle(ctx context.Context, titleID int64) error {
	if _, err := w.tx.ExecContext(ctx, `DELETE FROM title_relations WHERE title_id = ?`, titleID); err != nil {
		return fmt.Errorf("title_relations delete for title: %w", err)
	}
	return nil
}

func joinStrings(elems []string, sep string) string {
	res := ""
	for i, s := range elems {
		if i > 0 {
			res += sep
		}
		res += s
	}
	return res
}
