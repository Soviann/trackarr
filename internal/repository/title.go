package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
)

type TitleRepository struct {
	db database.DBTX
}

func NewTitleRepository(db database.DBTX) *TitleRepository {
	return &TitleRepository{db: db}
}

func (r *TitleRepository) DB() database.DBTX {
	return r.db
}

type TitleFilter struct {
	Status           *model.TitleStatus
	Type             *model.TitleType
	IsAnime          *bool
	Search           *string
	MatchStatus      *model.MatchStatus
	SeriesStatus     *model.SeriesStatus
	UpToDate         bool // server-side "up to date" filter (watching + all episodes watched)
	WatchingBehind   bool // server-side "watching but behind" filter (watching + has unwatched episodes)
	Limit            int
	Offset           int
	Sort             string   // column name: updated_at, original_title, year, my_rating, created_at, release_date, last_watched_at
	Order            string   // asc or desc
	Decade           *int     // e.g. 2020 → year BETWEEN 2020 AND 2029
	ReleaseFrom      *string  // YYYY-MM-DD, filters on release_date >=
	ReleaseTo        *string  // YYYY-MM-DD, filters on release_date <=
	IncludeNoRelease bool     // when false + date filter active, exclude NULL release_date
	Genres           []string // filter by these genres
	GenreOp          string   // "AND" | "OR", defaults to "OR"
}

const DefaultPageSize = 50

// PaginatedResult wraps a list of titles with pagination metadata.
type PaginatedResult struct {
	Titles  []model.Title `json:"titles"`
	Total   int           `json:"total"`
	HasMore bool          `json:"has_more"`
	Counts  *StatusCounts `json:"counts,omitempty"`
}

// StatusCounts holds global counts for the library overview.
type StatusCounts struct {
	PendingReview int `json:"pending_review"`
	Unconfirmed   int `json:"unconfirmed"`
}

type TitleUpdate struct {
	Status        *model.TitleStatus
	MatchStatus   *model.MatchStatus
	MyRating      *int
	SeriesStatus  *model.SeriesStatus
	CoverURL      *string
	IMDBID        *string
	AniListID     *int64
	TMDBID        *int64
	TVDBID        *int64
	PlexRatingKey *string
	MatchSource   *string
	OriginalTitle *string
	Type          *model.TitleType
	IsAnime       *bool
	Overview      *string
	Runtime       *int
	TMDBRating    *float64
	Credits       *string
	AniListRating *int
	ReleaseDate   *string
}

func (r *TitleRepository) Create(title *model.Title, names []model.TitleName) (int64, error) {
	// If already inside a transaction, use the DBTX directly.
	// Otherwise, wrap in a new transaction for atomicity.
	if db, ok := r.db.(*sql.DB); ok {
		var id int64
		err := database.WithTx(db, func(tx *sql.Tx) error {
			var err error
			id, err = r.createInTx(tx, title, names)
			return err
		})
		return id, err
	}
	return r.createInTx(r.db, title, names)
}

func (r *TitleRepository) createInTx(db database.DBTX, title *model.Title, names []model.TitleName) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO titles (type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, tmdb_rating, credits, anilist_rating, release_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		title.Type, title.IsAnime, title.Year, title.CoverURL, title.IMDBID, title.AniListID, title.TMDBID, title.TVDBID,
		title.PlexRatingKey, title.MyRating, title.Status, title.SeriesStatus, title.MatchStatus,
		title.OriginalTitle, title.MatchSource,
		title.Overview, title.Runtime, title.TMDBRating, title.Credits, title.AniListRating,
		title.ReleaseDate,
	)
	if err != nil {
		return 0, fmt.Errorf("insert title: %w", err)
	}

	id, _ := res.LastInsertId()

	for _, name := range names {
		_, err := db.Exec(`INSERT INTO title_names (title_id, name, language, is_primary) VALUES (?, ?, ?, ?)`,
			id, name.Name, name.Language, name.IsPrimary)
		if err != nil {
			return 0, fmt.Errorf("insert title name: %w", err)
		}
	}

	return id, nil
}

