package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
)

// BackupSeasonExternalID is a portable representation of a season_external_ids row.
type BackupSeasonExternalID struct {
	TitleID             int64   `json:"title_id"`
	SeasonNumber        int     `json:"season_number"`
	Provider            string  `json:"provider"`
	ExternalID          string  `json:"external_id"`
	AniListAverageScore *int    `json:"anilist_average_score,omitempty"`
	AniListEpisodeCount *int    `json:"anilist_episode_count,omitempty"`
	AniListStartDate    *string `json:"anilist_start_date,omitempty"`
	SortOrder           int     `json:"sort_order"`
}

// BackupWatchEvent is a portable representation of a watch_events row.
type BackupWatchEvent struct {
	TitleID       int64                  `json:"title_id"`
	SeasonNumber  *int                   `json:"season_number,omitempty"`
	EpisodeNumber *int                   `json:"episode_number,omitempty"`
	Source        model.WatchEventSource `json:"source"`
	RawPayload    *string                `json:"raw_payload,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// BackupWrappedSnapshot is a portable representation of a wrapped_snapshots row.
type BackupWrappedSnapshot struct {
	Year      int       `json:"year"`
	DataJSON  string    `json:"data_json"`
	CreatedAt time.Time `json:"created_at"`
}

// BackupRepository provides read-only queries for full lossless backups.
type BackupRepository struct {
	db database.DBTX
}

func NewBackupRepository(db database.DBTX) *BackupRepository {
	return &BackupRepository{db: db}
}

// VacuumInto writes a compacted, transactionally consistent SQLite snapshot to destPath.
func (r *BackupRepository) VacuumInto(ctx context.Context, destPath string) error {
	if _, err := r.db.ExecContext(ctx, `VACUUM INTO ?`, destPath); err != nil {
		return fmt.Errorf("vacuum into %s: %w", destPath, err)
	}
	return nil
}

// ListAllSeasonExternalIDs returns all season external ID mappings with their title_id and season_number.
func (r *BackupRepository) ListAllSeasonExternalIDs(ctx context.Context) ([]BackupSeasonExternalID, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.title_id, s.season_number, sei.provider, sei.external_id,
		       sei.anilist_average_score, sei.anilist_episode_count, sei.anilist_start_date, sei.sort_order
		FROM season_external_ids sei
		JOIN seasons s ON s.id = sei.season_id
		ORDER BY s.title_id, s.season_number, sei.sort_order`)
	if err != nil {
		return nil, fmt.Errorf("list all season external ids: %w", err)
	}
	defer rows.Close()

	var out []BackupSeasonExternalID
	for rows.Next() {
		var item BackupSeasonExternalID
		if err := rows.Scan(
			&item.TitleID, &item.SeasonNumber, &item.Provider, &item.ExternalID,
			&item.AniListAverageScore, &item.AniListEpisodeCount, &item.AniListStartDate, &item.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("scan season external id: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListAllWatchEvents returns all watch_events joined with season/episode numbers when applicable.
func (r *BackupRepository) ListAllWatchEvents(ctx context.Context) ([]BackupWatchEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT we.title_id, s.season_number, e.episode, we.source, we.raw_payload, we.created_at
		FROM watch_events we
		LEFT JOIN episodes e ON e.id = we.episode_id
		LEFT JOIN seasons s ON s.id = e.season_id
		ORDER BY we.id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list all watch events: %w", err)
	}
	defer rows.Close()

	var out []BackupWatchEvent
	for rows.Next() {
		var item BackupWatchEvent
		if err := rows.Scan(&item.TitleID, &item.SeasonNumber, &item.EpisodeNumber, &item.Source, &item.RawPayload, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan watch event: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListAllTitleRelations returns all title_relations rows for backup export.
func (r *BackupRepository) ListAllTitleRelations(ctx context.Context) ([]model.TitleRelation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tr.id, tr.title_id, tr.season_id, s.season_number, tr.provider, tr.external_id,
		       tr.relation_type, tr.format, tr.title, tr.romaji_title, tr.cover_url,
		       tr.year, tr.score, tr.episode_count, tr.duration
		FROM title_relations tr
		LEFT JOIN seasons s ON s.id = tr.season_id
		ORDER BY tr.title_id, tr.id`)
	if err != nil {
		return nil, fmt.Errorf("list all title relations: %w", err)
	}
	defer rows.Close()

	var out []model.TitleRelation
	for rows.Next() {
		var rel model.TitleRelation
		if err := rows.Scan(
			&rel.ID, &rel.TitleID, &rel.SeasonID, &rel.SeasonNumber, &rel.Provider, &rel.ExternalID,
			&rel.RelationType, &rel.Format, &rel.Title, &rel.RomajiTitle, &rel.CoverURL,
			&rel.Year, &rel.Score, &rel.EpisodeCount, &rel.Duration,
		); err != nil {
			return nil, fmt.Errorf("scan title relation: %w", err)
		}
		out = append(out, rel)
	}
	return out, rows.Err()
}

// ListAllWrappedSnapshots returns all wrapped_snapshots rows for backup export.
func (r *BackupRepository) ListAllWrappedSnapshots(ctx context.Context) ([]BackupWrappedSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT year, data_json, created_at FROM wrapped_snapshots ORDER BY year ASC`)
	if err != nil {
		return nil, fmt.Errorf("list wrapped snapshots: %w", err)
	}
	defer rows.Close()

	var out []BackupWrappedSnapshot
	for rows.Next() {
		var item BackupWrappedSnapshot
		if err := rows.Scan(&item.Year, &item.DataJSON, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan wrapped snapshot: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// BackupWriter performs transactional restore writes for v2 backups.
type BackupWriter struct {
	tx *sql.Tx
}

func NewBackupWriter(tx *sql.Tx) *BackupWriter {
	database.MarkTxDirty(tx, database.TagTitles, database.TagStats, database.TagCalendar)
	return &BackupWriter{tx: tx}
}

func (w *BackupWriter) RestoreSeasonExternalID(ctx context.Context, seasonID int64, item BackupSeasonExternalID) error {
	_, err := w.tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO season_external_ids
			(season_id, provider, external_id, anilist_average_score, anilist_episode_count, anilist_start_date, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		seasonID, item.Provider, item.ExternalID,
		item.AniListAverageScore, item.AniListEpisodeCount, item.AniListStartDate, item.SortOrder,
	)
	return err
}

func (w *BackupWriter) RestoreWatchEvent(ctx context.Context, titleID int64, episodeID *int64, item BackupWatchEvent) error {
	_, err := w.tx.ExecContext(ctx, `
		INSERT INTO watch_events (title_id, episode_id, source, raw_payload, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		titleID, episodeID, item.Source, item.RawPayload, item.CreatedAt.UTC(),
	)
	return err
}

func (w *BackupWriter) RestoreWrappedSnapshot(ctx context.Context, item BackupWrappedSnapshot) error {
	_, err := w.tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO wrapped_snapshots (year, data_json, created_at)
		VALUES (?, ?, ?)`,
		item.Year, item.DataJSON, item.CreatedAt.UTC(),
	)
	return err
}

func (w *BackupWriter) UpdateEpisodeState(ctx context.Context, epID int64, ep model.Episode) error {
	_, err := w.tx.ExecContext(ctx, `
		UPDATE episodes
		SET name = COALESCE(?, name),
		    air_date = COALESCE(?, air_date),
		    watched = ?,
		    first_watched_at = ?,
		    last_watched_at = ?,
		    downloaded_at = ?,
		    external_source_id = COALESCE(?, external_source_id)
		WHERE id = ?`,
		ep.Name, ep.AirDate, ep.Watched, ep.FirstWatchedAt, ep.LastWatchedAt, ep.DownloadedAt, ep.ExternalSourceID, epID,
	)
	return err
}

func (w *BackupWriter) UpdateTitleExtraFields(ctx context.Context, titleID int64, t model.Title) error {
	var watchProvidersJSON *string
	if len(t.WatchProviders) > 0 {
		if b, err := json.Marshal(t.WatchProviders); err == nil {
			s := string(b)
			watchProvidersJSON = &s
		}
	}
	_, err := w.tx.ExecContext(ctx, `
		UPDATE titles
		SET personal_notes = ?,
		    watch_providers = ?,
		    origin_country = ?,
		    downloaded_at = ?,
		    sonarr_deleted_at = ?,
		    last_watched_at = ?,
		    last_refreshed_at = ?,
		    accent_hex = ?
		WHERE id = ?`,
		t.PersonalNotes, watchProvidersJSON, t.OriginCountry, t.DownloadedAt, t.SonarrDeletedAt, t.LastWatchedAt, t.LastRefreshedAt, t.AccentHex, titleID,
	)
	return err
}
