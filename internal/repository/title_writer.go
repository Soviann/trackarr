package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
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
		INSERT INTO titles (type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, total_watch_minutes, tmdb_rating, credits, anilist_rating, release_date, next_air_date, next_air_episode, simkl_id, simkl_slug)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		title.Type, title.IsAnime, title.Year, title.CoverURL, title.IMDBID, title.AniListID, title.TMDBID, title.TVDBID,
		title.PlexRatingKey, title.MyRating, title.Status, title.SeriesStatus, title.MatchStatus,
		title.OriginalTitle, title.MatchSource,
		title.Overview, title.Runtime, title.TotalWatchMinutes, title.TMDBRating, title.Credits, title.AniListRating,
		title.ReleaseDate, title.NextAirDate, title.NextAirEpisode, title.SimklID, title.SimklSlug,
	)
	if err != nil {
		return 0, fmt.Errorf("insert title: %w", err)
	}

	id, _ := res.LastInsertId()

	if err := w.insertNames(ctx, id, names); err != nil {
		return 0, err
	}

	return id, nil
}

// insertNames batch-inserts names into title_names. No-op on empty slice.
// Caller owns the transaction.
func (w *TitleWriter) insertNames(ctx context.Context, titleID int64, names []model.TitleName) error {
	if len(names) == 0 {
		return nil
	}
	placeholders := make([]string, len(names))
	args := make([]any, 0, len(names)*4)
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

// Update applies a partial update. Nil fields on TitleUpdate are left untouched.
func (w *TitleWriter) Update(ctx context.Context, id int64, update TitleUpdate) error {
	var sets []string
	var args []any

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
	if update.ClearCoverURL {
		sets = append(sets, `cover_url = NULL`)
	} else if update.CoverURL != nil {
		sets = append(sets, `cover_url = ?`)
		args = append(args, *update.CoverURL)
	}
	if update.ClearIMDBID {
		sets = append(sets, `imdb_id = NULL`)
	} else if update.IMDBID != nil {
		sets = append(sets, `imdb_id = ?`)
		args = append(args, *update.IMDBID)
	}
	if update.ClearAniListID {
		sets = append(sets, `anilist_id = NULL`)
	} else if update.AniListID != nil {
		sets = append(sets, `anilist_id = ?`)
		args = append(args, *update.AniListID)
	}
	if update.ClearTMDBID {
		sets = append(sets, `tmdb_id = NULL`)
	} else if update.TMDBID != nil {
		sets = append(sets, `tmdb_id = ?`)
		args = append(args, *update.TMDBID)
	}
	if update.ClearTVDBID {
		sets = append(sets, `tvdb_id = NULL`)
	} else if update.TVDBID != nil {
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
	if update.WatchProviders != nil {
		sets = append(sets, `watch_providers = ?`)
		args = append(args, *update.WatchProviders)
	}
	if update.AniListRating != nil {
		sets = append(sets, `anilist_rating = ?`)
		args = append(args, *update.AniListRating)
	}
	if update.Year != nil {
		sets = append(sets, `year = ?`)
		args = append(args, *update.Year)
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
	if update.AccentHex != nil {
		sets = append(sets, `accent_hex = ?`)
		args = append(args, *update.AccentHex)
	}
	if update.SimklID != nil {
		sets = append(sets, `simkl_id = ?`)
		args = append(args, *update.SimklID)
	}
	if update.SimklSlug != nil {
		sets = append(sets, `simkl_slug = ?`)
		args = append(args, *update.SimklSlug)
	}
	if update.OriginCountry != nil {
		sets = append(sets, `origin_country = ?`)
		args = append(args, *update.OriginCountry)
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

// MarkRefreshed stamps last_refreshed_at without bumping updated_at, so the
// "fresh meta sync" signal stays orthogonal to user-driven changes (a viewer
// watching an episode must not move this timestamp, and a successful refresh
// must not poison "updated_at DESC" lists with daily noise).
func (w *TitleWriter) MarkRefreshed(ctx context.Context, id int64, at time.Time) error {
	_, err := w.tx.ExecContext(ctx, `UPDATE titles SET last_refreshed_at = ? WHERE id = ?`, at, id)
	if err != nil {
		return fmt.Errorf("mark refreshed: %w", err)
	}
	return nil
}

// AddWatchMinutesForEpisodes grows total_watch_minutes by episodeCount × the
// title's runtime. Used when a backfill marks historical episodes watched so
// their watchtime is reflected in stats. No-op for a non-positive count or a
// title with an unknown runtime. Does not bump updated_at (a metadata heal must
// not poison "updated_at DESC" lists).
func (w *TitleWriter) AddWatchMinutesForEpisodes(ctx context.Context, id int64, episodeCount int64) error {
	if episodeCount <= 0 {
		return nil
	}
	_, err := w.tx.ExecContext(ctx,
		`UPDATE titles SET total_watch_minutes = total_watch_minutes + ? * COALESCE(runtime, 0) WHERE id = ?`,
		episodeCount, id)
	if err != nil {
		return fmt.Errorf("add watch minutes for episodes: %w", err)
	}
	return nil
}

// AddMissingNames inserts names not already stored for the title, matched
// case-insensitively on (name, language). Existing names — anime romaji and
// merged-season aliases included — are never deleted, so a refresh can backfill
// translations without clobbering them. Incoming names are forced non-primary:
// a refreshed title already owns its primary; this only adds alternates. Runs
// in the caller's transaction; the FTS mirror updates via the title_names
// AFTER INSERT trigger.
func (w *TitleWriter) AddMissingNames(ctx context.Context, titleID int64, names []model.TitleName) error {
	if len(names) == 0 {
		return nil
	}
	rows, err := w.tx.QueryContext(ctx, `SELECT name, language FROM title_names WHERE title_id = ?`, titleID)
	if err != nil {
		return fmt.Errorf("list existing names: %w", err)
	}
	seen := make(map[string]struct{})
	for rows.Next() {
		var name, lang string
		if err := rows.Scan(&name, &lang); err != nil {
			rows.Close()
			return fmt.Errorf("scan existing name: %w", err)
		}
		seen[nameKey(name, lang)] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate existing names: %w", err)
	}

	var toInsert []model.TitleName
	for _, n := range names {
		if n.Name == "" {
			continue
		}
		key := nameKey(n.Name, n.Language)
		if _, ok := seen[key]; ok {
			continue // already present, or a duplicate within this batch
		}
		seen[key] = struct{}{}
		toInsert = append(toInsert, model.TitleName{Name: n.Name, Language: n.Language})
	}
	return w.insertNames(ctx, titleID, toInsert)
}

// nameKey builds the case-insensitive dedup key for a (name, language) pair.
// The NUL separator keeps "ab"+"c" distinct from "a"+"bc".
func nameKey(name, language string) string {
	return strings.ToLower(strings.TrimSpace(name)) + "\x00" + language
}

// ReplaceNames wipes and re-inserts the names for a title. Must run in the
// same transaction as the caller so readers never observe an empty set.
func (w *TitleWriter) ReplaceNames(ctx context.Context, titleID int64, names []model.TitleName) error {
	if _, err := w.tx.ExecContext(ctx, `DELETE FROM title_names WHERE title_id = ?`, titleID); err != nil {
		return fmt.Errorf("delete title names: %w", err)
	}
	return w.insertNames(ctx, titleID, names)
}

// Merge consolidates sourceID into destID. Moves seasons (shifting their
// number by seasonOffset), watch events, and external IDs before deleting the
// source title. Source names are deliberately dropped, not copied as aliases. All steps share the caller's transaction so
// a partial merge cannot leak to readers. When aniListID is non-zero, the
// moved/merged dest season is stamped with that AniList mapping in
// season_external_ids (first writer wins — existing dest mappings are kept).
func (w *TitleWriter) Merge(ctx context.Context, destID, sourceID int64, seasonOffset int, aniListID int64) error {
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
	maxSourceNew := 0 // highest dest-space season number the source contributes
	for rows.Next() {
		var sm seasonMove
		var oldNum int
		if err := rows.Scan(&sm.id, &oldNum); err != nil {
			rows.Close()
			return err
		}
		sm.newNum = oldNum + seasonOffset
		if len(moves) == 0 || sm.newNum > maxSourceNew {
			maxSourceNew = sm.newNum
		}
		moves = append(moves, sm)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate source seasons: %w", err)
	}
	rows.Close()

	// Capture the dest's highest season before re-parenting moves it. Combined
	// with maxSourceNew this tells us which block owns the newest season, which
	// drives status reconciliation below.
	var maxDest int
	if err := w.tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(season_number), 0) FROM seasons WHERE title_id = ?`, destID).Scan(&maxDest); err != nil {
		return fmt.Errorf("get dest max season: %w", err)
	}

	for _, m := range moves {
		var targetSeasonID int64
		err := w.tx.QueryRowContext(ctx, `SELECT id FROM seasons WHERE title_id = ? AND season_number = ?`, destID, m.newNum).Scan(&targetSeasonID)
		var finalSeasonID int64
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := w.tx.ExecContext(ctx, `UPDATE seasons SET title_id = ?, season_number = ? WHERE id = ?`, destID, m.newNum, m.id); err != nil {
				return fmt.Errorf("move season %d: %w", m.id, err)
			}
			finalSeasonID = m.id
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
			finalSeasonID = targetSeasonID
		}

		// Append the source's AniList id as a part on the dest season once its
		// ID is known. Stamp does ON CONFLICT(season_id,provider,external_id) DO
		// NOTHING: a different incoming id coexists as a new part (split-cour
		// merges keep both entries), the same id is a no-op, and the dest's
		// existing parts are never clobbered.
		if aniListID != 0 {
			if err := NewSeasonExternalIDWriter(w.tx).Stamp(ctx, finalSeasonID, ProviderAniList, strconv.FormatInt(aniListID, 10)); err != nil {
				return err
			}
		}
	}

	// 2. Source names are intentionally NOT copied. Merges consolidate seasons
	// of one series, so the source's names are just season labels ("Show
	// Season 2") — copying them as aliases polluted the dest and surfaced as
	// noise in search. Plex re-matching is by external ID (FindByExternalID),
	// not by name, so dropping the aliases costs no dedup safety.

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

	// 4b. Reconcile the watch status. The dest keeps its row, so without this it
	// would silently retain its own status and discard the source's (e.g. a
	// dropped sequel merged into a completed S1 would stay "completed"). The
	// newest season's block drives the result; ties keep the dest as anchor.
	var destStatus, sourceStatus model.TitleStatus
	if err := w.tx.QueryRowContext(ctx, `SELECT status FROM titles WHERE id = ?`, destID).Scan(&destStatus); err != nil {
		return fmt.Errorf("get dest status: %w", err)
	}
	if err := w.tx.QueryRowContext(ctx, `SELECT status FROM titles WHERE id = ?`, sourceID).Scan(&sourceStatus); err != nil {
		return fmt.Errorf("get source status: %w", err)
	}
	older, newest := sourceStatus, destStatus
	if maxSourceNew > maxDest {
		older, newest = destStatus, sourceStatus
	}
	if combined := model.CombineMergedStatus(older, newest); combined != destStatus {
		if _, err := w.tx.ExecContext(ctx, `UPDATE titles SET status = ? WHERE id = ?`, combined, destID); err != nil {
			return fmt.Errorf("reconcile merged status: %w", err)
		}
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
