package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
)

// ContinueWatchingItem represents a Watching title with episode progress.
type ContinueWatchingItem struct {
	ID              int64                 `json:"id"`
	Type            string                `json:"type"`
	IsAnime         bool                  `json:"is_anime"`
	CoverURL        *string               `json:"cover_url"`
	Name            string                `json:"name"`
	NextAirEpisode  *string               `json:"next_air_episode"`
	WatchedEpisodes int                   `json:"watched_episodes"`
	TotalEpisodes   int                   `json:"total_episodes"`
	LastWatchedAt   *string               `json:"last_watched_at"`
	WatchProviders  []model.WatchProvider `json:"watch_providers,omitempty"`
	SonarrID        *int64                `json:"sonarr_id,omitempty"`
	RadarrID        *int64                `json:"radarr_id,omitempty"`
	NextEpisode     *model.NextEpisode    `json:"next_episode,omitempty"`
}

type UpcomingItem struct {
	ID             int64                 `json:"id"`
	Type           string                `json:"type"`
	IsAnime        bool                  `json:"is_anime"`
	CoverURL       *string               `json:"cover_url"`
	Name           string                `json:"name"`
	NextAirDate    string                `json:"next_air_date"`
	NextAirEpisode *string               `json:"next_air_episode"`
	Status         string                `json:"status"`
	WatchProviders []model.WatchProvider `json:"watch_providers,omitempty"`
	SonarrID       *int64                `json:"sonarr_id,omitempty"`
	RadarrID       *int64                `json:"radarr_id,omitempty"`
}

// CalendarEventItem represents an air event (episode or title) in the calendar.
type CalendarEventItem struct {
	ID             string                `json:"id"`
	TitleID        int64                 `json:"title_id"`
	TitleName      string                `json:"title_name"`
	Type           string                `json:"type"`
	IsAnime        bool                  `json:"is_anime"`
	CoverURL       *string               `json:"cover_url"`
	AirDate        string                `json:"air_date"`
	EpisodeID      *int64                `json:"episode_id,omitempty"`
	SeasonNumber   *int                  `json:"season_number,omitempty"`
	EpisodeNumber  *int                  `json:"episode_number,omitempty"`
	EpisodeName    *string               `json:"episode_name,omitempty"`
	NextAirEpisode *string               `json:"next_air_episode,omitempty"`
	Status         string                `json:"status"`
	WatchProviders []model.WatchProvider `json:"watch_providers,omitempty"`
	AccentHex      *string               `json:"accent_hex,omitempty"`
	Overview       *string               `json:"overview,omitempty"`
	SonarrID       *int64                `json:"sonarr_id,omitempty"`
	RadarrID       *int64                `json:"radarr_id,omitempty"`
}

// ArrQueueItem represents a title pending Radarr/Sonarr push.
type ArrQueueItem struct {
	ID       int64   `json:"id"`
	Type     string  `json:"type"`
	CoverURL *string `json:"cover_url"`
	Name     string  `json:"name"`
	IsAnime  bool    `json:"is_anime"`
	Year     int     `json:"year"`
	TMDBID   *int64  `json:"tmdb_id"`
	TVDBID   *int64  `json:"tvdb_id"`
}

type TitleRepository struct {
	db database.DBTX
}

func NewTitleRepository(db database.DBTX) *TitleRepository {
	return &TitleRepository{db: db}
}

