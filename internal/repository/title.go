package repository

import (
	"database/sql"

	"github.com/nicolasvasse/plextracker/internal/database"
	"fmt"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/model"
)

type TitleRepository struct {
	db database.DBTX
}

func NewTitleRepository(db database.DBTX) *TitleRepository {
	return &TitleRepository{db: db}
}

type TitleFilter struct {
	Status      *model.TitleStatus
	Type        *model.TitleType
	Search      *string
	MatchStatus *model.MatchStatus
	UpToDate       bool // server-side "up to date" filter (watching + all episodes watched)
	WatchingBehind bool // server-side "watching but behind" filter (watching + has unwatched episodes)
	Limit          int
	Offset         int
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
	Status       *model.TitleStatus
	MatchStatus  *model.MatchStatus
	MyRating     *int
	SeriesStatus *model.SeriesStatus
	CoverURL     *string
	IMDBID       *string
	AniListID    *int64
	TMDBID       *int64
	TVDBID       *int64
	PlexRatingKey *string
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
		INSERT INTO titles (type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		title.Type, title.Year, title.CoverURL, title.IMDBID, title.AniListID, title.TMDBID, title.TVDBID,
		title.PlexRatingKey, title.MyRating, title.Status, title.SeriesStatus, title.MatchStatus,
		title.OriginalTitle, title.MatchSource,
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
	err := r.db.QueryRow(`SELECT id, type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, created_at, updated_at FROM titles WHERE id = ?`, id).
		Scan(&title.ID, &title.Type, &title.Year, &title.CoverURL, &title.IMDBID, &title.AniListID, &title.TMDBID, &title.TVDBID,
			&title.PlexRatingKey, &title.MyRating, &title.Status, &title.SeriesStatus, &title.MatchStatus, &title.OriginalTitle, &title.MatchSource, &title.CreatedAt, &title.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get title: %w", err)
	}

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

	baseCols := `t.id, t.type, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.original_title, t.match_source, t.created_at, t.updated_at`

	var conditions []string
	var args []interface{}

	switch {
	case filter.UpToDate:
		// "Up to date" = watching + every episode watched (no unwatched episodes)
		conditions = append(conditions, `t.status = 'watching'`)
		conditions = append(conditions, `t.type != 'movie'`)
		conditions = append(conditions, `EXISTS (SELECT 1 FROM seasons s2 JOIN episodes e2 ON e2.season_id = s2.id WHERE s2.title_id = t.id)`)
		conditions = append(conditions, `NOT EXISTS (SELECT 1 FROM seasons s3 JOIN episodes e3 ON e3.season_id = s3.id WHERE s3.title_id = t.id AND e3.watched = 0)`)
	case filter.WatchingBehind:
		// "Watching behind" = watching + has at least one unwatched episode (or is a movie, or has no episodes)
		conditions = append(conditions, `t.status = 'watching'`)
		conditions = append(conditions, `(t.type = 'movie' OR NOT EXISTS (SELECT 1 FROM seasons s2 JOIN episodes e2 ON e2.season_id = s2.id WHERE s2.title_id = t.id) OR EXISTS (SELECT 1 FROM seasons s4 JOIN episodes e4 ON e4.season_id = s4.id WHERE s4.title_id = t.id AND e4.watched = 0))`)
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

	query := `SELECT DISTINCT ` + baseCols + ` FROM titles t` + whereClause + ` ORDER BY t.updated_at DESC LIMIT ? OFFSET ?`
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
		if err := rows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
			&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource, &t.CreatedAt, &t.UpdatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan title: %w", err)
		}
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
	for i := range titles {
		nameRows, err := r.db.Query(`SELECT id, title_id, name, language, is_primary FROM title_names WHERE title_id = ?`, titles[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get title names: %w", err)
		}
		for nameRows.Next() {
			var n model.TitleName
			if err := nameRows.Scan(&n.ID, &n.TitleID, &n.Name, &n.Language, &n.IsPrimary); err != nil {
				nameRows.Close()
				return nil, fmt.Errorf("scan title name: %w", err)
			}
			titles[i].Names = append(titles[i].Names, n)
		}
		nameRows.Close()

		seasonRows, err := r.db.Query(`SELECT id, title_id, season_number, total_episodes, my_rating FROM seasons WHERE title_id = ? ORDER BY season_number`, titles[i].ID)
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
			titles[i].Seasons = append(titles[i].Seasons, s)
		}
		seasonRows.Close()

		for j := range titles[i].Seasons {
			epRows, err := r.db.Query(`SELECT id, season_id, episode, name, air_date, watched, watched_at, plex_rating_key FROM episodes WHERE season_id = ? ORDER BY episode`, titles[i].Seasons[j].ID)
			if err != nil {
				return nil, fmt.Errorf("get episodes: %w", err)
			}
			for epRows.Next() {
				var e model.Episode
				if err := epRows.Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.WatchedAt, &e.PlexRatingKey); err != nil {
					epRows.Close()
					return nil, fmt.Errorf("scan episode: %w", err)
				}
				titles[i].Seasons[j].Episodes = append(titles[i].Seasons[j].Episodes, e)
			}
			epRows.Close()
		}
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
	rows, err := r.db.Query(`SELECT id, type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, created_at, updated_at FROM titles ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all titles: %w", err)
	}

	var titles []model.Title
	for rows.Next() {
		var t model.Title
		if err := rows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
			&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource, &t.CreatedAt, &t.UpdatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan title: %w", err)
		}
		titles = append(titles, t)
	}
	rows.Close()

	return r.loadTitleRelations(titles)
}

// loadTitleRelationsLight loads names and seasons with watched/episode counts (no individual episodes).
// Used for listing endpoints where episode details are not needed.
func (r *TitleRepository) loadTitleRelationsLight(titles []model.Title) ([]model.Title, error) {
	for i := range titles {
		nameRows, err := r.db.Query(`SELECT id, title_id, name, language, is_primary FROM title_names WHERE title_id = ?`, titles[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get title names: %w", err)
		}
		for nameRows.Next() {
			var n model.TitleName
			if err := nameRows.Scan(&n.ID, &n.TitleID, &n.Name, &n.Language, &n.IsPrimary); err != nil {
				nameRows.Close()
				return nil, fmt.Errorf("scan title name: %w", err)
			}
			titles[i].Names = append(titles[i].Names, n)
		}
		nameRows.Close()

		seasonRows, err := r.db.Query(`
			SELECT s.id, s.title_id, s.season_number, s.total_episodes, s.my_rating,
				COUNT(e.id) AS episode_count,
				SUM(CASE WHEN e.watched THEN 1 ELSE 0 END) AS watched_count
			FROM seasons s
			LEFT JOIN episodes e ON e.season_id = s.id
			WHERE s.title_id = ?
			GROUP BY s.id
			ORDER BY s.season_number`, titles[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get seasons light: %w", err)
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
			titles[i].Seasons = append(titles[i].Seasons, s)
		}
		seasonRows.Close()

		// Load next unwatched episode for quick-mark
		var ne model.NextEpisode
		err = r.db.QueryRow(`
			SELECT e.id, e.season_id, e.episode, s.season_number
			FROM episodes e
			JOIN seasons s ON s.id = e.season_id
			WHERE s.title_id = ? AND e.watched = 0
			ORDER BY s.season_number, e.episode
			LIMIT 1`, titles[i].ID).Scan(&ne.ID, &ne.SeasonID, &ne.Episode, &ne.SeasonNumber)
		if err == nil {
			titles[i].NextEpisode = &ne
		}
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

// FindByExternalID looks up a title by external IDs (IMDB, TMDB, Plex rating key).
func (r *TitleRepository) FindByExternalID(imdbID *string, tmdbID *int64, plexRatingKey *string) (*model.Title, error) {
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

	if len(conditions) == 0 {
		return nil, sql.ErrNoRows
	}

	query := `SELECT id FROM titles WHERE ` + strings.Join(conditions, ` OR `) + ` LIMIT 1`
	var id int64
	if err := r.db.QueryRow(query, args...).Scan(&id); err != nil {
		return nil, err
	}

	return r.GetByID(id)
}