func (r *TitleRepository) GetByID(id int64) (*model.Title, error) {
	title := &model.Title{}
	var lastWatchedAtStr *string
	err := r.db.QueryRow(`SELECT id, type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, tmdb_rating, credits, anilist_rating, release_date, last_watched_at, created_at, updated_at FROM titles WHERE id = ?`, id).
		Scan(&title.ID, &title.Type, &title.IsAnime, &title.Year, &title.CoverURL, &title.IMDBID, &title.AniListID, &title.TMDBID, &title.TVDBID,
			&title.PlexRatingKey, &title.MyRating, &title.Status, &title.SeriesStatus, &title.MatchStatus, &title.OriginalTitle, &title.MatchSource,
			&title.Overview, &title.Runtime, &title.TMDBRating, &title.Credits, &title.AniListRating,
			&title.ReleaseDate, &lastWatchedAtStr, &title.CreatedAt, &title.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get title: %w", err)
	}
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
	seasonRows, err := r.db.Query(`SELECT id, title_id, season_number, total_episodes, my_rating FROM seasons WHERE title_id = ? ORDER BY season_number`, id)
	if err != nil {
		return nil, fmt.Errorf("get seasons: %w", err)
	}

	for seasonRows.Next() {
		var s model.Season
		if err := seasonRows.Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes, &s.MyRating); err != nil {
			seasonRows.Close()
			return nil, fmt.Errorf("scan season: %w", err)
		}
		s.Episodes = []model.Episode{}
		title.Seasons = append(title.Seasons, s)
	}
	seasonRows.Close()

	// Load episodes for each season (cursor closed above to avoid deadlock with MaxOpenConns=1)
	for i := range title.Seasons {
		epRows, err := r.db.Query(`SELECT id, season_id, episode, name, air_date, watched, watched_at, plex_rating_key FROM episodes WHERE season_id = ? ORDER BY episode`, title.Seasons[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get episodes: %w", err)
		}
		for epRows.Next() {
			var e model.Episode
			if err := epRows.Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.WatchedAt, &e.PlexRatingKey); err != nil {
				epRows.Close()
				return nil, fmt.Errorf("scan episode: %w", err)
			}
			title.Seasons[i].Episodes = append(title.Seasons[i].Episodes, e)
		}
		epRows.Close()
	}

	return title, nil
}