type TitleUpdate struct {
	Status       *model.TitleStatus
	MatchStatus  *model.MatchStatus
	MyRating     *int
	SeriesStatus *model.SeriesStatus
	CoverURL     *string
	IMDBID       *string
	AniListID    *int64
	TMDBID       *int64
	TVDBID       *int64
	// Clear* flags force the corresponding external ID to NULL. A nil pointer
	// means "leave unchanged"; a set pointer means "set this value"; the Clear
	// flag is the third state — "erase". Used by the manual ID editor so a user
	// can remove a wrong ID for a platform that doesn't carry the title.
	ClearIMDBID    bool
	ClearAniListID bool
	ClearTMDBID    bool
	ClearTVDBID    bool
	// ClearCoverURL resets the cover to NULL — used when the TMDB/TVDB poster
	// source is removed so a later refresh re-derives it (e.g. from AniList).
	ClearCoverURL      bool
	ExternalSourceID   *string
	MatchSource        *string
	OriginalTitle      *string
	Type               *model.TitleType
	IsAnime            *bool
	Overview           *string
	Runtime            *int
	TotalWatchMinutes  *int
	TMDBRating         *float64
	Credits            *string
	AniListRating      *int
	Year               *int
	ReleaseDate        *string
	NextAirDate        *string
	NextAirEpisode     *string
	AccentHex          *string
	SimklID            *int64
	SimklSlug          *string
	RadarrID           *int64
	SonarrID           *int64
	ArrIgnored         *bool
	WatchProviders     *string // JSON array of model.WatchProvider; "[]" clears
	OriginCountry      *string // ISO-3166-1 alpha-2; sets titles.origin_country
	PersonalNotes      *string
	ClearPersonalNotes bool
}

