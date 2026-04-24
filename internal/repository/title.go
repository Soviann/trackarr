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
}

func (r *TitleRepository) GetByID(id int64) (*model.Title, error) {
	title := &model.Title{}
	var firstWatchedAtStr, lastWatchedAtStr *string
	err := r.db.QueryRow(`SELECT id, type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, total_watch_minutes, tmdb_rating, credits, anilist_rating, release_date, next_air_date, next_air_episode, first_watched_at, last_watched_at, created_at, updated_at FROM titles WHERE id = ?`, id).
		Scan(&title.ID, &title.Type, &title.IsAnime, &title.Year, &title.CoverURL, &title.IMDBID, &title.AniListID, &title.TMDBID, &title.TVDBID,
			&title.PlexRatingKey, &title.MyRating, &title.Status, &title.SeriesStatus, &title.MatchStatus, &title.OriginalTitle, &title.MatchSource,
			&title.Overview, &title.Runtime, &title.TotalWatchMinutes, &title.TMDBRating, &title.Credits, &title.AniListRating,
			&title.ReleaseDate, &title.NextAirDate, &title.NextAirEpisode, &firstWatchedAtStr, &lastWatchedAtStr, &title.CreatedAt, &title.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get title: %w", err)
	}
	title.FirstWatchedAt = parseSQLiteTime(firstWatchedAtStr)
	title.LastWatchedAt = parseSQLiteTime(lastWatchedAtStr)

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
	genreRows.Close()

	// Load seasons
	seasonRows, err := r.db.Query(`SELECT id, title_id, season_number, total_episodes FROM seasons WHERE title_id = ? ORDER BY season_number`, id)
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
	epRows.Close()
	for i := range title.Seasons {
		if eps, ok := grouped[title.Seasons[i].ID]; ok {
			title.Seasons[i].Episodes = eps
		}
	}

	return title, nil
}

// ListAll returns all titles with full relations (names, seasons, episodes). Used by background jobs.
func (r *TitleRepository) ListAll() ([]model.Title, error) {
	rows, err := r.db.Query(`SELECT id, type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, total_watch_minutes, tmdb_rating, credits, anilist_rating, release_date, next_air_date, next_air_episode, last_watched_at, created_at, updated_at FROM titles ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all titles: %w", err)
	}

	var titles []model.Title
	for rows.Next() {
		var t model.Title
		var lastWatchedAtStr *string
		if err := rows.Scan(&t.ID, &t.Type, &t.IsAnime, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
			&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
			&t.Overview, &t.Runtime, &t.TotalWatchMinutes, &t.TMDBRating, &t.Credits, &t.AniListRating,
			&t.ReleaseDate, &t.NextAirDate, &t.NextAirEpisode, &lastWatchedAtStr, &t.CreatedAt, &t.UpdatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan title: %w", err)
		}
		t.LastWatchedAt = parseSQLiteTime(lastWatchedAtStr)
		titles = append(titles, t)
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
	return items, nil
}

// ReviewCount returns the number of titles with match_status in ('pending_review', 'unconfirmed').
func (r *TitleRepository) ReviewCount(_ context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM titles WHERE match_status IN ('pending_review', 'unconfirmed')`).Scan(&count)
	return count, err
}
