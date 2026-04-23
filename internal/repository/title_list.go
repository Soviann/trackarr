package repository

import (
	"fmt"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/model"
)

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
	Person           *string  // filter by credit name (json_each on credits column)
}

const DefaultPageSize = 50
const MaxPageSize = 200

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

func (r *TitleRepository) List(filter TitleFilter) (*PaginatedResult, error) {
	searchTerm := ""
	if filter.Search != nil {
		searchTerm = strings.TrimSpace(*filter.Search)
	}

	// Delegate search to relevance-ranked search
	if searchTerm != "" {
		return r.searchTitlesPaginated(searchTerm, filter)
	}

	baseCols := `t.id, t.type, t.is_anime, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.original_title, t.match_source, t.overview, t.runtime, t.total_watch_minutes, t.tmdb_rating, t.credits, t.anilist_rating, t.release_date, t.next_air_date, t.next_air_episode, t.last_watched_at, t.created_at, t.updated_at`

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

	if filter.Person != nil {
		conditions = append(conditions, `t.credits IS NOT NULL AND EXISTS (SELECT 1 FROM json_each(t.credits) je WHERE json_extract(je.value, '$.name') = ?)`)
		args = append(args, *filter.Person)
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
			&t.Overview, &t.Runtime, &t.TotalWatchMinutes, &t.TMDBRating, &t.Credits, &t.AniListRating,
			&t.ReleaseDate, &t.NextAirDate, &t.NextAirEpisode, &lastWatchedAtStr, &t.CreatedAt, &t.UpdatedAt); err != nil {
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