func (r *TitleRepository) GetByID(id int64) (*model.Title, error) {
	title := &model.Title{}
	var firstWatchedAtStr, lastWatchedAtStr, lastRefreshedAtStr *string
	var createdAtStr, updatedAtStr string
	var watchProvidersRaw *string
	err := r.db.QueryRow(`SELECT id, type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, external_source_id, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, total_watch_minutes, tmdb_rating, credits, watch_providers, anilist_rating, release_date, next_air_date, next_air_episode, first_watched_at, last_watched_at, last_refreshed_at, accent_hex, simkl_id, simkl_slug, radarr_id, sonarr_id, arr_ignored, personal_notes, created_at, updated_at FROM titles WHERE id = ?`, id).
		Scan(&title.ID, &title.Type, &title.IsAnime, &title.Year, &title.CoverURL, &title.IMDBID, &title.AniListID, &title.TMDBID, &title.TVDBID,
			&title.ExternalSourceID, &title.MyRating, &title.Status, &title.SeriesStatus, &title.MatchStatus, &title.OriginalTitle, &title.MatchSource,
			&title.Overview, &title.Runtime, &title.TotalWatchMinutes, &title.TMDBRating, &title.Credits, &watchProvidersRaw, &title.AniListRating,
			&title.ReleaseDate, &title.NextAirDate, &title.NextAirEpisode, &firstWatchedAtStr, &lastWatchedAtStr, &lastRefreshedAtStr, &title.AccentHex, &title.SimklID, &title.SimklSlug, &title.RadarrID, &title.SonarrID, &title.ArrIgnored, &title.PersonalNotes, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("get title: %w", err)
	}
	title.CreatedAt = parseSQLiteTimeVal(createdAtStr)
	title.UpdatedAt = parseSQLiteTimeVal(updatedAtStr)
	title.FirstWatchedAt = parseSQLiteTime(firstWatchedAtStr)
	title.LastWatchedAt = parseSQLiteTime(lastWatchedAtStr)
	title.LastRefreshedAt = parseSQLiteTime(lastRefreshedAtStr)
	title.WatchProviders = parseWatchProviders(watchProvidersRaw)

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

	// Load franchise relations (side stories, movies, OVAs, spin-offs)
	rels, err := NewTitleRelationRepository(r.db).GetByTitleID(context.Background(), id)
	if err != nil {
		return nil, fmt.Errorf("get title relations: %w", err)
	}
	title.Relations = rels

	if title.Type == model.TitleTypeMovie {
		return title, nil
	}

	// Load seasons plain (no JOIN on season_external_ids — a season may have
	// multiple AniList parts, and a join would multiply rows). Parts are
	// loaded separately below once this cursor is closed (MaxOpenConns=1).
	seasonRows, err := r.db.Query(`
		SELECT s.id, s.title_id, s.season_number, s.total_episodes
		FROM seasons s
		WHERE s.title_id = ?
		ORDER BY s.season_number`, id)
	if err != nil {
		return nil, fmt.Errorf("get seasons: %w", err)
	}
	for seasonRows.Next() {
		var s model.Season
		if err := seasonRows.Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes); err != nil {
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

	// Attach AniList parts (multiple per season for split-cour). Cursor above is
	// closed first (MaxOpenConns=1). Derive the primary-part aliases for
	// backward-compatible single-link consumers.
	parts := map[int64][]model.AniListPart{}
	pr, err := r.db.Query(`
		SELECT sei.season_id, sei.external_id, sei.anilist_average_score,
		       sei.anilist_episode_count, sei.anilist_start_date, sei.sort_order
		FROM season_external_ids sei
		JOIN seasons s ON s.id = sei.season_id
		WHERE s.title_id = ? AND sei.provider = 'anilist'
		ORDER BY (sei.sort_order IS NULL), sei.sort_order, (sei.anilist_start_date IS NULL), sei.anilist_start_date, sei.external_id`, id)
	if err != nil {
		return nil, fmt.Errorf("get season anilist parts: %w", err)
	}
	for pr.Next() {
		var sid int64
		var p model.AniListPart
		if err := pr.Scan(&sid, &p.ExternalID, &p.Score, &p.EpisodeCount, &p.StartDate, &p.SortOrder); err != nil {
			pr.Close()
			return nil, fmt.Errorf("scan season anilist part: %w", err)
		}
		parts[sid] = append(parts[sid], p)
	}
	if err := pr.Err(); err != nil {
		pr.Close()
		return nil, fmt.Errorf("iterate season anilist parts: %w", err)
	}
	pr.Close()
	for i := range title.Seasons {
		ps := parts[title.Seasons[i].ID]
		title.Seasons[i].AniListParts = ps
		if len(ps) > 0 {
			title.Seasons[i].AniListID = &ps[0].ExternalID
			title.Seasons[i].AniListAverageScore = ps[0].Score
		}
	}

	// Load all episodes in one query (seasons cursor is closed above; safe with MaxOpenConns=1)
	epRows, err := r.db.Query(`
		SELECT e.id, e.season_id, e.episode, e.name, e.air_date, e.watched, e.first_watched_at, e.last_watched_at, e.external_source_id
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
		if err := epRows.Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.FirstWatchedAt, &e.LastWatchedAt, &e.ExternalSourceID); err != nil {
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

	// Load franchise, side story, and saga relations
	relRepo := NewTitleRelationRepository(r.db)
	if rels, err := relRepo.GetByTitleID(context.Background(), id); err == nil {
		title.Relations = rels
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
	// HasSyncedSeasons is true when at least one of the title's seasons carries a
	// total_episodes value — the marker that its episode list was fetched from
	// TMDB. False means the list was never synced (Simkl-imported or scrobble-only),
	// which lets the refresh backfill it even for completed/dropped titles.
	HasSyncedSeasons bool
}

// titleLiteCols is the column list for TitleLite scans, embedding
// displayNameExpr (which assumes the outer titles row is aliased `t`).
const titleLiteCols = `t.id, t.type, t.is_anime, t.status, t.series_status, t.cover_url,
		t.tmdb_id, t.tvdb_id, t.anilist_id,
		COALESCE(` + displayNameExpr + `, '') AS name,
		EXISTS(SELECT 1 FROM seasons s WHERE s.title_id = t.id AND s.total_episodes IS NOT NULL) AS has_synced_seasons`

func scanTitleLite(scanner interface {
	Scan(dest ...any) error
}, t *TitleLite) error {
	return scanner.Scan(
		&t.ID, &t.Type, &t.IsAnime, &t.Status, &t.SeriesStatus, &t.CoverURL,
		&t.TMDBID, &t.TVDBID, &t.AniListID,
		&t.PrimaryName,
		&t.HasSyncedSeasons,
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
	rows, err := r.db.Query(`SELECT id, type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, external_source_id, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, total_watch_minutes, tmdb_rating, credits, anilist_rating, release_date, next_air_date, next_air_episode, last_watched_at, last_refreshed_at, accent_hex, simkl_id, simkl_slug, radarr_id, sonarr_id, arr_ignored, created_at, updated_at FROM titles ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all titles: %w", err)
	}

	var titles []model.Title
	for rows.Next() {
		var t model.Title
		var lastWatchedAtStr, lastRefreshedAtStr *string
		if err := rows.Scan(&t.ID, &t.Type, &t.IsAnime, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
			&t.ExternalSourceID, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
			&t.Overview, &t.Runtime, &t.TotalWatchMinutes, &t.TMDBRating, &t.Credits, &t.AniListRating,
			&t.ReleaseDate, &t.NextAirDate, &t.NextAirEpisode, &lastWatchedAtStr, &lastRefreshedAtStr, &t.AccentHex, &t.SimklID, &t.SimklSlug, &t.RadarrID, &t.SonarrID, &t.ArrIgnored, &t.CreatedAt, &t.UpdatedAt); err != nil {
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

// FindByExternalID looks up a title by external IDs (IMDB, TMDB, AniList, TVDB, external source ID).
// If titleType is non-nil, results are filtered by type (useful because TMDB IDs
// are only unique within a media type).
func (r *TitleRepository) FindByExternalID(imdbID *string, tmdbID *int64, externalSourceID *string, anilistID *int64, tvdbID *int64, titleType *model.TitleType) (*model.Title, error) {
	var conditions []string
	var args []any

	if imdbID != nil && *imdbID != "" {
		conditions = append(conditions, `imdb_id = ?`)
		args = append(args, *imdbID)
	}
	if tmdbID != nil && *tmdbID != 0 {
		conditions = append(conditions, `tmdb_id = ?`)
		args = append(args, *tmdbID)
	}
	if externalSourceID != nil && *externalSourceID != "" {
		conditions = append(conditions, `external_source_id = ?`)
		args = append(args, *externalSourceID)
	}
	if anilistID != nil && *anilistID != 0 {
		conditions = append(conditions, `anilist_id = ?`)
		args = append(args, *anilistID)
	}
	if tvdbID != nil && *tvdbID != 0 {
		conditions = append(conditions, `tvdb_id = ?`)
		args = append(args, *tvdbID)
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

// FindByNameAndType looks up an existing title by exact (case-insensitive) name
// match across all its title_names, filtered by title type. If year > 0, exact
// year matches are prioritized over other years.
func (r *TitleRepository) FindByNameAndType(name string, titleType model.TitleType, year int) (*model.Title, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, sql.ErrNoRows
	}

	query := `SELECT t.id FROM titles t
JOIN title_names tn ON tn.title_id = t.id
WHERE tn.name = ? COLLATE NOCASE AND t.type = ?
ORDER BY (CASE WHEN t.year = ? THEN 0 ELSE 1 END), t.id ASC
LIMIT 1`

	var id int64
	if err := r.db.QueryRow(query, name, titleType, year).Scan(&id); err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

// parseWatchProviders decodes the titles.watch_providers JSON column. A NULL
// column, empty string, or malformed JSON yields an empty slice (never an error):
// a bad availability blob must never fail a title read.
func parseWatchProviders(raw *string) []model.WatchProvider {
	if raw == nil || *raw == "" {
		return nil
	}
	var providers []model.WatchProvider
	if err := json.Unmarshal([]byte(*raw), &providers); err != nil {
		return nil
	}
	return providers
}

// parseSQLiteTimeVal parses a SQLite datetime string into a time.Time.
// SQLite stores datetimes in UTC without a timezone marker; time.Parse returns
// UTC when no timezone is embedded, which is the expected behaviour here.
// Accepted formats: "2006-01-02 15:04:05" (SQLite default) or RFC3339.
// Returns the zero time.Time if no format matches.
func parseSQLiteTimeVal(s string) time.Time {
	// Try standard SQLite datetime format
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err == nil {
		return t
	}
	// Try RFC3339 (ISO)
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return t
	}
	return time.Time{}
}

// parseSQLiteTime parses a nullable SQLite datetime string into a *time.Time.
func parseSQLiteTime(s *string) *time.Time {
	if s == nil {
		return nil
	}
	t := parseSQLiteTimeVal(*s)
	if t.IsZero() {
		return nil
	}
	return &t
}

// HasUnwatchedEpisodes returns true if the title has at least one unwatched episode.
func (r *TitleRepository) HasUnwatchedEpisodes(titleID int64) (bool, error) {
	query := `
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

// HasWatchedEpisodes returns true if the title has at least one watched episode.
func (r *TitleRepository) HasWatchedEpisodes(titleID int64) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM episodes e
			JOIN seasons s ON e.season_id = s.id
			WHERE s.title_id = ? AND e.watched = 1
		)`
	var exists bool
	if err := r.db.QueryRow(query, titleID).Scan(&exists); err != nil {
		return false, fmt.Errorf("has watched episodes: %w", err)
	}
	return exists, nil
}

// GetUsedCoversInBatch returns a map containing the subset of filenames that are currently in use.
func (r *TitleRepository) GetUsedCoversInBatch(filenames []string) (map[string]bool, error) {
	if len(filenames) == 0 {
		return make(map[string]bool), nil
	}

	placeholders := make([]string, len(filenames))
	args := make([]any, len(filenames))
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

// ListContinueWatching returns Watching titles that have at least one aired unwatched episode,
// ordered by last_watched_at DESC. Display name priority: fr → en → (x-romaji → ja when anime) → any.
func (r *TitleRepository) ListContinueWatching() ([]ContinueWatchingItem, error) {
	query := `
		SELECT t.id, t.type, t.is_anime, t.cover_url,
		       COALESCE(` + displayNameExpr + `, '') AS name,
		       t.next_air_episode,
		       (SELECT COUNT(*) FROM episodes e JOIN seasons s ON e.season_id = s.id WHERE s.title_id = t.id AND e.watched = 1) AS watched_episodes,
		       (SELECT COUNT(*) FROM episodes e JOIN seasons s ON e.season_id = s.id WHERE s.title_id = t.id) AS total_episodes,
		       t.last_watched_at,
		       t.watch_providers,
		       t.sonarr_id,
		       t.radarr_id
		FROM titles t
		WHERE t.status = 'watching'
		  AND EXISTS (SELECT 1 FROM episodes e JOIN seasons s ON e.season_id = s.id WHERE s.title_id = t.id AND e.watched = 0 AND ` + airedEpisode + `)
		ORDER BY t.last_watched_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list continue watching: %w", err)
	}
	defer rows.Close()

	var items []ContinueWatchingItem
	for rows.Next() {
		var item ContinueWatchingItem
		var watchProvidersRaw *string
		if err := rows.Scan(&item.ID, &item.Type, &item.IsAnime, &item.CoverURL, &item.Name, &item.NextAirEpisode,
			&item.WatchedEpisodes, &item.TotalEpisodes, &item.LastWatchedAt, &watchProvidersRaw,
			&item.SonarrID, &item.RadarrID); err != nil {
			return nil, fmt.Errorf("scan continue watching: %w", err)
		}
		item.WatchProviders = parseWatchProviders(watchProvidersRaw)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate continue watching: %w", err)
	}

	if len(items) > 0 {
		args := make([]any, len(items))
		placeholders := make([]string, len(items))
		for i, it := range items {
			args[i] = it.ID
			placeholders[i] = "?"
		}
		inClause := strings.Join(placeholders, ",")
		nextEpRows, err := r.db.Query(`
			SELECT title_id, ep_id, season_id, episode_number, season_number
			FROM (
				SELECT s.title_id, e.id AS ep_id, e.season_id, e.episode AS episode_number, s.season_number,
					   ROW_NUMBER() OVER (PARTITION BY s.title_id ORDER BY s.season_number, e.episode) as rn
				FROM episodes e
				JOIN seasons s ON s.id = e.season_id
				WHERE s.title_id IN (`+inClause+`) AND e.watched = 0
			)
			WHERE rn = 1`, args...)
		if err == nil {
			defer nextEpRows.Close()
			itemMap := make(map[int64]*ContinueWatchingItem, len(items))
			for i := range items {
				itemMap[items[i].ID] = &items[i]
			}
			for nextEpRows.Next() {
				var titleID int64
				var ne model.NextEpisode
				if err := nextEpRows.Scan(&titleID, &ne.ID, &ne.SeasonID, &ne.Episode, &ne.SeasonNumber); err == nil {
					if it, ok := itemMap[titleID]; ok {
						it.NextEpisode = &ne
					}
				}
			}
		}
	}

	return items, nil
}

// ListUpcoming returns Watching and PlanToWatch titles with next_air_date >= today,
// ordered by next_air_date ASC.
func (r *TitleRepository) ListUpcoming(today string) ([]UpcomingItem, error) {
	query := `
		SELECT t.id, t.type, t.is_anime, t.cover_url,
		       COALESCE(` + displayNameExpr + `, '') AS name,
		       t.next_air_date, t.next_air_episode, t.status,
		       t.watch_providers,
		       t.sonarr_id,
		       t.radarr_id
		FROM titles t
		WHERE (t.status IN ('watching', 'plan_to_watch') OR (t.status = 'completed' AND t.series_status = 'returning'))
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
		var watchProvidersRaw *string
		if err := rows.Scan(&item.ID, &item.Type, &item.IsAnime, &item.CoverURL, &item.Name,
			&item.NextAirDate, &item.NextAirEpisode, &item.Status, &watchProvidersRaw,
			&item.SonarrID, &item.RadarrID); err != nil {
			return nil, fmt.Errorf("scan upcoming: %w", err)
		}
		item.WatchProviders = parseWatchProviders(watchProvidersRaw)
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

// ListCalendarEvents returns calendar events (unwatched episodes and title airings/releases)
// within the [from, to] date range for tracked titles (watching, plan_to_watch, or completed returning).
func (r *TitleRepository) ListCalendarEvents(from, to string) ([]CalendarEventItem, error) {
	if from == "" {
		from = "1970-01-01"
	}
	if to == "" {
		to = "2099-12-31"
	}

	var items []CalendarEventItem
	seen := make(map[string]bool)

	// 1. Query unwatched episodes with air_date in [from, to]
	epQuery := `
		SELECT e.id, e.episode, e.name, e.air_date, s.season_number,
		       t.id, t.type, t.is_anime, t.cover_url,
		       COALESCE(` + displayNameExpr + `, '') AS title_name,
		       t.status, t.watch_providers, t.accent_hex, t.overview,
		       t.sonarr_id, t.radarr_id
		FROM episodes e
		JOIN seasons s ON s.id = e.season_id
		JOIN titles t ON t.id = s.title_id
		WHERE (t.status IN ('watching', 'plan_to_watch') OR (t.status = 'completed' AND t.series_status = 'returning'))
		  AND e.air_date IS NOT NULL AND e.air_date != ''
		  AND e.air_date >= ? AND e.air_date <= ?
		  AND e.watched = 0
		ORDER BY e.air_date ASC, s.season_number ASC, e.episode ASC`

	epRows, err := r.db.Query(epQuery, from, to)
	if err != nil {
		return nil, fmt.Errorf("list calendar episodes: %w", err)
	}
	defer epRows.Close()

	for epRows.Next() {
		var epID int64
		var epNum int
		var epName *string
		var airDate string
		var seasonNum int
		var titleID int64
		var tType string
		var isAnime bool
		var coverURL *string
		var titleName string
		var status string
		var watchProvidersRaw *string
		var accentHex *string
		var overview *string
		var sonarrID *int64
		var radarrID *int64

		if err := epRows.Scan(&epID, &epNum, &epName, &airDate, &seasonNum,
			&titleID, &tType, &isAnime, &coverURL,
			&titleName, &status, &watchProvidersRaw, &accentHex, &overview,
			&sonarrID, &radarrID); err != nil {
			return nil, fmt.Errorf("scan calendar episode: %w", err)
		}

		item := CalendarEventItem{
			ID:             fmt.Sprintf("ep-%d", epID),
			TitleID:        titleID,
			TitleName:      titleName,
			Type:           tType,
			IsAnime:        isAnime,
			CoverURL:       coverURL,
			AirDate:        airDate,
			EpisodeID:      &epID,
			SeasonNumber:   &seasonNum,
			EpisodeNumber:  &epNum,
			EpisodeName:    epName,
			Status:         status,
			WatchProviders: parseWatchProviders(watchProvidersRaw),
			AccentHex:      accentHex,
			Overview:       overview,
			SonarrID:       sonarrID,
			RadarrID:       radarrID,
		}
		items = append(items, item)
		seen[fmt.Sprintf("%d-%s", titleID, airDate)] = true
	}
	if err := epRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate calendar episodes: %w", err)
	}

	// 2. Query title-level air dates (movies or series next_air_date without duplicate episode)
	titleQuery := `
		SELECT t.id, t.type, t.is_anime, t.cover_url,
		       COALESCE(` + displayNameExpr + `, '') AS title_name,
		       COALESCE(t.next_air_date, CASE WHEN t.type = 'movie' THEN t.release_date ELSE NULL END) AS air_date,
		       t.next_air_episode, t.status, t.watch_providers, t.accent_hex, t.overview,
		       t.sonarr_id, t.radarr_id
		FROM titles t
		WHERE (t.status IN ('watching', 'plan_to_watch') OR (t.status = 'completed' AND t.series_status = 'returning'))
		  AND (
		    (t.next_air_date IS NOT NULL AND t.next_air_date >= ? AND t.next_air_date <= ?)
		    OR (t.type = 'movie' AND t.release_date IS NOT NULL AND t.release_date >= ? AND t.release_date <= ?)
		  )
		ORDER BY air_date ASC`

	titleRows, err := r.db.Query(titleQuery, from, to, from, to)
	if err != nil {
		return nil, fmt.Errorf("list calendar titles: %w", err)
	}
	defer titleRows.Close()

	for titleRows.Next() {
		var titleID int64
		var tType string
		var isAnime bool
		var coverURL *string
		var titleName string
		var airDate string
		var nextAirEp *string
		var status string
		var watchProvidersRaw *string
		var accentHex *string
		var overview *string
		var sonarrID *int64
		var radarrID *int64

		if err := titleRows.Scan(&titleID, &tType, &isAnime, &coverURL,
			&titleName, &airDate, &nextAirEp, &status, &watchProvidersRaw,
			&accentHex, &overview, &sonarrID, &radarrID); err != nil {
			return nil, fmt.Errorf("scan calendar title: %w", err)
		}

		key := fmt.Sprintf("%d-%s", titleID, airDate)
		if seen[key] {
			continue
		}

		item := CalendarEventItem{
			ID:             fmt.Sprintf("title-%d-%s", titleID, airDate),
			TitleID:        titleID,
			TitleName:      titleName,
			Type:           tType,
			IsAnime:        isAnime,
			CoverURL:       coverURL,
			AirDate:        airDate,
			NextAirEpisode: nextAirEp,
			Status:         status,
			WatchProviders: parseWatchProviders(watchProvidersRaw),
			AccentHex:      accentHex,
			Overview:       overview,
			SonarrID:       sonarrID,
			RadarrID:       radarrID,
		}
		items = append(items, item)
		seen[key] = true
	}
	if err := titleRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate calendar titles: %w", err)
	}

	// Sort chronologically
	sort.Slice(items, func(i, j int) bool {
		if items[i].AirDate != items[j].AirDate {
			return items[i].AirDate < items[j].AirDate
		}
		if items[i].TitleName != items[j].TitleName {
			return items[i].TitleName < items[j].TitleName
		}
		s1, s2 := 0, 0
		if items[i].SeasonNumber != nil {
			s1 = *items[i].SeasonNumber
		}
		if items[j].SeasonNumber != nil {
			s2 = *items[j].SeasonNumber
		}
		if s1 != s2 {
			return s1 < s2
		}
		e1, e2 := 0, 0
		if items[i].EpisodeNumber != nil {
			e1 = *items[i].EpisodeNumber
		}
		if items[j].EpisodeNumber != nil {
			e2 = *items[j].EpisodeNumber
		}
		return e1 < e2
	})

	if items == nil {
		items = []CalendarEventItem{}
	}
	return items, nil
}
