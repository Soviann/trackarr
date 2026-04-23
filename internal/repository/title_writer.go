package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// TitleWriter performs write operations on titles within a caller-owned
// transaction. Accepting only *sql.Tx makes "write to the pool without a
// transaction" a compile-time error: that class of bug used to surface as
// SQLite BUSY deadlocks or partially-applied multi-statement writes.
type TitleWriter struct {
	tx *sql.Tx
}

func NewTitleWriter(tx *sql.Tx) *TitleWriter {
	return &TitleWriter{tx: tx}
}

// Create inserts a title plus its names. Caller must open the transaction.
func (w *TitleWriter) Create(ctx context.Context, title *model.Title, names []model.TitleName) (int64, error) {
	res, err := w.tx.ExecContext(ctx, `
		INSERT INTO titles (type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, total_watch_minutes, tmdb_rating, credits, anilist_rating, release_date, next_air_date, next_air_episode)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		title.Type, title.IsAnime, title.Year, title.CoverURL, title.IMDBID, title.AniListID, title.TMDBID, title.TVDBID,
		title.PlexRatingKey, title.MyRating, title.Status, title.SeriesStatus, title.MatchStatus,
		title.OriginalTitle, title.MatchSource,
		title.Overview, title.Runtime, title.TotalWatchMinutes, title.TMDBRating, title.Credits, title.AniListRating,
		title.ReleaseDate, title.NextAirDate, title.NextAirEpisode,
	)
	if err != nil {
		return 0, fmt.Errorf("insert title: %w", err)
	}

	id, _ := res.LastInsertId()

	for _, name := range names {
		_, err := w.tx.ExecContext(ctx, `INSERT INTO title_names (title_id, name, language, is_primary) VALUES (?, ?, ?, ?)`,
			id, name.Name, name.Language, name.IsPrimary)
		if err != nil {
			return 0, fmt.Errorf("insert title name: %w", err)
		}
	}

	return id, nil
}

// Update applies a partial update. Nil fields on TitleUpdate are left untouched.
func (w *TitleWriter) Update(ctx context.Context, id int64, update TitleUpdate) error {
	var sets []string
	var args []interface{}

	if update.Status != nil {
		sets = append(sets, `status = ?`)
		args = append(args, *update.Status)
	}
	if update.MatchStatus != nil {
		sets = append(sets, `match_status = ?`)
		args = append(args, *update.MatchStatus)
	}
	if update.MyRating != nil {
		sets = append(sets, `my_rating = ?`)
		args = append(args, *update.MyRating)
	}
	if update.SeriesStatus != nil {
		sets = append(sets, `series_status = ?`)
		args = append(args, *update.SeriesStatus)
	}
	if update.CoverURL != nil {
		sets = append(sets, `cover_url = ?`)
		args = append(args, *update.CoverURL)
	}
	if update.IMDBID != nil {
		sets = append(sets, `imdb_id = ?`)
		args = append(args, *update.IMDBID)
	}
	if update.AniListID != nil {
		sets = append(sets, `anilist_id = ?`)
		args = append(args, *update.AniListID)
	}
	if update.TMDBID != nil {
		sets = append(sets, `tmdb_id = ?`)
		args = append(args, *update.TMDBID)
	}
	if update.TVDBID != nil {
		sets = append(sets, `tvdb_id = ?`)
		args = append(args, *update.TVDBID)
	}
	if update.PlexRatingKey != nil {
		sets = append(sets, `plex_rating_key = ?`)
		args = append(args, *update.PlexRatingKey)
	}
	if update.MatchSource != nil {
		sets = append(sets, `match_source = ?`)
		args = append(args, *update.MatchSource)
	}
	if update.OriginalTitle != nil {
		sets = append(sets, `original_title = ?`)
		args = append(args, *update.OriginalTitle)
	}
	if update.Type != nil {
		sets = append(sets, `type = ?`)
		args = append(args, *update.Type)
	}
	if update.IsAnime != nil {
		sets = append(sets, `is_anime = ?`)
		args = append(args, *update.IsAnime)
	}
	if update.Overview != nil {
		sets = append(sets, `overview = ?`)
		args = append(args, *update.Overview)
	}
	if update.Runtime != nil {
		sets = append(sets, `runtime = ?`)
		args = append(args, *update.Runtime)
	}
	if update.TotalWatchMinutes != nil {
		sets = append(sets, `total_watch_minutes = ?`)
		args = append(args, *update.TotalWatchMinutes)
	}
	if update.TMDBRating != nil {
		sets = append(sets, `tmdb_rating = ?`)
		args = append(args, *update.TMDBRating)
	}
	if update.Credits != nil {
		sets = append(sets, `credits = ?`)
		args = append(args, *update.Credits)
	}
	if update.AniListRating != nil {
		sets = append(sets, `anilist_rating = ?`)
		args = append(args, *update.AniListRating)
	}
	if update.ReleaseDate != nil {
		sets = append(sets, `release_date = ?`)
		args = append(args, *update.ReleaseDate)
	}
	if update.NextAirDate != nil {
		sets = append(sets, `next_air_date = ?`)
		args = append(args, *update.NextAirDate)
	}
	if update.NextAirEpisode != nil {
		sets = append(sets, `next_air_episode = ?`)
		args = append(args, *update.NextAirEpisode)
	}

	if len(sets) == 0 {
		return nil
	}

	sets = append(sets, `updated_at = CURRENT_TIMESTAMP`)
	args = append(args, id)

	_, err := w.tx.ExecContext(ctx, `UPDATE titles SET `+strings.Join(sets, `, `)+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update title: %w", err)
	}

	return nil
}

// UpdateLastWatchedAt advances last_watched_at only forward — never overwrites
// with an older value, so out-of-order webhooks cannot rewind the timestamp.
func (w *TitleWriter) UpdateLastWatchedAt(ctx context.Context, id int64, at time.Time) error {
	_, err := w.tx.ExecContext(ctx, `UPDATE titles SET last_watched_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND (last_watched_at IS NULL OR ? > last_watched_at)`, at, id, at)
	if err != nil {
		return fmt.Errorf("update last watched at: %w", err)
	}
	return nil
}

// ReplaceNames wipes and re-inserts the names for a title. Must run in the
// same transaction as the caller so readers never observe an empty set.
func (w *TitleWriter) ReplaceNames(ctx context.Context, titleID int64, names []model.TitleName) error {
	if _, err := w.tx.ExecContext(ctx, `DELETE FROM title_names WHERE title_id = ?`, titleID); err != nil {
		return fmt.Errorf("delete title names: %w", err)
	}
	if len(names) == 0 {
		return nil
	}
	placeholders := make([]string, len(names))
	args := make([]interface{}, 0, len(names)*4)
	for i, n := range names {
		placeholders[i] = "(?, ?, ?, ?)"
		args = append(args, titleID, n.Name, n.Language, n.IsPrimary)
	}
	query := fmt.Sprintf(`INSERT INTO title_names (title_id, name, language, is_primary) VALUES %s`, strings.Join(placeholders, ","))
	if _, err := w.tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert title names: %w", err)
	}
	return nil
}

// Merge consolidates sourceID into destID. Moves seasons (shifting their
// number by seasonOffset), names, watch events, and external IDs before
// deleting the source title. All steps share the caller's transaction so
// a partial merge cannot leak to readers.
func (w *TitleWriter) Merge(ctx context.Context, destID, sourceID int64, seasonOffset int) error {
	// 1. Move seasons. Guards against UNIQUE(title_id, season_number) by
	// merging colliding seasons instead of re-parenting blindly.
	rows, err := w.tx.QueryContext(ctx, `SELECT id, season_number FROM seasons WHERE title_id = ?`, sourceID)
	if err != nil {
		return fmt.Errorf("get source seasons: %w", err)
	}

	type seasonMove struct {
		id     int64
		newNum int
	}
	var moves []seasonMove
	for rows.Next() {
		var sm seasonMove
		var oldNum int
		if err := rows.Scan(&sm.id, &oldNum); err != nil {
			rows.Close()
			return err
		}
		sm.newNum = oldNum + seasonOffset
		moves = append(moves, sm)
	}
	rows.Close()

	for _, m := range moves {
		var targetSeasonID int64
		err := w.tx.QueryRowContext(ctx, `SELECT id FROM seasons WHERE title_id = ? AND season_number = ?`, destID, m.newNum).Scan(&targetSeasonID)
		switch {
		case err == sql.ErrNoRows:
			if _, err := w.tx.ExecContext(ctx, `UPDATE seasons SET title_id = ?, season_number = ? WHERE id = ?`, destID, m.newNum, m.id); err != nil {
				return fmt.Errorf("move season %d: %w", m.id, err)
			}
		case err != nil:
			return fmt.Errorf("check season collision %d: %w", m.id, err)
		default:
			// Collision: merge episodes into the existing target season.
			// UPDATE OR IGNORE skips episodes whose number already exists in the target season.
			if _, err := w.tx.ExecContext(ctx, `UPDATE OR IGNORE episodes SET season_id = ? WHERE season_id = ?`, targetSeasonID, m.id); err != nil {
				return fmt.Errorf("merge episodes into season %d: %w", targetSeasonID, err)
			}
			// Remaining source episodes are duplicates; their watch_events.episode_id
			// gets set to NULL via ON DELETE SET NULL so history is kept.
			if _, err := w.tx.ExecContext(ctx, `DELETE FROM episodes WHERE season_id = ?`, m.id); err != nil {
				return fmt.Errorf("delete duplicate episodes from season %d: %w", m.id, err)
			}
			if _, err := w.tx.ExecContext(ctx, `DELETE FROM seasons WHERE id = ?`, m.id); err != nil {
				return fmt.Errorf("delete merged season %d: %w", m.id, err)
			}
		}
	}

	// 2. Move names as aliases (is_primary=0); INSERT OR IGNORE dedupes against dest.
	nameRows, err := w.tx.QueryContext(ctx, `SELECT name, language FROM title_names WHERE title_id = ?`, sourceID)
	if err == nil {
		type nameMove struct {
			name string
			lang string
		}
		var names []nameMove
		for nameRows.Next() {
			var nm nameMove
			if err := nameRows.Scan(&nm.name, &nm.lang); err == nil {
				names = append(names, nm)
			}
		}
		nameRows.Close()
		for _, nm := range names {
			_, _ = w.tx.ExecContext(ctx, `INSERT OR IGNORE INTO title_names (title_id, name, language, is_primary) VALUES (?, ?, ?, 0)`, destID, nm.name, nm.lang)
		}
	}

	// 3. Move watch events.
	if _, err := w.tx.ExecContext(ctx, `UPDATE watch_events SET title_id = ? WHERE title_id = ?`, destID, sourceID); err != nil {
		return fmt.Errorf("move watch events: %w", err)
	}

	// 4. Transfer external IDs NULL-only. Never overwriting dest values
	// prevents future webhooks on the source's ratingKey from re-creating
	// a duplicate title.
	if _, err := w.tx.ExecContext(ctx, `UPDATE titles SET
		imdb_id         = COALESCE(imdb_id,         (SELECT imdb_id         FROM titles WHERE id = ?)),
		tmdb_id         = COALESCE(tmdb_id,         (SELECT tmdb_id         FROM titles WHERE id = ?)),
		tvdb_id         = COALESCE(tvdb_id,         (SELECT tvdb_id         FROM titles WHERE id = ?)),
		anilist_id      = COALESCE(anilist_id,      (SELECT anilist_id      FROM titles WHERE id = ?)),
		plex_rating_key = COALESCE(plex_rating_key, (SELECT plex_rating_key FROM titles WHERE id = ?))
		WHERE id = ?`, sourceID, sourceID, sourceID, sourceID, sourceID, destID); err != nil {
		return fmt.Errorf("transfer external ids: %w", err)
	}

	// 5. Delete source title; FK cascades handle whatever we haven't moved.
	if _, err := w.tx.ExecContext(ctx, `DELETE FROM titles WHERE id = ?`, sourceID); err != nil {
		return fmt.Errorf("delete source title: %w", err)
	}

	return nil
}

// Delete removes a title. FK cascades drop seasons, episodes, and watch events.
func (w *TitleWriter) Delete(ctx context.Context, id int64) error {
	_, err := w.tx.ExecContext(ctx, `DELETE FROM titles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete title: %w", err)
	}
	return nil
}

// BatchDelete removes multiple titles in one statement.
func (w *TitleWriter) BatchDelete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `DELETE FROM titles WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	if _, err := w.tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("batch delete titles: %w", err)
	}
	return nil
}

// BatchUpdateStatus updates the status of multiple titles in one statement.
func (w *TitleWriter) BatchUpdateStatus(ctx context.Context, ids []int64, status string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids)+1)
	args[0] = status
	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id
	}
	query := `UPDATE titles SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	if _, err := w.tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("batch update status: %w", err)
	}
	return nil
}