func (r *TitleRepository) List(filter TitleFilter) (*PaginatedResult, error) {
	searchTerm := ""
	if filter.Search != nil {
		searchTerm = strings.TrimSpace(*filter.Search)
	}

	// Delegate search to relevance-ranked search
	if searchTerm != "" {
		return r.searchTitlesPaginated(searchTerm, filter)
	}

	baseCols := `t.id, t.type, t.is_anime, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.original_title, t.match_source, t.overview, t.runtime, t.tmdb_rating, t.credits, t.anilist_rating, t.release_date, t.last_watched_at, t.created_at, t.updated_at`

	var conditions []string
	var args []interface{}

	switch {
	case filter.UpToDate:
		// "Up to date" = watching + every episode watched (no unwatched episodes)
		conditions = append(conditions, `t.status = 'watching'`)
		conditions = append(conditions, `(t.type != 'movie' OR t.is_anime = 1)`)
		conditions = append(conditions, `EXISTS (SELECT 1 FROM seasons s2 JOIN episodes e2 ON e2.season_id = s2.id WHERE s2.title_id = t.id)`)
		conditions = append(conditions, `NOT EXISTS (SELECT 1 FROM seasons s3 JOIN episodes e3 ON e3.season_id = s3.id WHERE s3.title_id = t.id AND e3.watched = 0)`)
	case filter.WatchingBehind:
		// "Watching behind" = watching + has at least one unwatched episode (or is a movie, or has no episodes)
		conditions = append(conditions, `t.status = 'watching'`)
		conditions = append(conditions, `((t.type = 'movie' AND t.is_anime = 0) OR NOT EXISTS (SELECT 1 FROM seasons s2 JOIN episodes e2 ON e2.season_id = s2.id WHERE s2.title_id = t.id) OR EXISTS (SELECT 1 FROM seasons s4 JOIN episodes e4 ON e4.season_id = s4.id WHERE s4.title_id = t.id AND e4.watched = 0))`)
	default:
		if filter.Status != nil {
			conditions = append(conditions, `t.status = ?`)
			args = append(args, *filter.Status)
		}
	}
	if filter.Type != nil {
		conditions = append(conditions, `t.type = ?`)
		args = append(args, *filter.Type)
	}
	if filter.MatchStatus != nil {
		conditions = append(conditions, `t.match_status = ?`)
		args = append(args, *filter.MatchStatus)
	}
	if filter.SeriesStatus != nil {
		conditions = append(conditions, `t.series_status = ?`)
		args = append(args, *filter.SeriesStatus)
	}
	if filter.Decade != nil {
		conditions = append(conditions, `t.year BETWEEN ? AND ?`)
		args = append(args, *filter.Decade, *filter.Decade+9)
	}
	if filter.ReleaseFrom != nil {
		if filter.IncludeNoRelease {
			conditions = append(conditions, `(t.release_date >= ? OR t.release_date IS NULL)`)
		} else {
			conditions = append(conditions, `t.release_date >= ?`)
		}
		args = append(args, *filter.ReleaseFrom)
	}
	if filter.ReleaseTo != nil {
		if filter.IncludeNoRelease {
			conditions = append(conditions, `(t.release_date <= ? OR t.release_date IS NULL)`)
		} else {
			conditions = append(conditions, `t.release_date <= ?`)
		}
		args = append(args, *filter.ReleaseTo)
	}
	if !filter.IncludeNoRelease && (filter.ReleaseFrom != nil || filter.ReleaseTo != nil) {
		conditions = append(conditions, `t.release_date IS NOT NULL`)
	}
	if filter.IsAnime != nil {
		conditions = append(conditions, `t.is_anime = ?`)
		if *filter.IsAnime {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if len(filter.Genres) > 0 {
		op := filter.GenreOp
		if op != "AND" {
			op = "OR"
		}
		if op == "OR" {
			placeholders := make([]string, len(filter.Genres))
			for i, g := range filter.Genres {
				placeholders[i] = "?"
				args = append(args, g)
			}
			conditions = append(conditions, `EXISTS (SELECT 1 FROM title_genres tg WHERE tg.title_id = t.id AND tg.genre IN (`+strings.Join(placeholders, ",")+`))`)
		} else { // AND
			for _, g := range filter.Genres {
				conditions = append(conditions, `EXISTS (SELECT 1 FROM title_genres tg WHERE tg.title_id = t.id AND tg.genre = ?)`)
				args = append(args, g)
			}
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = ` WHERE ` + strings.Join(conditions, ` AND `)
	}

	// Count total
	var total int
	countQuery := `SELECT COUNT(DISTINCT t.id) FROM titles t` + whereClause
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count titles: %w", err)
	}

	// Apply pagination
	limit := filter.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	offset := filter.Offset

	// Build ORDER BY
	orderBy := "t.updated_at DESC" // default
	if filter.Sort != "" {
		dir := "DESC"
		if filter.Order == "asc" {
			dir = "ASC"
		} else if filter.Order == "desc" {
			dir = "DESC"
		}
		col := "t." + filter.Sort
		// NULLS LAST: for nullable columns, sort nulls to the end
		switch filter.Sort {
		case "my_rating", "year", "original_title", "release_date", "last_watched_at":
			orderBy = fmt.Sprintf("CASE WHEN %s IS NULL THEN 1 ELSE 0 END, %s %s", col, col, dir)
		default:
			orderBy = fmt.Sprintf("%s %s", col, dir)
		}
	}

	query := `SELECT ` + baseCols + ` FROM titles t` + whereClause + ` ORDER BY ` + orderBy + ` LIMIT ? OFFSET ?`
	queryArgs := make([]interface{}, len(args), len(args)+2)
	copy(queryArgs, args)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := r.db.Query(query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list titles: %w", err)
	}

	var titles []model.Title
	for rows.Next() {
		var t model.Title
		var lastWatchedAtStr *string
		if err := rows.Scan(&t.ID, &t.Type, &t.IsAnime, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
			&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
			&t.Overview, &t.Runtime, &t.TMDBRating, &t.Credits, &t.AniListRating,
			&t.ReleaseDate, &lastWatchedAtStr, &t.CreatedAt, &t.UpdatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan title: %w", err)
		}
		t.LastWatchedAt = parseSQLiteTime(lastWatchedAtStr)
		titles = append(titles, t)
	}
	rows.Close()

	titles, err = r.loadTitleRelationsLight(titles)
	if err != nil {
		return nil, err
	}

	return &PaginatedResult{
		Titles:  titles,
		Total:   total,
		HasMore: offset+len(titles) < total,
	}, nil
}

// loadTitleRelations loads names, seasons, and episodes for a slice of titles.
func (r *TitleRepository) loadTitleRelations(titles []model.Title) ([]model.Title, error) {
	if len(titles) == 0 {
		return titles, nil
	}

	ids := make([]int64, len(titles))
	titleMap := make(map[int64]*model.Title)
	placeholders := make([]string, len(titles))
	args := make([]interface{}, len(titles))

	for i := range titles {
		ids[i] = titles[i].ID
		titleMap[ids[i]] = &titles[i]
		placeholders[i] = "?"
		args[i] = ids[i]
	}

	inClause := strings.Join(placeholders, ",")

	// 1. Bulk load names
	nameRows, err := r.db.Query(`SELECT id, title_id, name, language, is_primary FROM title_names WHERE title_id IN (`+inClause+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("get title names bulk: %w", err)
	}
	for nameRows.Next() {
		var n model.TitleName
		if err := nameRows.Scan(&n.ID, &n.TitleID, &n.Name, &n.Language, &n.IsPrimary); err != nil {
			nameRows.Close()
			return nil, fmt.Errorf("scan title name: %w", err)
		}
		if t, ok := titleMap[n.TitleID]; ok {
			t.Names = append(t.Names, n)
		}
	}
	nameRows.Close()

	// 2. Bulk load seasons
	seasonRows, err := r.db.Query(`SELECT id, title_id, season_number, total_episodes, my_rating FROM seasons WHERE title_id IN (`+inClause+`) ORDER BY title_id, season_number`, args...)
	if err != nil {
		return nil, fmt.Errorf("get seasons bulk: %w", err)
	}

	seasonMap := make(map[int64]*model.Season)
	var seasonIDs []int64
	var seasonPlaceholders []string
	var seasonArgs []interface{}

	for seasonRows.Next() {
		var s model.Season
		if err := seasonRows.Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes, &s.MyRating); err != nil {
			seasonRows.Close()
			return nil, fmt.Errorf("scan season: %w", err)
		}
		s.Episodes = []model.Episode{}
		if t, ok := titleMap[s.TitleID]; ok {
			t.Seasons = append(t.Seasons, s)
			// Get reference to the season in the title slice to add episodes later
			newSeasonRef := &t.Seasons[len(t.Seasons)-1]
			seasonMap[s.ID] = newSeasonRef
			seasonIDs = append(seasonIDs, s.ID)
			seasonPlaceholders = append(seasonPlaceholders, "?")
			seasonArgs = append(seasonArgs, s.ID)
		}
	}
	seasonRows.Close()

	// 3. Bulk load episodes
	if len(seasonIDs) > 0 {
		epInClause := strings.Join(seasonPlaceholders, ",")
		epRows, err := r.db.Query(`SELECT id, season_id, episode, name, air_date, watched, watched_at, plex_rating_key FROM episodes WHERE season_id IN (`+epInClause+`) ORDER BY season_id, episode`, seasonArgs...)
		if err != nil {
			return nil, fmt.Errorf("get episodes bulk: %w", err)
		}
		for epRows.Next() {
			var e model.Episode
			if err := epRows.Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.WatchedAt, &e.PlexRatingKey); err != nil {
				epRows.Close()
				return nil, fmt.Errorf("scan episode: %w", err)
			}
			if s, ok := seasonMap[e.SeasonID]; ok {
				s.Episodes = append(s.Episodes, e)
			}
		}
		epRows.Close()
	}

	// 4. Bulk load genres from title_genres
	genreRows, err := r.db.Query(`SELECT title_id, genre FROM title_genres WHERE title_id IN (`+inClause+`) ORDER BY title_id, genre`, args...)
	if err == nil {
		for genreRows.Next() {
			var titleID int64
			var genre string
			if err := genreRows.Scan(&titleID, &genre); err == nil {
				if t, ok := titleMap[titleID]; ok {
					t.Genres = append(t.Genres, genre)
				}
			}
		}
		genreRows.Close()
	}

	return titles, nil
}

