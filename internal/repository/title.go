package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
)

// ContinueWatchingItem represents a Watching title with episode progress.
type ContinueWatchingItem struct {
	ID              int64   `json:"id"`
	Type            string  `json:"type"`
	CoverURL        *string `json:"cover_url"`
	Name            string  `json:"name"`
	NextAirEpisode  *string `json:"next_air_episode"`
	WatchedEpisodes int     `json:"watched_episodes"`
	TotalEpisodes   int     `json:"total_episodes"`
	LastWatchedAt   *string `json:"last_watched_at"`
}

// UpcomingItem represents a title with an upcoming air date.
type UpcomingItem struct {
	ID             int64   `json:"id"`
	Type           string  `json:"type"`
	CoverURL       *string `json:"cover_url"`
	Name           string  `json:"name"`
	NextAirDate    string  `json:"next_air_date"`
	NextAirEpisode *string `json:"next_air_episode"`
	Status         string  `json:"status"`
}

type TitleRepository struct {
	db database.DBTX
}

func NewTitleRepository(db database.DBTX) *TitleRepository {
	return &TitleRepository{db: db}
}

type TitleUpdate struct {
	Status            *model.TitleStatus
	MatchStatus       *model.MatchStatus
	MyRating          *int
	SeriesStatus      *model.SeriesStatus
	CoverURL          *string
	IMDBID            *string
	AniListID         *int64
	TMDBID            *int64
	TVDBID            *int64
	PlexRatingKey     *string
	MatchSource       *string
	OriginalTitle     *string
	Type              *model.TitleType
	IsAnime           *bool
	Overview          *string
	Runtime           *int
	TotalWatchMinutes *int
	TMDBRating        *float64
	Credits           *string
	AniListRating     *int
	ReleaseDate       *string
	NextAirDate       *string
	NextAirEpisode    *string
	AccentHex         *string
}

