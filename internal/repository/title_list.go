package repository

import (
	"fmt"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// airedEpisode matches an episode whose air date is known and in the past.
// Empty/NULL air_date counts as "not aired" (unknown) so not-yet-dated
// episodes never falsely block "caught up".
const airedEpisode = `e.air_date IS NOT NULL AND e.air_date != '' AND e.air_date <= date('now')`

const existsAiredUnwatched = `EXISTS (SELECT 1 FROM seasons s JOIN episodes e ON e.season_id = s.id WHERE s.title_id = t.id AND ` + airedEpisode + ` AND e.watched = 0)`

// caughtUpCond: watching series with no aired-unwatched episode.
const caughtUpCond = `t.status = 'watching' AND (t.type != 'movie' OR t.is_anime = 1) AND NOT ` + existsAiredUnwatched

// watchingBehindCond: the exact complement over watching titles.
const watchingBehindCond = `t.status = 'watching' AND ((t.type = 'movie' AND t.is_anime = 0) OR ` + existsAiredUnwatched + `)`

// titleSelectCols is the canonical title column list for list/search queries,
// with the derived caught_up flag appended. Centralized so the three query
// paths (List, searchTitles, fuzzySearch) cannot drift.
const titleSelectCols = `t.id, t.type, t.is_anime, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.original_title, t.match_source, t.overview, t.runtime, t.total_watch_minutes, t.tmdb_rating, t.credits, t.anilist_rating, t.release_date, t.next_air_date, t.next_air_episode, t.last_watched_at, t.accent_hex, t.simkl_id, t.simkl_slug, t.radarr_id, t.sonarr_id, t.arr_ignored, t.created_at, t.updated_at, (CASE WHEN ` + caughtUpCond + ` THEN 1 ELSE 0 END) AS caught_up`

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
	OriginCountries  []string // OR match on origin_country
	MyRatingMin      *int     // my_rating >= this
	TMDBRatingMin    *float64 // tmdb_rating >= this
}

const DefaultPageSize = 50
const MaxPageSize = 200

// allowedSortColumns guards against SQL injection in the ORDER BY clause by
// enforcing a hard whitelist of column names inside the repository itself,
// independently from whatever the handler layer may validate. Any caller that
// passes an unknown value gets the default ordering instead of a concatenated
// column name.
var allowedSortColumns = map[string]bool{
	"updated_at":      true,
	"original_title":  true,
	"year":            true,
	"my_rating":       true,
	"created_at":      true,
	"release_date":    true,
	"last_watched_at": true,
}

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

	var conditions []string
	var args []any

	switch {
	case filter.UpToDate:
		// "Up to date" = watching + every aired episode watched (air-date gated)
		conditions = append(conditions, caughtUpCond)
	case filter.WatchingBehind:
		// "Watching behind" = the exact complement of caught-up over watching titles
		conditions = append(conditions, watchingBehindCond)
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

	if len(filter.OriginCountries) > 0 {
		placeholders := make([]string, len(filter.OriginCountries))
		for i, c := range filter.OriginCountries {
			placeholders[i] = "?"
			args = append(args, c)
		}
		conditions = append(conditions, `t.origin_country IN (`+strings.Join(placeholders, ",")+`)`)
	}
	if filter.MyRatingMin != nil {
		conditions = append(conditions, `t.my_rating >= ?`)
		args = append(args, *filter.MyRatingMin)
	}
	if filter.TMDBRatingMin != nil {
		conditions = append(conditions, `t.tmdb_rating >= ?`)
		args = append(args, *filter.TMDBRatingMin)
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
	if filter.Sort != "" && allowedSortColumns[filter.Sort] {
		dir := "DESC"
		if filter.Order == "asc" {
			dir = "ASC"
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

	query := `SELECT ` + titleSelectCols + ` FROM titles t` + whereClause + ` ORDER BY ` + orderBy + ` LIMIT ? OFFSET ?`
	queryArgs := make([]any, len(args), len(args)+2)
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
			&t.ReleaseDate, &t.NextAirDate, &t.NextAirEpisode, &lastWatchedAtStr, &t.AccentHex, &t.SimklID, &t.SimklSlug, &t.RadarrID, &t.SonarrID, &t.ArrIgnored, &t.CreatedAt, &t.UpdatedAt, &t.CaughtUp); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan title: %w", err)
		}
		t.LastWatchedAt = parseSQLiteTime(lastWatchedAtStr)
		titles = append(titles, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate titles: %w", err)
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

// CountryCount holds an ISO-3166-1 origin country and how many titles carry it.
type CountryCount struct {
	Country string `json:"country"`
	Count   int    `json:"count"`
}

// ListOriginCountries returns the distinct origin countries present in the
// library with title counts, ordered by count desc then code asc. NULL/empty
// origins are excluded so the filter only offers countries that exist.
func (r *TitleRepository) ListOriginCountries() ([]CountryCount, error) {
	rows, err := r.db.Query(`
		SELECT origin_country, COUNT(*) AS count
		FROM titles
		WHERE origin_country IS NOT NULL AND origin_country != ''
		GROUP BY origin_country
		ORDER BY count DESC, origin_country ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list origin countries: %w", err)
	}
	defer rows.Close()

	var out []CountryCount
	for rows.Next() {
		var c CountryCount
		if err := rows.Scan(&c.Country, &c.Count); err != nil {
			return nil, fmt.Errorf("scan origin country: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
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