// GetStatusCounts returns global match status counts for the library banner.
func (r *TitleRepository) GetStatusCounts() (*StatusCounts, error) {
	var counts StatusCounts
	err := r.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN match_status = 'pending_review' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN match_status = 'unconfirmed' THEN 1 ELSE 0 END), 0)
		FROM titles`).Scan(&counts.PendingReview, &counts.Unconfirmed)
	if err != nil {
		return nil, fmt.Errorf("get status counts: %w", err)
	}
	return &counts, nil
}

// ListAll returns all titles with full relations (names, seasons, episodes). Used by background jobs.
func (r *TitleRepository) ListAll() ([]model.Title, error) {
	rows, err := r.db.Query(`SELECT id, type, is_anime, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, runtime, tmdb_rating, credits, anilist_rating, release_date, last_watched_at, created_at, updated_at FROM titles ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all titles: %w", err)
	}

	var titles []model.Title
	for rows.Next() {
		var t model.Title
		var lastWatchedAtStr *string
		if err := rows.Scan(&t.ID, &t.Type, &t.IsAnime, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
			&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
			&t.Overview, &t.Runtime, &t.TMDBRating, &t.Credits, &t.AniListRating,
			&t.ReleaseDate, &lastWatchedAtStr, &t.CreatedAt, &t.UpdatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan title: %w", err)
		}
		t.LastWatchedAt = parseSQLiteTime(lastWatchedAtStr)
		titles = append(titles, t)
	}
	rows.Close()

	return r.loadTitleRelations(titles)
}