func (r *TitleRepository) GetByID(id int64) (*model.Title, error) {
	title := &model.Title{}
	var firstWatchedAtStr, lastWatchedAtStr, lastRefreshedAtStr *string
	err := r.db.QueryRow(`SELECT id, type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, total_watch_minutes, tmdb_rating, credits, anilist_rating, release_date, next_air_date, next_air_episode, first_watched_at, last_watched_at, last_refreshed_at, accent_hex, created_at, updated_at FROM titles WHERE id = ?`, id).
		Scan(&title.ID, &title.Type, &title.IsAnime, &title.Year, &title.CoverURL, &title.IMDBID, &title.AniListID, &title.TMDBID, &title.TVDBID,
			&title.PlexRatingKey, &title.MyRating, &title.Status, &title.SeriesStatus, &title.MatchStatus, &title.OriginalTitle, &title.MatchSource,
			&title.Overview, &title.Runtime, &title.TotalWatchMinutes, &title.TMDBRating, &title.Credits, &title.AniListRating,
			&title.ReleaseDate, &title.NextAirDate, &title.NextAirEpisode, &firstWatchedAtStr, &lastWatchedAtStr, &lastRefreshedAtStr, &title.AccentHex, &title.CreatedAt, &title.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get title: %w", err)
	}
	title.FirstWatchedAt = parseSQLiteTime(firstWatchedAtStr)
	title.LastWatchedAt = parseSQLiteTime(lastWatchedAtStr)
	title.LastRefreshedAt = parseSQLiteTime(lastRefreshedAtStr)

	// Load names
	rows, err := r.db.Query(`SELECT id, title_id, name, language, is_primary FROM title_names WHERE title_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("get title names: %w", err)
	}

	for rows.Next() {
		var n model.TitleName
		if err := rows.Scan(&n.ID, &n.TitleID, &n.Name, &n.Language, &n.IsPrimary); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan title name: %w", err)
		}
		title.Names = append(title.Names, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate title names: %w", err)
	}
	rows.Close()

	// Load genres from title_genres
	genreRows, err := r.db.Query(`SELECT genre FROM title_genres WHERE title_id = ? ORDER BY genre`, id)
	if err != nil {
		return nil, fmt.Errorf("get title genres: %w", err)
	}
	for genreRows.Next() {
		var g string
		if err := genreRows.Scan(&g); err != nil {
			genreRows.Close()
			return nil, fmt.Errorf("scan genre: %w", err)
		}
		title.Genres = append(title.Genres, g)
	}
	if err := genreRows.Err(); err != nil {
		genreRows.Close()
		return nil, fmt.Errorf("iterate title genres: %w", err)
	}
	genreRows.Close()

	// Load seasons. Detail path joins season_external_ids (anilist) and
	// the per-season AniList score so the title detail UI can render the
	// per-season info strip without an extra round-trip. The listing
	// path (loadTitleRelationsLight) intentionally skips these fields.
	seasonRows, err := r.db.Query(`
		SELECT s.id, s.title_id, s.season_number, s.total_episodes,
		       sei.external_id AS anilist_id,
		       s.anilist_average_score
		FROM seasons s
		LEFT JOIN season_external_ids sei
		       ON sei.season_id = s.id AND sei.provider = 'anilist'
		WHERE s.title_id = ?
		ORDER BY s.season_number`, id)
	if err != nil {
		return nil, fmt.Errorf("get seasons: %w", err)
	}

	for seasonRows.Next() {
		var s model.Season
		if err := seasonRows.Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes,
			&s.AniListID, &s.AniListAverageScore); err != nil {
			seasonRows.Close()
			return nil, fmt.Errorf("scan season: %w", err)
		}
		s.Episodes = []model.Episode{}
		title.Seasons = append(title.Seasons, s)
	}
	if err := seasonRows.Err(); err != nil {
		seasonRows.Close()
		return nil, fmt.Errorf("iterate seasons: %w", err)
	}
	seasonRows.Close()

	// Load all episodes in one query (seasons cursor is closed above; safe with MaxOpenConns=1)
	epRows, err := r.db.Query(`
		SELECT e.id, e.season_id, e.episode, e.name, e.air_date, e.watched, e.first_watched_at, e.last_watched_at, e.plex_rating_key
		FROM episodes e
		JOIN seasons s ON e.season_id = s.id
		WHERE s.title_id = ?
		ORDER BY s.season_number, e.episode`, id)
	if err != nil {
		return nil, fmt.Errorf("get episodes: %w", err)
	}
	grouped := make(map[int64][]model.Episode)
	for epRows.Next() {
		var e model.Episode
		if err := epRows.Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.FirstWatchedAt, &e.LastWatchedAt, &e.PlexRatingKey); err != nil {
			epRows.Close()
			return nil, fmt.Errorf("scan episode: %w", err)
		}
		grouped[e.SeasonID] = append(grouped[e.SeasonID], e)
	}
	if err := epRows.Err(); err != nil {
		epRows.Close()
		return nil, fmt.Errorf("iterate episodes: %w", err)
	}
	epRows.Close()
	for i := range title.Seasons {
		if eps, ok := grouped[title.Seasons[i].ID]; ok {
			title.Seasons[i].Episodes = eps
		}
	}

	return title, nil
}

// TitleLite is a lean projection of a title used by background refresh and
// cover fetch flows. They iterate the entire library but only read external
// IDs, type/status flags, the cover URL and a display name — loading
// title_names / seasons / episodes via loadTitleRelations was allocating
// dozens of MB per daily refresh for nothing.
type TitleLite struct {
	ID           int64
	Type         model.TitleType
	IsAnime      bool
	Status       model.TitleStatus
	SeriesStatus *model.SeriesStatus
	CoverURL     *string
	TMDBID       *int64
	TVDBID       *int64
	AniListID    *int64
	PrimaryName  string
}

// titleLiteCols is the column list for TitleLite scans, embedding
// displayNameExpr (which assumes the outer titles row is aliased `t`).
const titleLiteCols = `t.id, t.type, t.is_anime, t.status, t.series_status, t.cover_url,
		t.tmdb_id, t.tvdb_id, t.anilist_id,
		COALESCE(` + displayNameExpr + `, '') AS name`

func scanTitleLite(scanner interface {
	Scan(dest ...any) error
}, t *TitleLite) error {
	return scanner.Scan(
		&t.ID, &t.Type, &t.IsAnime, &t.Status, &t.SeriesStatus, &t.CoverURL,
		&t.TMDBID, &t.TVDBID, &t.AniListID,
		&t.PrimaryName,
	)
}

// ListAllForRefresh returns lean projections of every title for background
// refresh and cover-fetch loops. Skips loadTitleRelations.
func (r *TitleRepository) ListAllForRefresh(ctx context.Context) ([]TitleLite, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+titleLiteCols+`
		FROM titles t
		ORDER BY t.updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list titles for refresh: %w", err)
	}
	defer rows.Close()

	var titles []TitleLite
	for rows.Next() {
		var t TitleLite
		if err := scanTitleLite(rows, &t); err != nil {
			return nil, fmt.Errorf("scan title lite: %w", err)
		}
		titles = append(titles, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate titles for refresh: %w", err)
	}
	return titles, nil
}

// GetLiteByID returns the lean projection used by single-title refresh paths
// (admin RefreshOne, retry-from-task-queue) so they don't pay loadTitleRelations
// cost when they only need IDs/status/cover.
func (r *TitleRepository) GetLiteByID(ctx context.Context, id int64) (*TitleLite, error) {
	var t TitleLite
	err := scanTitleLite(r.db.QueryRowContext(ctx, `
		SELECT `+titleLiteCols+`
		FROM titles t WHERE t.id = ?`, id), &t)
	if err != nil {
		return nil, fmt.Errorf("get title lite: %w", err)
	}
	return &t, nil
}

// ListAll returns all titles with full relations (names, seasons, episodes). Used by background jobs.
func (r *TitleRepository) ListAll() ([]model.Title, error) {
	rows, err := r.db.Query(`SELECT id, type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, total_watch_minutes, tmdb_rating, credits, anilist_rating, release_date, next_air_date, next_air_episode, last_watched_at, last_refreshed_at, accent_hex, created_at, updated_at FROM titles ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all titles: %w", err)
	}

	var titles []model.Title
	for rows.Next() {
		var t model.Title
		var lastWatchedAtStr, lastRefreshedAtStr *string
		if err := rows.Scan(&t.ID, &t.Type, &t.IsAnime, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
			&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
			&t.Overview, &t.Runtime, &t.TotalWatchMinutes, &t.TMDBRating, &t.Credits, &t.AniListRating,
			&t.ReleaseDate, &t.NextAirDate, &t.NextAirEpisode, &lastWatchedAtStr, &lastRefreshedAtStr, &t.AccentHex, &t.CreatedAt, &t.UpdatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan title: %w", err)
		}
		t.LastWatchedAt = parseSQLiteTime(lastWatchedAtStr)
		t.LastRefreshedAt = parseSQLiteTime(lastRefreshedAtStr)
		titles = append(titles, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate all titles: %w", err)
	}
	rows.Close()

	return r.loadTitleRelations(titles)
}

// FindByExternalID looks up a title by external IDs (IMDB, TMDB, AniList, Plex rating key).
// If titleType is non-nil, results are filtered by type (useful because TMDB IDs
// are only unique within a media type).
func (r *TitleRepository) FindByExternalID(imdbID *string, tmdbID *int64, plexRatingKey *string, anilistID *int64, titleType *model.TitleType) (*model.Title, error) {
	var conditions []string
	var args []interface{}

	if imdbID != nil && *imdbID != "" {
		conditions = append(conditions, `imdb_id = ?`)
		args = append(args, *imdbID)
	}
	if tmdbID != nil && *tmdbID != 0 {
		conditions = append(conditions, `tmdb_id = ?`)
		args = append(args, *tmdbID)
	}
	if plexRatingKey != nil && *plexRatingKey != "" {
		conditions = append(conditions, `plex_rating_key = ?`)
		args = append(args, *plexRatingKey)
	}
	if anilistID != nil && *anilistID != 0 {
		conditions = append(conditions, `anilist_id = ?`)
		args = append(args, *anilistID)
	}

	if len(conditions) == 0 {
		return nil, sql.ErrNoRows
	}

	query := `SELECT id FROM titles WHERE (` + strings.Join(conditions, ` OR `) + `)`
	if titleType != nil {
		query += ` AND type = ?`
		args = append(args, *titleType)
	}
	query += ` LIMIT 1`

	var id int64
	if err := r.db.QueryRow(query, args...).Scan(&id); err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

// parseSQLiteTime parses a nullable SQLite datetime string into a *time.Time.
// SQLite stores datetimes in UTC without a timezone marker; time.Parse returns
// UTC when no timezone is embedded, which is the expected behaviour here.
// Accepted formats: "2006-01-02 15:04:05" (SQLite default) or RFC3339.
func parseSQLiteTime(s *string) *time.Time {
	if s == nil {
		return nil
	}
	// Try standard SQLite datetime format
	t, err := time.Parse("2006-01-02 15:04:05", *s)
	if err == nil {
		return &t
	}
	// Try RFC3339 (ISO)
	t, err = time.Parse(time.RFC3339, *s)
	if err == nil {
		return &t
	}
	return nil
}

// HasUnwatchedEpisodes returns true if the title has at least one unwatched episode.
func (r *TitleRepository) HasUnwatchedEpisodes(titleID int64) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM episodes e
			JOIN seasons s ON e.season_id = s.id
			WHERE s.title_id = ? AND e.watched = 0
		)`
	var exists bool
	if err := r.db.QueryRow(query, titleID).Scan(&exists); err != nil {
		return false, fmt.Errorf("has unwatched episodes: %w", err)
	}
	return exists, nil
}

// GetUsedCoversInBatch returns a map containing the subset of filenames that are currently in use.
func (r *TitleRepository) GetUsedCoversInBatch(filenames []string) (map[string]bool, error) {
	if len(filenames) == 0 {
		return make(map[string]bool), nil
	}

	placeholders := make([]string, len(filenames))
	args := make([]interface{}, len(filenames))
	for i, name := range filenames {
		placeholders[i] = "?"
		args[i] = name
	}

	query := fmt.Sprintf(`SELECT cover_url FROM titles WHERE cover_url IN (%s)`, strings.Join(placeholders, ","))
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get used covers in batch: %w", err)
	}
	defer rows.Close()

	used := make(map[string]bool)
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("scan used cover: %w", err)
		}
		used[url] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate used covers: %w", err)
	}

	return used, nil
}

