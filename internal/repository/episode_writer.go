package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// EpisodeWriter performs write operations on episodes within a caller-owned
// transaction. Accepting only *sql.Tx makes "write to the pool without a
// transaction" a compile-time error — the same class of bug that used to
// surface as SQLite BUSY deadlocks or partially-applied multi-statement writes.
type EpisodeWriter struct {
	tx *sql.Tx
}

func NewEpisodeWriter(tx *sql.Tx) *EpisodeWriter {
	return &EpisodeWriter{tx: tx}
}

// GetOrCreate returns the episode for the given season and number, creating
// it if needed. The row is read inside the caller's transaction so the insert
// never races a concurrent writer.
func (w *EpisodeWriter) GetOrCreate(ctx context.Context, seasonID int64, episodeNumber int) (*model.Episode, error) {
	var e model.Episode
	err := w.tx.QueryRowContext(ctx,
		`SELECT id, season_id, episode, name, air_date, watched, first_watched_at, last_watched_at, plex_rating_key FROM episodes WHERE season_id = ? AND episode = ?`,
		seasonID, episodeNumber,
	).Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.FirstWatchedAt, &e.LastWatchedAt, &e.PlexRatingKey)
	if err == nil {
		return &e, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get episode: %w", err)
	}

	res, err := w.tx.ExecContext(ctx, `INSERT INTO episodes (season_id, episode) VALUES (?, ?)`, seasonID, episodeNumber)
	if err != nil {
		return nil, fmt.Errorf("create episode: %w", err)
	}

	id, _ := res.LastInsertId()
	return &model.Episode{ID: id, SeasonID: seasonID, Episode: episodeNumber}, nil
}

func (w *EpisodeWriter) ToggleWatched(ctx context.Context, id int64) (*model.Episode, error) {
	var e model.Episode
	err := w.tx.QueryRowContext(ctx,
		`UPDATE episodes
		 SET watched = CASE WHEN watched = 1 THEN 0 ELSE 1 END,
		     first_watched_at = CASE WHEN watched = 1 THEN NULL ELSE COALESCE(first_watched_at, ?) END,
		     last_watched_at  = CASE WHEN watched = 1 THEN NULL ELSE ? END
		 WHERE id = ?
		 RETURNING id, season_id, episode, name, air_date, watched, first_watched_at, last_watched_at, plex_rating_key`,
		time.Now().UTC(), time.Now().UTC(), id,
	).Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.FirstWatchedAt, &e.LastWatchedAt, &e.PlexRatingKey)
	if err != nil {
		return nil, fmt.Errorf("toggle episode: %w", err)
	}
	return &e, nil
}

func (w *EpisodeWriter) BatchMarkWatched(ctx context.Context, ids []int64, watchedAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+2)
	args = append(args, watchedAt.UTC(), watchedAt.UTC())
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := fmt.Sprintf(
		`UPDATE episodes
		 SET watched = 1,
		     first_watched_at = CASE WHEN first_watched_at IS NULL THEN ? ELSE first_watched_at END,
		     last_watched_at  = ?
		 WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	if _, err := w.tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("batch mark watched: %w", err)
	}
	return nil
}

// UpdateMetadata sets name and air_date on an episode, only if the new value is non-empty.
func (w *EpisodeWriter) UpdateMetadata(ctx context.Context, id int64, name, airDate string) error {
	var sets []string
	var args []interface{}

	if name != "" {
		sets = append(sets, "name = ?")
		args = append(args, name)
	}
	if airDate != "" {
		sets = append(sets, "air_date = ?")
		args = append(args, airDate)
	}
	if len(sets) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE episodes SET %s WHERE id = ?", strings.Join(sets, ", "))
	if _, err := w.tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update episode metadata: %w", err)
	}
	return nil
}

// UpsertBatch creates or updates episodes for a season in a single query.
// Collapses the per-episode GetOrCreate + UpdateMetadata pattern into one round-trip.
func (w *EpisodeWriter) UpsertBatch(ctx context.Context, seasonID int64, entries []EpisodeUpsert) error {
	if len(entries) == 0 {
		return nil
	}
	placeholders := make([]string, len(entries))
	args := make([]interface{}, 0, len(entries)*4)
	for i, e := range entries {
		placeholders[i] = "(?, ?, ?, ?)"
		args = append(args, seasonID, e.EpisodeNumber, e.Name, e.AirDate)
	}
	query := fmt.Sprintf(
		`INSERT INTO episodes (season_id, episode, name, air_date) VALUES %s
		 ON CONFLICT(season_id, episode) DO UPDATE SET
		   name    = CASE WHEN excluded.name    != '' THEN excluded.name    ELSE name    END,
		   air_date = CASE WHEN excluded.air_date != '' THEN excluded.air_date ELSE air_date END`,
		strings.Join(placeholders, ", "),
	)
	if _, err := w.tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert episodes: %w", err)
	}
	return nil
}

func (w *EpisodeWriter) MarkWatched(ctx context.Context, id int64, watchedAt time.Time) error {
	_, err := w.tx.ExecContext(ctx,
		`UPDATE episodes
		 SET watched = 1,
		     first_watched_at = CASE WHEN first_watched_at IS NULL THEN ? ELSE first_watched_at END,
		     last_watched_at  = ?
		 WHERE id = ?`,
		watchedAt.UTC(), watchedAt.UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("mark watched: %w", err)
	}
	return nil
}

// UpdateLastWatchedAt sets last_watched_at on an episode without touching first_watched_at.
// Used for rewatch events (media.play on already-watched episodes).
func (w *EpisodeWriter) UpdateLastWatchedAt(ctx context.Context, id int64, at time.Time) error {
	if _, err := w.tx.ExecContext(ctx, `UPDATE episodes SET last_watched_at = ? WHERE id = ?`, at.UTC(), id); err != nil {
		return fmt.Errorf("update episode last watched at: %w", err)
	}
	return nil
}
