package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
)

type TitleRelationRepository struct {
	db database.DBTX
}

func NewTitleRelationRepository(db database.DBTX) *TitleRelationRepository {
	return &TitleRelationRepository{db: db}
}

const selectTitleRelationsQuery = `
	SELECT 
		tr.id, tr.title_id, tr.season_id, s.season_number,
		tr.provider, tr.external_id, tr.relation_type, tr.format,
		tr.title, tr.romaji_title, tr.cover_url, tr.year, tr.score,
		tr.episode_count, tr.duration, tr.overview, tr.sort_order,
		tr.created_at, tr.updated_at,
		mt.id AS matched_title_id,
		mt.status AS matched_status,
		mt.my_rating AS matched_rating,
		mt.type AS matched_type,
		mt.radarr_id AS matched_radarr_id,
		mt.sonarr_id AS matched_sonarr_id
	FROM title_relations tr
	LEFT JOIN seasons s ON s.id = tr.season_id
	LEFT JOIN titles mt ON (
		(tr.provider = 'anilist' AND mt.anilist_id = tr.external_id) OR
		(tr.provider = 'tmdb' AND mt.tmdb_id = tr.external_id) OR
		(tr.provider = 'tvdb' AND mt.tvdb_id = tr.external_id)
	)
`

func scanTitleRelation(rows *sql.Rows) (model.TitleRelation, error) {
	var r model.TitleRelation
	var relType, formatStr, createdAtStr, updatedAtStr string
	var matchedStatusStr, matchedTypeStr *string

	err := rows.Scan(
		&r.ID, &r.TitleID, &r.SeasonID, &r.SeasonNumber,
		&r.Provider, &r.ExternalID, &relType, &formatStr,
		&r.Title, &r.RomajiTitle, &r.CoverURL, &r.Year, &r.Score,
		&r.EpisodeCount, &r.Duration, &r.Overview, &r.SortOrder,
		&createdAtStr, &updatedAtStr,
		&r.MatchedTitleID, &matchedStatusStr, &r.MatchedRating,
		&matchedTypeStr, &r.RadarrID, &r.SonarrID,
	)
	if err != nil {
		return r, err
	}

	r.RelationType = model.RelationType(relType)
	r.Format = formatStr
	if matchedStatusStr != nil {
		st := model.TitleStatus(*matchedStatusStr)
		r.MatchedStatus = &st
	}
	if matchedTypeStr != nil {
		tp := model.TitleType(*matchedTypeStr)
		r.MatchedType = &tp
	}
	r.CreatedAt = parseSQLiteTimeVal(createdAtStr)
	r.UpdatedAt = parseSQLiteTimeVal(updatedAtStr)

	return r, nil
}

// GetByTitleID returns all relations for a title (ordered by season, then sort_order / year / title).
func (repo *TitleRelationRepository) GetByTitleID(ctx context.Context, titleID int64) ([]model.TitleRelation, error) {
	query := selectTitleRelationsQuery + `
		WHERE tr.title_id = ?
		ORDER BY (s.season_number IS NULL), s.season_number, tr.sort_order, tr.year, tr.title
	`
	rows, err := repo.db.QueryContext(ctx, query, titleID)
	if err != nil {
		return nil, fmt.Errorf("title_relations get by title_id: %w", err)
	}
	defer rows.Close()

	var out []model.TitleRelation
	for rows.Next() {
		r, err := scanTitleRelation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan title relation: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetBySeasonID returns all relations attached to a specific season.
func (repo *TitleRelationRepository) GetBySeasonID(ctx context.Context, seasonID int64) ([]model.TitleRelation, error) {
	query := selectTitleRelationsQuery + `
		WHERE tr.season_id = ?
		ORDER BY tr.sort_order, tr.year, tr.title
	`
	rows, err := repo.db.QueryContext(ctx, query, seasonID)
	if err != nil {
		return nil, fmt.Errorf("title_relations get by season_id: %w", err)
	}
	defer rows.Close()

	var out []model.TitleRelation
	for rows.Next() {
		r, err := scanTitleRelation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan season title relation: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteForTitle removes all relations for a title.
func (repo *TitleRelationRepository) DeleteForTitle(ctx context.Context, titleID int64) error {
	_, err := repo.db.ExecContext(ctx, `DELETE FROM title_relations WHERE title_id = ?`, titleID)
	if err != nil {
		return fmt.Errorf("title_relations delete for title: %w", err)
	}
	return nil
}