// ListContinueWatching returns Watching titles that have at least one unwatched episode,
// ordered by last_watched_at DESC. Display name priority: fr → en → (x-romaji → ja when anime) → any.
func (r *TitleRepository) ListContinueWatching() ([]ContinueWatchingItem, error) {
	query := `
		SELECT t.id, t.type, t.cover_url,
		       COALESCE(` + displayNameExpr + `, '') AS name,
		       t.next_air_episode,
		       (SELECT COUNT(*) FROM episodes e JOIN seasons s ON e.season_id = s.id WHERE s.title_id = t.id AND e.watched = 1) AS watched_episodes,
		       (SELECT COUNT(*) FROM episodes e JOIN seasons s ON e.season_id = s.id WHERE s.title_id = t.id) AS total_episodes,
		       t.last_watched_at
		FROM titles t
		WHERE t.status = 'watching'
		  AND (SELECT COUNT(*) FROM episodes e JOIN seasons s ON e.season_id = s.id WHERE s.title_id = t.id AND e.watched = 0) > 0
		ORDER BY t.last_watched_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list continue watching: %w", err)
	}
	defer rows.Close()

	var items []ContinueWatchingItem
	for rows.Next() {
		var item ContinueWatchingItem
		if err := rows.Scan(&item.ID, &item.Type, &item.CoverURL, &item.Name, &item.NextAirEpisode,
			&item.WatchedEpisodes, &item.TotalEpisodes, &item.LastWatchedAt); err != nil {
			return nil, fmt.Errorf("scan continue watching: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate continue watching: %w", err)
	}
	return items, nil
}

// ListUpcoming returns Watching and PlanToWatch titles with next_air_date >= today,
// ordered by next_air_date ASC.
func (r *TitleRepository) ListUpcoming(today string) ([]UpcomingItem, error) {
	query := `
		SELECT t.id, t.type, t.cover_url,
		       COALESCE(` + displayNameExpr + `, '') AS name,
		       t.next_air_date, t.next_air_episode, t.status
		FROM titles t
		WHERE t.status IN ('watching', 'plan_to_watch')
		  AND t.next_air_date IS NOT NULL
		  AND t.next_air_date >= ?
		ORDER BY t.next_air_date ASC`

	rows, err := r.db.Query(query, today)
	if err != nil {
		return nil, fmt.Errorf("list upcoming: %w", err)
	}
	defer rows.Close()

	var items []UpcomingItem
	for rows.Next() {
		var item UpcomingItem
		if err := rows.Scan(&item.ID, &item.Type, &item.CoverURL, &item.Name,
			&item.NextAirDate, &item.NextAirEpisode, &item.Status); err != nil {
			return nil, fmt.Errorf("scan upcoming: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate upcoming: %w", err)
	}
	return items, nil
}

// ReviewCount returns the number of titles with match_status in ('pending_review', 'unconfirmed').
func (r *TitleRepository) ReviewCount(_ context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM titles WHERE match_status IN ('pending_review', 'unconfirmed')`).Scan(&count)
	return count, err
}