// loadTitleRelationsLight loads names and seasons with watched/episode counts (no individual episodes).
// Used for listing endpoints where episode details are not needed.
func (r *TitleRepository) loadTitleRelationsLight(titles []model.Title) ([]model.Title, error) {
	if len(titles) == 0 {
		return titles, nil
	}

	ids := make([]int64, len(titles))
	titleMap := make(map[int64]*model.Title)
	placeholders := make([]string, len(titles))
	args := make([]interface{}, len(titles))

	for i := range titles {
		ids[i] = titles[i].ID
		titleMap[ids[i]] = &titles[i]
		placeholders[i] = "?"
		args[i] = ids[i]
	}

	inClause := strings.Join(placeholders, ",")

	// 1. Bulk load names
	nameRows, err := r.db.Query(`SELECT id, title_id, name, language, is_primary FROM title_names WHERE title_id IN (`+inClause+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("get title names bulk: %w", err)
	}
	for nameRows.Next() {
		var n model.TitleName
		if err := nameRows.Scan(&n.ID, &n.TitleID, &n.Name, &n.Language, &n.IsPrimary); err != nil {
			nameRows.Close()
			return nil, fmt.Errorf("scan title name: %w", err)
		}
		if t, ok := titleMap[n.TitleID]; ok {
			t.Names = append(t.Names, n)
		}
	}
	nameRows.Close()

	// 2. Bulk load seasons with counts
	seasonRows, err := r.db.Query(`
		SELECT s.id, s.title_id, s.season_number, s.total_episodes, s.my_rating,
			COUNT(e.id) AS episode_count,
			SUM(CASE WHEN e.watched THEN 1 ELSE 0 END) AS watched_count
		FROM seasons s
		LEFT JOIN episodes e ON e.season_id = s.id
		WHERE s.title_id IN (`+inClause+`)
		GROUP BY s.id
		ORDER BY s.title_id, s.season_number`, args...)
	if err != nil {
		return nil, fmt.Errorf("get seasons light bulk: %w", err)
	}
	for seasonRows.Next() {
		var s model.Season
		var episodeCount, watchedCount int
		if err := seasonRows.Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes, &s.MyRating, &episodeCount, &watchedCount); err != nil {
			seasonRows.Close()
			return nil, fmt.Errorf("scan season light: %w", err)
		}
		s.EpisodeCount = &episodeCount
		s.WatchedCount = &watchedCount
		s.Episodes = []model.Episode{}
		if t, ok := titleMap[s.TitleID]; ok {
			t.Seasons = append(t.Seasons, s)
		}
	}
	seasonRows.Close()

	// 3. Bulk load next unwatched episode using window function (SQLite 3.25+)
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
		for nextEpRows.Next() {
			var titleID int64
			var ne model.NextEpisode
			if err := nextEpRows.Scan(&titleID, &ne.ID, &ne.SeasonID, &ne.Episode, &ne.SeasonNumber); err == nil {
				if t, ok := titleMap[titleID]; ok {
					t.NextEpisode = &ne
				}
			}
		}
		nextEpRows.Close()
	}

	// 4. Bulk load genres from title_genres
	genreRows, err := r.db.Query(`SELECT title_id, genre FROM title_genres WHERE title_id IN (`+inClause+`) ORDER BY title_id, genre`, args...)
	if err == nil {
		for genreRows.Next() {
			var titleID int64
			var genre string
			if err := genreRows.Scan(&titleID, &genre); err == nil {
				if t, ok := titleMap[titleID]; ok {
					t.Genres = append(t.Genres, genre)
				}
			}
		}
		genreRows.Close()
	}

	return titles, nil
}

