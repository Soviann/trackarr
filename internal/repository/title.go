package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/model"
)

type TitleRepository struct {
	db *sql.DB
}

func NewTitleRepository(db *sql.DB) *TitleRepository {
	return &TitleRepository{db: db}
}

type TitleFilter struct {
	Status      *model.TitleStatus
	Type        *model.TitleType
	Search      *string
	MatchStatus *model.MatchStatus
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
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO titles (type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		title.Type, title.Year, title.CoverURL, title.IMDBID, title.AniListID, title.TMDBID, title.TVDBID,
		title.PlexRatingKey, title.MyRating, title.Status, title.SeriesStatus, title.MatchStatus,
	)
	if err != nil {
		return 0, fmt.Errorf("insert title: %w", err)
	}

	id, _ := res.LastInsertId()

	for _, name := range names {
		_, err := tx.Exec(`INSERT INTO title_names (title_id, name, language, is_primary) VALUES (?, ?, ?, ?)`,
			id, name.Name, name.Language, name.IsPrimary)
		if err != nil {
			return 0, fmt.Errorf("insert title name: %w", err)
		}
	}

	return id, tx.Commit()
}

func (r *TitleRepository) GetByID(id int64) (*model.Title, error) {
	title := &model.Title{}
	err := r.db.QueryRow(`SELECT id, type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, created_at, updated_at FROM titles WHERE id = ?`, id).
		Scan(&title.ID, &title.Type, &title.Year, &title.CoverURL, &title.IMDBID, &title.AniListID, &title.TMDBID, &title.TVDBID,
			&title.PlexRatingKey, &title.MyRating, &title.Status, &title.SeriesStatus, &title.MatchStatus, &title.CreatedAt, &title.UpdatedAt)
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

func (r *TitleRepository) List(filter TitleFilter) ([]model.Title, error) {
	query := `SELECT DISTINCT t.id, t.type, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.created_at, t.updated_at FROM titles t`
	var conditions []string
	var args []interface{}

	if filter.Search != nil {
		query += ` JOIN title_names tn ON tn.title_id = t.id`
		conditions = append(conditions, `tn.name LIKE ?`)
		args = append(args, "%"+*filter.Search+"%")
	}

	if filter.Status != nil {
		conditions = append(conditions, `t.status = ?`)
		args = append(args, *filter.Status)
	}
	if filter.Type != nil {
		conditions = append(conditions, `t.type = ?`)
		args = append(args, *filter.Type)
	}
	if filter.MatchStatus != nil {
		conditions = append(conditions, `t.match_status = ?`)
		args = append(args, *filter.MatchStatus)
	}

	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, ` AND `)
	}

	query += ` ORDER BY t.updated_at DESC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list titles: %w", err)
	}

	var titles []model.Title
	for rows.Next() {
		var t model.Title
		if err := rows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
			&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.CreatedAt, &t.UpdatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan title: %w", err)
		}
		titles = append(titles, t)
	}
	rows.Close()

	// Load names, seasons, and episodes for all titles
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