func (r *TitleRepository) Update(id int64, update TitleUpdate) error {
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

	if len(sets) == 0 {
		return nil
	}

	sets = append(sets, `updated_at = CURRENT_TIMESTAMP`)
	args = append(args, id)

	_, err := r.db.Exec(`UPDATE titles SET `+strings.Join(sets, `, `)+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update title: %w", err)
	}

	return nil
}

// UpdateLastWatchedAt sets the last_watched_at date for a title only if it is NULL
// or if the new date is more recent than the current one.
func (r *TitleRepository) UpdateLastWatchedAt(id int64, at time.Time) error {
	_, err := r.db.Exec(`UPDATE titles SET last_watched_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND (last_watched_at IS NULL OR ? > last_watched_at)`, at, id, at)
	if err != nil {
		return fmt.Errorf("update last watched at: %w", err)
	}
	return nil
}

// ReplaceNames deletes all existing names for a title and inserts new ones atomically.
func (r *TitleRepository) ReplaceNames(titleID int64, names []model.TitleName) error {
	doReplace := func(db database.DBTX) error {
		if _, err := db.Exec(`DELETE FROM title_names WHERE title_id = ?`, titleID); err != nil {
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
		if _, err := db.Exec(query, args...); err != nil {
			return fmt.Errorf("insert title names: %w", err)
		}
		return nil
	}

	if db, ok := r.db.(*sql.DB); ok {
		return database.WithTx(db, func(tx *sql.Tx) error {
			return doReplace(tx)
		})
	}
	return doReplace(r.db)
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

// Merge consolidates sourceID into destID.
// It moves seasons (shifting their number by seasonOffset), names, and watch events.
// The source title is deleted at the end.
func (r *TitleRepository) Merge(destID, sourceID int64, seasonOffset int) error {
	// If already inside a transaction, use the DBTX directly.
	if db, ok := r.db.(*sql.DB); ok {
		return database.WithTx(db, func(tx *sql.Tx) error {
			return r.mergeInTx(tx, destID, sourceID, seasonOffset)
		})
	}
	return r.mergeInTx(r.db, destID, sourceID, seasonOffset)
}

func (r *TitleRepository) mergeInTx(db database.DBTX, destID, sourceID int64, seasonOffset int) error {
	// 1. Move seasons. We must be careful about UNIQUE(title_id, season_number).
	// We increment season numbers by seasonOffset.
	rows, err := db.Query(`SELECT id, season_number FROM seasons WHERE title_id = ?`, sourceID)
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
		// If a season with this number already exists in dest, we might want to merge episodes
		// but for simplicity (and usually correct for anime splits), we just move it.
		// If it crashes on unique constraint, it means we have overlapping seasons.
		_, err := db.Exec(`UPDATE seasons SET title_id = ?, season_number = ? WHERE id = ?`, destID, m.newNum, m.id)
		if err != nil {
			return fmt.Errorf("move season %d: %w", m.id, err)
		}
	}

	// 2. Move names (as aliases, set is_primary=0)
	// We use INSERT OR IGNORE to avoid duplicates if the master already has this name.
	nameRows, err := db.Query(`SELECT name, language FROM title_names WHERE title_id = ?`, sourceID)
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
			_, _ = db.Exec(`INSERT OR IGNORE INTO title_names (title_id, name, language, is_primary) VALUES (?, ?, ?, 0)`, destID, nm.name, nm.lang)
		}
	}

	// 3. Move watch events
	_, err = db.Exec(`UPDATE watch_events SET title_id = ? WHERE title_id = ?`, destID, sourceID)
	if err != nil {
		return fmt.Errorf("move watch events: %w", err)
	}

	// 4. Delete source title (cascades should be handled by DB, but we already moved most things)
	_, err = db.Exec(`DELETE FROM titles WHERE id = ?`, sourceID)
	if err != nil {
		return fmt.Errorf("delete source title: %w", err)
	}

	return nil
}

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
