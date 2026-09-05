package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
)

type StatsRepository struct {
	db database.DBTX
}

func NewStatsRepository(db database.DBTX) *StatsRepository {
	return &StatsRepository{db: db}
}

func (r *StatsRepository) GetAll(ctx context.Context) (*model.StatsResponse, error) {
	return r.GetFiltered(ctx, model.StatsFilter{Timeframe: "all", MediaType: "all"})
}

func mediaTypeCondition(mediaType string, tableAlias string) (string, []any) {
	alias := ""
	if tableAlias != "" {
		alias = tableAlias + "."
	}
	switch mediaType {
	case "movie":
		return alias + "type = 'movie'", nil
	case "series":
		return alias + "type = 'series' AND " + alias + "is_anime = 0", nil
	case "anime":
		return alias + "is_anime = 1", nil
	default:
		return "1=1", nil
	}
}

func timeframeCondition(filter model.StatsFilter, tableAlias string) (string, []any) {
	alias := ""
	if tableAlias != "" {
		alias = tableAlias + "."
	}
	switch filter.Timeframe {
	case "year":
		if filter.Year > 0 {
			start := fmt.Sprintf("%04d-01-01 00:00:00", filter.Year)
			end := fmt.Sprintf("%04d-01-01 00:00:00", filter.Year+1)
			return alias + "created_at >= ? AND " + alias + "created_at < ?", []any{start, end}
		}
		return "1=1", nil
	case "30d", "30days":
		start := time.Now().AddDate(0, 0, -30).UTC().Format("2006-01-02 15:04:05")
		return alias + "created_at >= ?", []any{start}
	default:
		return "1=1", nil
	}
}

// GetFiltered returns stats matching the specified filter (timeframe & media type).
func (r *StatsRepository) GetFiltered(ctx context.Context, filter model.StatsFilter) (*model.StatsResponse, error) {
	isTimeFiltered := filter.Timeframe == "year" || filter.Timeframe == "30d" || filter.Timeframe == "30days"

	var overview *model.StatsOverview
	var err error
	if isTimeFiltered {
		overview, err = r.overviewTimeFiltered(ctx, filter)
	} else {
		overview, err = r.overviewFiltered(ctx, filter)
	}
	if err != nil {
		return nil, fmt.Errorf("stats overview: %w", err)
	}

	ratings, err := r.ratingsFiltered(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("stats ratings: %w", err)
	}

	breakdown, err := r.breakdownFiltered(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("stats breakdown: %w", err)
	}

	funStats, err := r.funStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("stats fun: %w", err)
	}

	targetYear := time.Now().Year()
	if filter.Timeframe == "year" && filter.Year > 0 {
		targetYear = filter.Year
	}
	yearSummary, err := r.yearSummary(ctx, targetYear)
	if err != nil {
		return nil, fmt.Errorf("stats year: %w", err)
	}

	genres, err := r.topGenresFiltered(ctx, 10, filter)
	if err != nil {
		return nil, fmt.Errorf("stats genres: %w", err)
	}

	topActors, err := r.topActorsFiltered(ctx, 10, filter)
	if err != nil {
		return nil, fmt.Errorf("stats top actors: %w", err)
	}

	topDirectors, err := r.topDirectorsFiltered(ctx, 10, filter)
	if err != nil {
		return nil, fmt.Errorf("stats top directors: %w", err)
	}

	currentStreak, err := r.CurrentStreak(ctx)
	if err != nil {
		return nil, fmt.Errorf("stats current streak: %w", err)
	}

	bestStreak, err := r.BestStreak(ctx)
	if err != nil {
		return nil, fmt.Errorf("stats best streak: %w", err)
	}

	totalWatchMinutes, err := r.totalWatchMinutesFiltered(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("stats total watch minutes: %w", err)
	}

	now := time.Now()
	watchedThisYear, avgRatingThisYear, err := r.libraryStripYear(ctx, now.Year())
	if err != nil {
		return nil, fmt.Errorf("stats library strip year: %w", err)
	}

	minutesThisWeek, err := r.MinutesSince(ctx, now.AddDate(0, 0, -7))
	if err != nil {
		return nil, fmt.Errorf("stats minutes this week: %w", err)
	}

	availableYears, err := r.AvailableYears(ctx)
	if err != nil {
		availableYears = []int{time.Now().Year()}
	}

	return &model.StatsResponse{
		Overview:     *overview,
		Ratings:      *ratings,
		Breakdown:    *breakdown,
		FunStats:     funStats,
		Year:         *yearSummary,
		Genres:       genres,
		TopActors:    topActors,
		TopDirectors: topDirectors,
		Streaks: model.StatsStreaks{
			Current: currentStreak,
			Best:    bestStreak,
		},
		TotalWatchMinutes: totalWatchMinutes,
		WatchedThisYear:   watchedThisYear,
		AvgRatingThisYear: avgRatingThisYear,
		MinutesThisWeek:   minutesThisWeek,
		AvailableYears:    availableYears,
	}, nil
}

func (r *StatsRepository) overviewFiltered(ctx context.Context, filter model.StatsFilter) (*model.StatsOverview, error) {
	typeCond, _ := mediaTypeCondition(filter.MediaType, "")
	o := &model.StatsOverview{}

	var completed int
	var avgRating sql.NullFloat64

	err := r.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN type = 'movie' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN type = 'series' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_anime = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0),
			AVG(my_rating)
		FROM titles
		WHERE %s
	`, typeCond)).Scan(&o.TotalTitles, &o.TotalMovies, &o.TotalSeries, &o.TotalAnime, &completed, &avgRating)
	if err != nil {
		return nil, fmt.Errorf("count titles: %w", err)
	}

	if filter.MediaType == "movie" {
		o.EpisodesWatched = 0
	} else {
		epTypeCond, _ := mediaTypeCondition(filter.MediaType, "t")
		err = r.db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM episodes e
			JOIN seasons s ON e.season_id = s.id
			JOIN titles t ON s.title_id = t.id
			WHERE e.watched = 1 AND %s
		`, epTypeCond)).Scan(&o.EpisodesWatched)
		if err != nil {
			return nil, fmt.Errorf("count episodes: %w", err)
		}
	}

	if o.TotalTitles > 0 {
		o.CompletionRate = math.Round(float64(completed)/float64(o.TotalTitles)*100) / 100
	}

	if avgRating.Valid {
		o.AverageRating = math.Round(avgRating.Float64*10) / 10
	}

	return o, nil
}

func (r *StatsRepository) overviewTimeFiltered(ctx context.Context, filter model.StatsFilter) (*model.StatsOverview, error) {
	timeCond, timeArgs := timeframeCondition(filter, "we")
	typeCond, _ := mediaTypeCondition(filter.MediaType, "t")
	where := fmt.Sprintf("%s AND %s", timeCond, typeCond)

	o := &model.StatsOverview{}
	var completed int
	var avgRating sql.NullFloat64

	q := fmt.Sprintf(`
		SELECT
			COUNT(DISTINCT t.id),
			COUNT(DISTINCT CASE WHEN t.type = 'movie' THEN t.id END),
			COUNT(DISTINCT CASE WHEN t.type = 'series' THEN t.id END),
			COUNT(DISTINCT CASE WHEN t.is_anime = 1 THEN t.id END),
			COUNT(DISTINCT CASE WHEN t.status = 'completed' THEN t.id END)
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id
		WHERE %s
	`, where)
	err := r.db.QueryRowContext(ctx, q, timeArgs...).Scan(&o.TotalTitles, &o.TotalMovies, &o.TotalSeries, &o.TotalAnime, &completed)
	if err != nil {
		return nil, fmt.Errorf("count titles (time filtered): %w", err)
	}

	if filter.MediaType == "movie" {
		o.EpisodesWatched = 0
	} else {
		err = r.db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM watch_events we
			JOIN titles t ON we.title_id = t.id
			WHERE we.episode_id IS NOT NULL AND %s
		`, where), timeArgs...).Scan(&o.EpisodesWatched)
		if err != nil {
			return nil, fmt.Errorf("count episodes (time filtered): %w", err)
		}
	}

	err = r.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT AVG(t.my_rating)
		FROM (
			SELECT DISTINCT t.id, t.my_rating
			FROM watch_events we
			JOIN titles t ON we.title_id = t.id
			WHERE t.my_rating IS NOT NULL AND %s
		) t
	`, where), timeArgs...).Scan(&avgRating)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("avg rating (time filtered): %w", err)
	}

	if o.TotalTitles > 0 {
		o.CompletionRate = math.Round(float64(completed)/float64(o.TotalTitles)*100) / 100
	}

	if avgRating.Valid {
		o.AverageRating = math.Round(avgRating.Float64*10) / 10
	}

	return o, nil
}

func (r *StatsRepository) ratingsFiltered(ctx context.Context, filter model.StatsFilter) (*model.StatsRatings, error) {
	s := &model.StatsRatings{
		AverageByType: make(map[string]float64),
	}

	isTimeFiltered := filter.Timeframe == "year" || filter.Timeframe == "30d" || filter.Timeframe == "30days"
	typeCond, _ := mediaTypeCondition(filter.MediaType, "t")

	var distQuery, avgQuery string
	var queryArgs []any

	if !isTimeFiltered {
		distQuery = fmt.Sprintf(`SELECT t.my_rating, COUNT(*) FROM titles t WHERE t.my_rating IS NOT NULL AND %s GROUP BY t.my_rating ORDER BY t.my_rating`, typeCond)
		avgQuery = fmt.Sprintf(`SELECT t.type, AVG(t.my_rating) FROM titles t WHERE t.my_rating IS NOT NULL AND %s GROUP BY t.type`, typeCond)
	} else {
		timeCond, timeArgs := timeframeCondition(filter, "we")
		queryArgs = timeArgs
		where := fmt.Sprintf("%s AND %s", timeCond, typeCond)
		distQuery = fmt.Sprintf(`
			SELECT t.my_rating, COUNT(DISTINCT t.id)
			FROM watch_events we
			JOIN titles t ON we.title_id = t.id
			WHERE t.my_rating IS NOT NULL AND %s
			GROUP BY t.my_rating
			ORDER BY t.my_rating
		`, where)
		avgQuery = fmt.Sprintf(`
			SELECT t.type, AVG(t.my_rating)
			FROM (
				SELECT DISTINCT t.id, t.type, t.my_rating
				FROM watch_events we
				JOIN titles t ON we.title_id = t.id
				WHERE t.my_rating IS NOT NULL AND %s
			) t
			GROUP BY t.type
		`, where)
	}

	rows, err := r.db.QueryContext(ctx, distQuery, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("rating distribution: %w", err)
	}
	var totalRated, highRated int
	for rows.Next() {
		var rating, count int
		if err := rows.Scan(&rating, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan rating: %w", err)
		}
		if rating >= 1 && rating <= 10 {
			s.Distribution[rating-1] = count
			totalRated += count
			if rating >= 7 {
				highRated += count
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate rating distribution: %w", err)
	}
	rows.Close()

	if totalRated > 0 {
		pct := int(math.Round(float64(highRated) / float64(totalRated) * 100))
		switch {
		case pct >= 60:
			s.Insight = fmt.Sprintf("You rate pretty generously — %d%% of your ratings are 7 or above.", pct)
		case pct <= 30:
			s.Insight = fmt.Sprintf("You're pretty demanding — only %d%% of your ratings are 7 or above.", pct)
		default:
			s.Insight = fmt.Sprintf("%d%% of your ratings are 7 or above.", pct)
		}
	}

	typeRows, err := r.db.QueryContext(ctx, avgQuery, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("avg by type: %w", err)
	}
	for typeRows.Next() {
		var t string
		var avg float64
		if err := typeRows.Scan(&t, &avg); err != nil {
			typeRows.Close()
			return nil, fmt.Errorf("scan avg type: %w", err)
		}
		s.AverageByType[t] = math.Round(avg*10) / 10
	}
	if err := typeRows.Err(); err != nil {
		typeRows.Close()
		return nil, fmt.Errorf("iterate avg by type: %w", err)
	}
	typeRows.Close()

	return s, nil
}

func (r *StatsRepository) breakdownFiltered(ctx context.Context, filter model.StatsFilter) (*model.StatsBreakdown, error) {
	b := &model.StatsBreakdown{
		ByStatus: make(map[string]int),
		ByType:   make(map[string]int),
	}

	isTimeFiltered := filter.Timeframe == "year" || filter.Timeframe == "30d" || filter.Timeframe == "30days"
	typeCond, _ := mediaTypeCondition(filter.MediaType, "t")

	var query string
	var args []any

	if !isTimeFiltered {
		query = fmt.Sprintf(`
			SELECT 'status' AS dim, t.status AS k, COUNT(*) FROM titles t WHERE %s GROUP BY t.status
			UNION ALL
			SELECT 'type', t.type, COUNT(*) FROM titles t WHERE %s GROUP BY t.type`, typeCond, typeCond)
	} else {
		timeCond, timeArgs := timeframeCondition(filter, "we")
		where := fmt.Sprintf("%s AND %s", timeCond, typeCond)
		query = fmt.Sprintf(`
			SELECT 'status' AS dim, t.status AS k, COUNT(DISTINCT t.id)
			FROM watch_events we JOIN titles t ON we.title_id = t.id
			WHERE %s GROUP BY t.status
			UNION ALL
			SELECT 'type', t.type, COUNT(DISTINCT t.id)
			FROM watch_events we JOIN titles t ON we.title_id = t.id
			WHERE %s GROUP BY t.type`, where, where)
		args = append(timeArgs, timeArgs...)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("breakdown: %w", err)
	}
	for rows.Next() {
		var dim, k string
		var count int
		if err := rows.Scan(&dim, &k, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan breakdown: %w", err)
		}
		switch dim {
		case "status":
			b.ByStatus[k] = count
		case "type":
			b.ByType[k] = count
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate breakdown: %w", err)
	}
	rows.Close()

	return b, nil
}

func (r *StatsRepository) totalWatchMinutesFiltered(ctx context.Context, filter model.StatsFilter) (int, error) {
	isTimeFiltered := filter.Timeframe == "year" || filter.Timeframe == "30d" || filter.Timeframe == "30days"
	typeCond, _ := mediaTypeCondition(filter.MediaType, "t")

	var total int
	if !isTimeFiltered {
		err := r.db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(t.total_watch_minutes), 0)
			FROM titles t
			WHERE %s
		`, typeCond)).Scan(&total)
		if err != nil {
			if strings.Contains(err.Error(), "no such column") {
				return 0, nil
			}
			return 0, fmt.Errorf("stats: total watch minutes: %w", err)
		}
		return total, nil
	}

	timeCond, timeArgs := timeframeCondition(filter, "we")
	where := fmt.Sprintf("%s AND %s AND t.runtime IS NOT NULL", timeCond, typeCond)
	err := r.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(t.runtime), 0)
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id
		WHERE %s
	`, where), timeArgs...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("stats: total watch minutes (time filtered): %w", err)
	}
	return total, nil
}

func (r *StatsRepository) topGenresFiltered(ctx context.Context, limit int, filter model.StatsFilter) ([]model.GenreStat, error) {
	isTimeFiltered := filter.Timeframe == "year" || filter.Timeframe == "30d" || filter.Timeframe == "30days"
	typeCond, _ := mediaTypeCondition(filter.MediaType, "t")

	var query string
	var args []any

	if !isTimeFiltered {
		query = fmt.Sprintf(`
			SELECT tg.genre, COUNT(*) AS count
			FROM title_genres tg
			JOIN titles t ON tg.title_id = t.id
			WHERE %s
			GROUP BY tg.genre
			ORDER BY count DESC
			LIMIT ?
		`, typeCond)
		args = []any{limit}
	} else {
		timeCond, timeArgs := timeframeCondition(filter, "we")
		where := fmt.Sprintf("%s AND %s", timeCond, typeCond)
		query = fmt.Sprintf(`
			SELECT tg.genre, COUNT(DISTINCT t.id) AS count
			FROM watch_events we
			JOIN titles t ON we.title_id = t.id
			JOIN title_genres tg ON tg.title_id = t.id
			WHERE %s
			GROUP BY tg.genre
			ORDER BY count DESC
			LIMIT ?
		`, where)
		args = append(timeArgs, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("stats: top genres: %w", err)
	}
	defer rows.Close()

	var results []model.GenreStat
	for rows.Next() {
		var g model.GenreStat
		if err := rows.Scan(&g.Genre, &g.Count); err != nil {
			return nil, fmt.Errorf("stats: genre scan: %w", err)
		}
		results = append(results, g)
	}
	return results, rows.Err()
}

func (r *StatsRepository) topActorsFiltered(ctx context.Context, limit int, filter model.StatsFilter) ([]model.PersonStat, error) {
	isTimeFiltered := filter.Timeframe == "year" || filter.Timeframe == "30d" || filter.Timeframe == "30days"
	typeCond, _ := mediaTypeCondition(filter.MediaType, "t")

	var query string
	var args []any

	if !isTimeFiltered {
		query = fmt.Sprintf(`
			SELECT
				json_extract(je.value, '$.name') AS name,
				COUNT(DISTINCT t.id) AS count
			FROM titles t, json_each(t.credits) je
			WHERE t.credits IS NOT NULL
			  AND t.credits != ''
			  AND t.credits != '[]'
			  AND json_valid(t.credits) = 1
			  AND json_extract(je.value, '$.role') != 'Director'
			  AND (t.last_watched_at IS NOT NULL OR t.status IN ('completed', 'watching'))
			  AND json_extract(je.value, '$.name') IS NOT NULL
			  AND trim(json_extract(je.value, '$.name')) != ''
			  AND %s
			GROUP BY name
			ORDER BY count DESC, name ASC
			LIMIT ?
		`, typeCond)
		args = []any{limit}
	} else {
		timeCond, timeArgs := timeframeCondition(filter, "we")
		where := fmt.Sprintf("%s AND %s", timeCond, typeCond)
		query = fmt.Sprintf(`
			SELECT
				json_extract(je.value, '$.name') AS name,
				COUNT(DISTINCT t.id) AS count
			FROM watch_events we
			JOIN titles t ON we.title_id = t.id,
			json_each(t.credits) je
			WHERE t.credits IS NOT NULL
			  AND t.credits != ''
			  AND t.credits != '[]'
			  AND json_valid(t.credits) = 1
			  AND json_extract(je.value, '$.role') != 'Director'
			  AND json_extract(je.value, '$.name') IS NOT NULL
			  AND trim(json_extract(je.value, '$.name')) != ''
			  AND %s
			GROUP BY name
			ORDER BY count DESC, name ASC
			LIMIT ?
		`, where)
		args = append(timeArgs, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("stats: top actors: %w", err)
	}
	defer rows.Close()

	results := []model.PersonStat{}
	for rows.Next() {
		var p model.PersonStat
		if err := rows.Scan(&p.Name, &p.Count); err != nil {
			return nil, fmt.Errorf("stats: actor scan: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (r *StatsRepository) topDirectorsFiltered(ctx context.Context, limit int, filter model.StatsFilter) ([]model.PersonStat, error) {
	isTimeFiltered := filter.Timeframe == "year" || filter.Timeframe == "30d" || filter.Timeframe == "30days"
	typeCond, _ := mediaTypeCondition(filter.MediaType, "t")

	var query string
	var args []any

	if !isTimeFiltered {
		query = fmt.Sprintf(`
			SELECT
				json_extract(je.value, '$.name') AS name,
				COUNT(DISTINCT t.id) AS count
			FROM titles t, json_each(t.credits) je
			WHERE t.credits IS NOT NULL
			  AND t.credits != ''
			  AND t.credits != '[]'
			  AND json_valid(t.credits) = 1
			  AND json_extract(je.value, '$.role') = 'Director'
			  AND (t.last_watched_at IS NOT NULL OR t.status IN ('completed', 'watching'))
			  AND json_extract(je.value, '$.name') IS NOT NULL
			  AND trim(json_extract(je.value, '$.name')) != ''
			  AND %s
			GROUP BY name
			ORDER BY count DESC, name ASC
			LIMIT ?
		`, typeCond)
		args = []any{limit}
	} else {
		timeCond, timeArgs := timeframeCondition(filter, "we")
		where := fmt.Sprintf("%s AND %s", timeCond, typeCond)
		query = fmt.Sprintf(`
			SELECT
				json_extract(je.value, '$.name') AS name,
				COUNT(DISTINCT t.id) AS count
			FROM watch_events we
			JOIN titles t ON we.title_id = t.id,
			json_each(t.credits) je
			WHERE t.credits IS NOT NULL
			  AND t.credits != ''
			  AND t.credits != '[]'
			  AND json_valid(t.credits) = 1
			  AND json_extract(je.value, '$.role') = 'Director'
			  AND json_extract(je.value, '$.name') IS NOT NULL
			  AND trim(json_extract(je.value, '$.name')) != ''
			  AND %s
			GROUP BY name
			ORDER BY count DESC, name ASC
			LIMIT ?
		`, where)
		args = append(timeArgs, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("stats: top directors: %w", err)
	}
	defer rows.Close()

	results := []model.PersonStat{}
	for rows.Next() {
		var p model.PersonStat
		if err := rows.Scan(&p.Name, &p.Count); err != nil {
			return nil, fmt.Errorf("stats: director scan: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

// libraryStripYear returns (count of titles last-watched in the given year, avg my_rating among them).
// Average is rounded to one decimal; 0 when no rated title qualifies.
func (r *StatsRepository) libraryStripYear(ctx context.Context, year int) (int, float64, error) {
	yearStart := fmt.Sprintf("%d-01-01", year)
	yearEnd := fmt.Sprintf("%d-01-01", year+1)

	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM titles
		WHERE last_watched_at >= ? AND last_watched_at < ?
	`, yearStart, yearEnd).Scan(&count)
	if err != nil {
		return 0, 0, fmt.Errorf("watched this year: %w", err)
	}

	var avg sql.NullFloat64
	err = r.db.QueryRowContext(ctx, `
		SELECT AVG(my_rating) FROM titles
		WHERE last_watched_at >= ? AND last_watched_at < ?
		  AND my_rating IS NOT NULL
	`, yearStart, yearEnd).Scan(&avg)
	if err != nil {
		return 0, 0, fmt.Errorf("avg rating this year: %w", err)
	}

	rounded := 0.0
	if avg.Valid {
		rounded = math.Round(avg.Float64*10) / 10
	}
	return count, rounded, nil
}

// MinutesSince returns the sum of title runtimes attached to watch events
// since the given timestamp. Episodes inherit the parent title's runtime
// (Trackarr's existing watchtime convention).
func (r *StatsRepository) MinutesSince(ctx context.Context, since time.Time) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(t.runtime), 0)
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id
		WHERE we.created_at >= ? AND t.runtime IS NOT NULL
	`, since.UTC().Format("2006-01-02 15:04:05")).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("minutes since: %w", err)
	}
	return total, nil
}

func (r *StatsRepository) overview(ctx context.Context) (*model.StatsOverview, error) {
	o := &model.StatsOverview{}

	var completed int
	var avgRating sql.NullFloat64

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN type = 'movie' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN type = 'series' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_anime = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0),
			AVG(my_rating)
		FROM titles
	`).Scan(&o.TotalTitles, &o.TotalMovies, &o.TotalSeries, &o.TotalAnime, &completed, &avgRating)
	if err != nil {
		return nil, fmt.Errorf("count titles: %w", err)
	}

	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM episodes WHERE watched = 1`).Scan(&o.EpisodesWatched)
	if err != nil {
		return nil, fmt.Errorf("count episodes: %w", err)
	}

	if o.TotalTitles > 0 {
		o.CompletionRate = math.Round(float64(completed)/float64(o.TotalTitles)*100) / 100
	}

	if avgRating.Valid {
		o.AverageRating = math.Round(avgRating.Float64*10) / 10
	}

	return o, nil
}

func (r *StatsRepository) ratings(ctx context.Context) (*model.StatsRatings, error) {
	s := &model.StatsRatings{
		AverageByType: make(map[string]float64),
	}

	rows, err := r.db.QueryContext(ctx, `SELECT my_rating, COUNT(*) FROM titles WHERE my_rating IS NOT NULL GROUP BY my_rating ORDER BY my_rating`)
	if err != nil {
		return nil, fmt.Errorf("rating distribution: %w", err)
	}
	var totalRated, highRated int
	for rows.Next() {
		var rating, count int
		if err := rows.Scan(&rating, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan rating: %w", err)
		}
		if rating >= 1 && rating <= 10 {
			s.Distribution[rating-1] = count
			totalRated += count
			if rating >= 7 {
				highRated += count
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate rating distribution: %w", err)
	}
	rows.Close()

	if totalRated > 0 {
		pct := int(math.Round(float64(highRated) / float64(totalRated) * 100))
		switch {
		case pct >= 60:
			s.Insight = fmt.Sprintf("You rate pretty generously — %d%% of your ratings are 7 or above.", pct)
		case pct <= 30:
			s.Insight = fmt.Sprintf("You're pretty demanding — only %d%% of your ratings are 7 or above.", pct)
		default:
			s.Insight = fmt.Sprintf("%d%% of your ratings are 7 or above.", pct)
		}
	}

	typeRows, err := r.db.QueryContext(ctx, `SELECT type, AVG(my_rating) FROM titles WHERE my_rating IS NOT NULL GROUP BY type`)
	if err != nil {
		return nil, fmt.Errorf("avg by type: %w", err)
	}
	for typeRows.Next() {
		var t string
		var avg float64
		if err := typeRows.Scan(&t, &avg); err != nil {
			typeRows.Close()
			return nil, fmt.Errorf("scan avg type: %w", err)
		}
		s.AverageByType[t] = math.Round(avg*10) / 10
	}
	if err := typeRows.Err(); err != nil {
		typeRows.Close()
		return nil, fmt.Errorf("iterate avg by type: %w", err)
	}
	typeRows.Close()

	return s, nil
}

func (r *StatsRepository) breakdown(ctx context.Context) (*model.StatsBreakdown, error) {
	b := &model.StatsBreakdown{
		ByStatus: make(map[string]int),
		ByType:   make(map[string]int),
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT 'status' AS dim, status AS k, COUNT(*) FROM titles GROUP BY status
		UNION ALL
		SELECT 'type', type, COUNT(*) FROM titles GROUP BY type`)
	if err != nil {
		return nil, fmt.Errorf("breakdown: %w", err)
	}
	for rows.Next() {
		var dim, k string
		var count int
		if err := rows.Scan(&dim, &k, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan breakdown: %w", err)
		}
		switch dim {
		case "status":
			b.ByStatus[k] = count
		case "type":
			b.ByType[k] = count
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate breakdown: %w", err)
	}
	rows.Close()

	return b, nil
}

func (r *StatsRepository) funStats(ctx context.Context) ([]model.FunStat, error) {
	var stats []model.FunStat

	// 1. Longest binge — day with most episodes watched for a single title
	var bingeCount int
	var bingeTitle, bingeDate string
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) AS cnt, `+displayNameExpr+` AS name, DATE(e.first_watched_at) AS d
		FROM episodes e
		JOIN seasons s ON e.season_id = s.id
		JOIN titles t ON s.title_id = t.id
		WHERE e.watched = 1 AND e.first_watched_at IS NOT NULL
		GROUP BY t.id, d
		ORDER BY cnt DESC
		LIMIT 1
	`).Scan(&bingeCount, &bingeTitle, &bingeDate)
	if err == nil && bingeCount >= 3 {
		d, _ := time.Parse("2006-01-02", bingeDate)
		stats = append(stats, model.FunStat{
			ID:     "longest_binge",
			Icon:   "flame",
			Title:  "Biggest binge",
			Value:  fmt.Sprintf("%d episodes", bingeCount),
			Detail: fmt.Sprintf("%s — %s", bingeTitle, d.Format("January 2, 2006")),
		})
	}

	// 2. Most loyal series — longest tracking duration
	var loyalTitle string
	var loyalDays int
	err = r.db.QueryRowContext(ctx, `
		SELECT `+displayNameExpr+` AS name, CAST(julianday(MAX(e.first_watched_at)) - julianday(t.created_at) AS INTEGER) AS days
		FROM titles t
		JOIN seasons s ON s.title_id = t.id
		JOIN episodes e ON e.season_id = s.id AND e.watched = 1 AND e.first_watched_at IS NOT NULL
		WHERE t.type IN ('series', 'anime')
		GROUP BY t.id
		ORDER BY days DESC
		LIMIT 1
	`).Scan(&loyalTitle, &loyalDays)
	if err == nil && loyalDays >= 30 {
		stats = append(stats, model.FunStat{
			ID:     "most_loyal",
			Icon:   "heart",
			Title:  "Most loyal series",
			Value:  fmt.Sprintf("%d days", loyalDays),
			Detail: loyalTitle,
		})
	}

	// 3. Speed completer — fastest completed title
	var speedTitle string
	var speedDays int
	err = r.db.QueryRowContext(ctx, `
		SELECT `+displayNameExpr+` AS name, CAST(julianday(MAX(e.first_watched_at)) - julianday(MIN(e.first_watched_at)) AS INTEGER) AS days
		FROM titles t
		JOIN seasons s ON s.title_id = t.id
		JOIN episodes e ON e.season_id = s.id AND e.watched = 1 AND e.first_watched_at IS NOT NULL
		WHERE t.status = 'completed' AND t.type IN ('series', 'anime')
		GROUP BY t.id
		HAVING COUNT(e.id) >= 5
		ORDER BY days ASC
		LIMIT 1
	`).Scan(&speedTitle, &speedDays)
	if err == nil {
		value := fmt.Sprintf("%d days", speedDays)
		if speedDays <= 1 {
			value = "1 day"
		}
		stats = append(stats, model.FunStat{
			ID:     "speed_completer",
			Icon:   "zap",
			Title:  "Speed completer",
			Value:  value,
			Detail: speedTitle,
		})
	}

	// 4. Night owl / Early bird — hour distribution of watches
	var nightCount, totalCount int
	err = r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN CAST(strftime('%H', first_watched_at) AS INTEGER) >= 20 OR CAST(strftime('%H', first_watched_at) AS INTEGER) < 6 THEN 1 ELSE 0 END), 0),
			COUNT(*)
		FROM episodes
		WHERE watched = 1 AND first_watched_at IS NOT NULL
	`).Scan(&nightCount, &totalCount)
	if err == nil && totalCount >= 10 {
		pct := int(math.Round(float64(nightCount) / float64(totalCount) * 100))
		if pct >= 55 {
			stats = append(stats, model.FunStat{
				ID:     "night_owl",
				Icon:   "moon",
				Title:  "Night owl",
				Value:  fmt.Sprintf("%d%% after 8pm", pct),
				Detail: "Most of your watching happens in the evening.",
			})
		} else if pct <= 25 {
			stats = append(stats, model.FunStat{
				ID:     "early_bird",
				Icon:   "sun",
				Title:  "Early bird",
				Value:  fmt.Sprintf("%d%% during the day", 100-pct),
				Detail: "You watch mostly during the day.",
			})
		}
	}

	// 5. Plex vs Manual
	var plexCount, manualCount int
	err = r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN source = 'plex' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN source = 'manual' THEN 1 ELSE 0 END), 0)
		FROM watch_events
	`).Scan(&plexCount, &manualCount)
	if err == nil && plexCount+manualCount > 0 {
		total := plexCount + manualCount
		plexPct := int(math.Round(float64(plexCount) / float64(total) * 100))
		stats = append(stats, model.FunStat{
			ID:     "plex_vs_manual",
			Icon:   "tv",
			Title:  "Plex vs manual",
			Value:  fmt.Sprintf("%d%% Plex, %d%% manual", plexPct, 100-plexPct),
			Detail: fmt.Sprintf("%d events total.", total),
		})
	}

	// 6. Rating gap — avg rating by type (highest vs lowest)
	typeRows, err := r.db.QueryContext(ctx, `
		SELECT type, AVG(my_rating) AS avg_r, COUNT(*) AS cnt
		FROM titles
		WHERE my_rating IS NOT NULL
		GROUP BY type
		HAVING cnt >= 3
		ORDER BY avg_r DESC
	`)
	if err == nil {
		type typeAvg struct {
			t   string
			avg float64
		}
		var avgs []typeAvg
		for typeRows.Next() {
			var t string
			var avg float64
			var cnt int
			if err := typeRows.Scan(&t, &avg, &cnt); err == nil {
				avgs = append(avgs, typeAvg{t, math.Round(avg*10) / 10})
			}
		}
		typeRows.Close()
		if len(avgs) >= 2 {
			high, low := avgs[0], avgs[len(avgs)-1]
			if high.avg-low.avg >= 0.5 {
				typeLabel := map[string]string{"movie": "movies", "series": "series"}
				stats = append(stats, model.FunStat{
					ID:     "rating_gap",
					Icon:   "bar-chart",
					Title:  "Rating gap",
					Value:  fmt.Sprintf("%.1f vs %.1f", high.avg, low.avg),
					Detail: fmt.Sprintf("Your %s average %.1f, vs %.1f for your %s.", typeLabel[high.t], high.avg, low.avg, typeLabel[low.t]),
				})
			}
		}
	}

	// 7. Decade preference
	var topDecade, topCount int
	var totalForDecade int
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM titles`).Scan(&totalForDecade)
	if err == nil && totalForDecade > 0 {
		err = r.db.QueryRowContext(ctx, `
			SELECT (year / 10) * 10 AS decade, COUNT(*) AS cnt
			FROM titles
			GROUP BY decade
			ORDER BY cnt DESC
			LIMIT 1
		`).Scan(&topDecade, &topCount)
		if err == nil {
			pct := int(math.Round(float64(topCount) / float64(totalForDecade) * 100))
			stats = append(stats, model.FunStat{
				ID:     "decade_preference",
				Icon:   "calendar",
				Title:  "Decade preference",
				Value:  fmt.Sprintf("%ds", topDecade),
				Detail: fmt.Sprintf("Most of your titles come from the %ds (%d%%).", topDecade, pct),
			})
		}
	}

	// 8. The graveyard — dropped count
	var droppedCount int
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM titles WHERE status = 'dropped'`).Scan(&droppedCount)
	if err == nil && droppedCount > 0 {
		stats = append(stats, model.FunStat{
			ID:     "graveyard",
			Icon:   "skull",
			Title:  "The graveyard",
			Value:  fmt.Sprintf("%d titles", droppedCount),
			Detail: "R.I.P.",
		})
	}

	// 9. Backlog pressure — plan_to_watch count + estimate
	var backlogCount int
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM titles WHERE status = 'plan_to_watch'`).Scan(&backlogCount)
	if err == nil && backlogCount > 0 {
		// Estimate: average episodes per day over the last 90 days
		var recentEps int
		err = r.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM episodes
			WHERE watched = 1 AND first_watched_at IS NOT NULL
			AND first_watched_at >= datetime('now', '-90 days')
		`).Scan(&recentEps)

		detail := fmt.Sprintf("%d titles waiting.", backlogCount)
		if err == nil && recentEps > 0 {
			// Count total episodes in plan_to_watch titles
			var backlogEps int
			err = r.db.QueryRowContext(ctx, `
				SELECT COALESCE(SUM(s.total_episodes), 0)
				FROM seasons s
				JOIN titles t ON s.title_id = t.id
				WHERE t.status = 'plan_to_watch'
			`).Scan(&backlogEps)
			if err == nil && backlogEps > 0 {
				epsPerDay := float64(recentEps) / 90.0
				daysNeeded := float64(backlogEps) / epsPerDay
				months := int(math.Round(daysNeeded / 30))
				if months > 0 {
					detail = fmt.Sprintf("%d titles waiting. That's ~%d months at your current pace.", backlogCount, months)
				}
			}
		}

		stats = append(stats, model.FunStat{
			ID:     "backlog_pressure",
			Icon:   "clock",
			Title:  "Backlog pressure",
			Value:  fmt.Sprintf("%d titles", backlogCount),
			Detail: detail,
		})
	}

	// 10. Peak month — month with the most watched episodes
	var peakMonth string
	var peakCount int
	err = r.db.QueryRowContext(ctx, `
		SELECT strftime('%Y-%m', first_watched_at) AS m, COUNT(*) AS cnt
		FROM episodes
		WHERE watched = 1 AND first_watched_at IS NOT NULL
		GROUP BY m
		ORDER BY cnt DESC
		LIMIT 1
	`).Scan(&peakMonth, &peakCount)
	if err == nil && peakCount >= 5 {
		t, _ := time.Parse("2006-01", peakMonth)
		stats = append(stats, model.FunStat{
			ID:     "peak_month",
			Icon:   "trophy",
			Title:  "Peak month",
			Value:  fmt.Sprintf("%d episodes", peakCount),
			Detail: fmt.Sprintf("%s %d, your record.", t.Month().String(), t.Year()),
		})
	}

	return stats, nil
}

func (r *StatsRepository) yearSummary(ctx context.Context, year int) (*model.StatsYear, error) {
	y := &model.StatsYear{}
	yearStart := fmt.Sprintf("%d-01-01", year)
	yearEnd := fmt.Sprintf("%d-01-01", year+1)

	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM titles WHERE created_at >= ? AND created_at < ?`, yearStart, yearEnd).Scan(&y.TitlesAdded)
	if err != nil {
		return nil, fmt.Errorf("titles added: %w", err)
	}

	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM episodes WHERE watched = 1 AND first_watched_at >= ? AND first_watched_at < ?`, yearStart, yearEnd).Scan(&y.EpisodesWatched)
	if err != nil {
		return nil, fmt.Errorf("eps watched year: %w", err)
	}

	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM titles WHERE status = 'completed' AND updated_at >= ? AND updated_at < ?`, yearStart, yearEnd).Scan(&y.Completions)
	if err != nil {
		return nil, fmt.Errorf("completions year: %w", err)
	}

	return y, nil
}

// TotalWatchMinutes returns the sum of total_watch_minutes across all titles.
// Returns 0 gracefully if the column does not exist (soft dependency on watchtime plan).
func (r *StatsRepository) TotalWatchMinutes(ctx context.Context) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total_watch_minutes), 0) FROM titles
	`).Scan(&total)
	if err != nil {
		// Gracefully handle missing column (soft dependency)
		if strings.Contains(err.Error(), "no such column") {
			return 0, nil
		}
		return 0, fmt.Errorf("stats: total watch minutes: %w", err)
	}
	return total, nil
}

// TopGenres returns the top N genres by title count.
// Returns an empty slice gracefully if the title_genres table does not exist (soft dependency on search-filter plan).
func (r *StatsRepository) TopGenres(ctx context.Context, limit int) ([]model.GenreStat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT genre, COUNT(*) AS count
		FROM title_genres
		GROUP BY genre
		ORDER BY count DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("stats: top genres: %w", err)
	}
	defer rows.Close()

	var results []model.GenreStat
	for rows.Next() {
		var g model.GenreStat
		if err := rows.Scan(&g.Genre, &g.Count); err != nil {
			return nil, fmt.Errorf("stats: genre scan: %w", err)
		}
		results = append(results, g)
	}
	return results, rows.Err()
}

// CurrentStreak returns the number of consecutive calendar days (ending today or yesterday) with ≥1 watch event.
func (r *StatsRepository) CurrentStreak(ctx context.Context) (int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT DATE(created_at) AS day
		FROM watch_events
		ORDER BY day DESC
		LIMIT 400
	`)
	if err != nil {
		return 0, fmt.Errorf("stats: current streak: %w", err)
	}
	defer rows.Close()

	var days []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0, err
		}
		days = append(days, d)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	return computeCurrentStreak(days, time.Now()), nil
}

func computeCurrentStreak(days []string, now time.Time) int {
	if len(days) == 0 {
		return 0
	}
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	// Streak must end today or yesterday
	if days[0] != today && days[0] != yesterday {
		return 0
	}

	streak := 1
	for i := 1; i < len(days); i++ {
		prev, _ := time.Parse("2006-01-02", days[i-1])
		curr, _ := time.Parse("2006-01-02", days[i])
		if prev.AddDate(0, 0, -1).Format("2006-01-02") == curr.Format("2006-01-02") {
			streak++
		} else {
			break
		}
	}
	return streak
}

// BestStreak returns the longest ever consecutive watch streak.
func (r *StatsRepository) BestStreak(ctx context.Context) (int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT DATE(created_at) AS day
		FROM watch_events
		ORDER BY day ASC
	`)
	if err != nil {
		return 0, fmt.Errorf("stats: best streak: %w", err)
	}
	defer rows.Close()

	var days []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0, err
		}
		days = append(days, d)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	return computeBestStreak(days), nil
}

func computeBestStreak(days []string) int {
	if len(days) == 0 {
		return 0
	}
	best, current := 1, 1
	for i := 1; i < len(days); i++ {
		prev, _ := time.Parse("2006-01-02", days[i-1])
		curr, _ := time.Parse("2006-01-02", days[i])
		if prev.AddDate(0, 0, 1).Format("2006-01-02") == curr.Format("2006-01-02") {
			current++
			if current > best {
				best = current
			}
		} else {
			current = 1
		}
	}
	return best
}

// TopActors returns the top N actors by distinct watched titles count.
func (r *StatsRepository) TopActors(ctx context.Context, limit int) ([]model.PersonStat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			json_extract(je.value, '$.name') AS name,
			COUNT(DISTINCT t.id) AS count
		FROM titles t, json_each(t.credits) je
		WHERE t.credits IS NOT NULL
		  AND t.credits != ''
		  AND t.credits != '[]'
		  AND json_valid(t.credits) = 1
		  AND json_extract(je.value, '$.role') != 'Director'
		  AND (t.last_watched_at IS NOT NULL OR t.status IN ('completed', 'watching'))
		  AND json_extract(je.value, '$.name') IS NOT NULL
		  AND trim(json_extract(je.value, '$.name')) != ''
		GROUP BY name
		ORDER BY count DESC, name ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("stats: top actors: %w", err)
	}
	defer rows.Close()

	results := []model.PersonStat{}
	for rows.Next() {
		var p model.PersonStat
		if err := rows.Scan(&p.Name, &p.Count); err != nil {
			return nil, fmt.Errorf("stats: actor scan: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

// TopDirectors returns the top N directors by distinct watched titles count.
func (r *StatsRepository) TopDirectors(ctx context.Context, limit int) ([]model.PersonStat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			json_extract(je.value, '$.name') AS name,
			COUNT(DISTINCT t.id) AS count
		FROM titles t, json_each(t.credits) je
		WHERE t.credits IS NOT NULL
		  AND t.credits != ''
		  AND t.credits != '[]'
		  AND json_valid(t.credits) = 1
		  AND json_extract(je.value, '$.role') = 'Director'
		  AND (t.last_watched_at IS NOT NULL OR t.status IN ('completed', 'watching'))
		  AND json_extract(je.value, '$.name') IS NOT NULL
		  AND trim(json_extract(je.value, '$.name')) != ''
		GROUP BY name
		ORDER BY count DESC, name ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("stats: top directors: %w", err)
	}
	defer rows.Close()

	results := []model.PersonStat{}
	for rows.Next() {
		var p model.PersonStat
		if err := rows.Scan(&p.Name, &p.Count); err != nil {
			return nil, fmt.Errorf("stats: director scan: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

// AvailableYears returns all calendar years from the earliest release or watch year down to the current year, descending.
func (r *StatsRepository) AvailableYears(ctx context.Context) ([]int, error) {
	currentYear := time.Now().Year()
	var minYear int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MIN(y), ?)
		FROM (
			SELECT MIN(year) AS y FROM titles WHERE year IS NOT NULL AND year > 1900
			UNION ALL
			SELECT MIN(CAST(strftime('%Y', created_at) AS INTEGER)) AS y FROM watch_events WHERE created_at IS NOT NULL AND CAST(strftime('%Y', created_at) AS INTEGER) > 1900
		)
	`, currentYear).Scan(&minYear)
	if err != nil || minYear <= 1900 || minYear > currentYear {
		minYear = currentYear
	}

	var years []int
	for y := currentYear; y >= minYear; y-- {
		years = append(years, y)
	}
	if len(years) == 0 {
		years = []int{currentYear}
	}
	return years, nil
}

func (r *StatsRepository) queryWrappedTitleItems(ctx context.Context, query string, args ...any) ([]model.WrappedTitleItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.WrappedTitleItem
	for rows.Next() {
		var item model.WrappedTitleItem
		var name string
		var origTitle, cover, accent, releaseDate sql.NullString
		var rating sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&name,
			&origTitle,
			&item.Year,
			&item.Type,
			&item.IsAnime,
			&cover,
			&accent,
			&rating,
			&item.WatchCount,
			&releaseDate,
		); err != nil {
			return nil, err
		}
		item.Title = name
		if origTitle.Valid && origTitle.String != "" {
			item.OriginalTitle = &origTitle.String
		}
		if cover.Valid && cover.String != "" {
			item.CoverURL = &cover.String
		}
		if accent.Valid && accent.String != "" {
			item.AccentHex = &accent.String
		}
		if releaseDate.Valid && releaseDate.String != "" {
			item.ReleaseDate = &releaseDate.String
		}
		if rating.Valid {
			rVal := int(rating.Int64)
			item.MyRating = &rVal
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetWrappedData collects raw stats and partial WrappedResponse for the specified year.
func (r *StatsRepository) GetWrappedData(ctx context.Context, year int) (*model.WrappedRawStats, *model.WrappedResponse, error) {
	availableYears, err := r.AvailableYears(ctx)
	if err != nil {
		return nil, nil, err
	}

	targetYear := year
	if targetYear <= 0 {
		targetYear = availableYears[0]
	}

	yearStart := fmt.Sprintf("%04d-01-01 00:00:00", targetYear)
	yearEnd := fmt.Sprintf("%04d-01-01 00:00:00", targetYear+1)

	// 1. Overview for year
	var totalTitles, totalMovies, totalSeries, totalAnime, completions int
	var avgRating sql.NullFloat64
	err = r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(DISTINCT t.id),
			COUNT(DISTINCT CASE WHEN t.type = 'movie' THEN t.id END),
			COUNT(DISTINCT CASE WHEN t.type = 'series' AND t.is_anime = 0 THEN t.id END),
			COUNT(DISTINCT CASE WHEN t.is_anime = 1 THEN t.id END),
			COUNT(DISTINCT CASE WHEN t.status = 'completed' THEN t.id END),
			AVG(t.my_rating)
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id
		WHERE we.created_at >= ? AND we.created_at < ?
	`, yearStart, yearEnd).Scan(&totalTitles, &totalMovies, &totalSeries, &totalAnime, &completions, &avgRating)
	if err != nil {
		return nil, nil, fmt.Errorf("wrapped overview: %w", err)
	}

	var episodesWatched int
	_ = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM watch_events
		WHERE episode_id IS NOT NULL AND created_at >= ? AND created_at < ?
	`, yearStart, yearEnd).Scan(&episodesWatched)

	var totalWatchMinutes int
	_ = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(t.runtime), 0)
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id
		WHERE we.created_at >= ? AND we.created_at < ? AND t.runtime IS NOT NULL
	`, yearStart, yearEnd).Scan(&totalWatchMinutes)

	completionRate := 0.0
	if totalTitles > 0 {
		completionRate = math.Round(float64(completions)/float64(totalTitles)*100) / 100
	}
	averageRating := 0.0
	if avgRating.Valid {
		averageRating = math.Round(avgRating.Float64*10) / 10
	}

	overview := model.StatsOverview{
		TotalTitles:     totalTitles,
		TotalMovies:     totalMovies,
		TotalSeries:     totalSeries,
		TotalAnime:      totalAnime,
		EpisodesWatched: episodesWatched,
		CompletionRate:  completionRate,
		AverageRating:   averageRating,
	}

	// 2. Top Favorites (Movies, Series, Anime)
	const selectTitleFields = `
		SELECT
			t.id,
			` + displayNameExpr + ` AS name,
			t.original_title,
			t.year,
			t.type,
			t.is_anime,
			t.cover_url,
			t.accent_hex,
			t.my_rating,
			CASE
				WHEN (SELECT COUNT(*) FROM episodes e JOIN seasons s ON e.season_id = s.id WHERE s.title_id = t.id) = 0
					THEN (COUNT(CASE WHEN we.source IN ('plex', 'jellyfin', 'emby') THEN 1 END) +
					      CASE WHEN COUNT(CASE WHEN we.source NOT IN ('plex', 'jellyfin', 'emby') THEN 1 END) > 0 THEN 1 ELSE 0 END)
				ELSE (COUNT(CASE WHEN we.source IN ('plex', 'jellyfin', 'emby') AND we.episode_id IS NOT NULL THEN 1 END) +
				      COUNT(DISTINCT CASE WHEN we.source NOT IN ('plex', 'jellyfin', 'emby') AND we.episode_id IS NOT NULL THEN we.episode_id END))
			END AS watch_count,
			t.release_date
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id
		WHERE we.created_at >= ? AND we.created_at < ?
	`

	favMovies, _ := r.queryWrappedTitleItems(ctx, selectTitleFields+`
		AND t.type = 'movie'
		GROUP BY t.id
		ORDER BY COALESCE(t.my_rating, 0) DESC, watch_count DESC, t.id DESC
		LIMIT 3
	`, yearStart, yearEnd)

	favSeries, _ := r.queryWrappedTitleItems(ctx, selectTitleFields+`
		AND t.type = 'series' AND t.is_anime = 0
		GROUP BY t.id
		ORDER BY COALESCE(t.my_rating, 0) DESC, watch_count DESC, t.id DESC
		LIMIT 3
	`, yearStart, yearEnd)

	favAnime, _ := r.queryWrappedTitleItems(ctx, selectTitleFields+`
		AND t.is_anime = 1
		GROUP BY t.id
		ORDER BY COALESCE(t.my_rating, 0) DESC, watch_count DESC, t.id DESC
		LIMIT 3
	`, yearStart, yearEnd)

	topFavorites := model.WrappedCategoryTop{
		Movies: favMovies,
		Series: favSeries,
		Anime:  favAnime,
	}

	// 3. Top Releases of the Year
	yearStr := fmt.Sprintf("%04d", targetYear)
	relMovies, _ := r.queryWrappedTitleItems(ctx, selectTitleFields+`
		AND t.type = 'movie'
		AND (t.year = ? OR strftime('%Y', t.release_date) = ?)
		GROUP BY t.id
		ORDER BY COALESCE(t.my_rating, 0) DESC, watch_count DESC, t.id DESC
		LIMIT 3
	`, yearStart, yearEnd, targetYear, yearStr)

	relSeries, _ := r.queryWrappedTitleItems(ctx, selectTitleFields+`
		AND t.type = 'series' AND t.is_anime = 0
		AND (t.year = ? OR strftime('%Y', t.release_date) = ?)
		GROUP BY t.id
		ORDER BY COALESCE(t.my_rating, 0) DESC, watch_count DESC, t.id DESC
		LIMIT 3
	`, yearStart, yearEnd, targetYear, yearStr)

	relAnime, _ := r.queryWrappedTitleItems(ctx, selectTitleFields+`
		AND t.is_anime = 1
		AND (t.year = ? OR strftime('%Y', t.release_date) = ?)
		GROUP BY t.id
		ORDER BY COALESCE(t.my_rating, 0) DESC, watch_count DESC, t.id DESC
		LIMIT 3
	`, yearStart, yearEnd, targetYear, yearStr)

	topReleases := model.WrappedCategoryTop{
		Movies: relMovies,
		Series: relSeries,
		Anime:  relAnime,
	}

	// 4. Rewatch Champion
	// Automated scrobbles (plex, jellyfin, emby) are counted in full;
	// manual marks/backfills are capped at 1 per title (movies) or 1 per episode (series).
	var rewatchChamp *model.WrappedRewatch
	var bestTitleID int64
	var isMovie bool
	var totalEffectivePlays, distinctEps int
	var cycles float64

	champRow := r.db.QueryRowContext(ctx, `
		SELECT
			t.id,
			(SELECT COUNT(*) FROM episodes e JOIN seasons s ON e.season_id = s.id WHERE s.title_id = t.id) = 0 AS is_pure_movie,
			CASE
				WHEN (SELECT COUNT(*) FROM episodes e JOIN seasons s ON e.season_id = s.id WHERE s.title_id = t.id) = 0
					THEN (COUNT(CASE WHEN we.source IN ('plex', 'jellyfin', 'emby') THEN 1 END) +
					      CASE WHEN COUNT(CASE WHEN we.source NOT IN ('plex', 'jellyfin', 'emby') THEN 1 END) > 0 THEN 1 ELSE 0 END)
				ELSE (COUNT(CASE WHEN we.source IN ('plex', 'jellyfin', 'emby') AND we.episode_id IS NOT NULL THEN 1 END) +
				      COUNT(DISTINCT CASE WHEN we.source NOT IN ('plex', 'jellyfin', 'emby') AND we.episode_id IS NOT NULL THEN we.episode_id END))
			END AS effective_plays,
			COUNT(DISTINCT we.episode_id) AS distinct_eps,
			CASE
				-- Episodic titles:
				WHEN (SELECT COUNT(*) FROM episodes e JOIN seasons s ON e.season_id = s.id WHERE s.title_id = t.id) > 0 THEN
					CASE
						WHEN COUNT(DISTINCT we.episode_id) > 0 THEN
							((COUNT(CASE WHEN we.source IN ('plex', 'jellyfin', 'emby') AND we.episode_id IS NOT NULL THEN 1 END) +
							  COUNT(DISTINCT CASE WHEN we.source NOT IN ('plex', 'jellyfin', 'emby') AND we.episode_id IS NOT NULL THEN we.episode_id END)) * 1.0 /
							 COUNT(DISTINCT we.episode_id))
						ELSE 1.0
					END
				-- Pure movies:
				ELSE (COUNT(CASE WHEN we.source IN ('plex', 'jellyfin', 'emby') THEN 1 END) +
				      CASE WHEN COUNT(CASE WHEN we.source NOT IN ('plex', 'jellyfin', 'emby') THEN 1 END) > 0 THEN 1 ELSE 0 END) * 1.0
			END AS cycles
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id
		WHERE we.created_at >= ? AND we.created_at < ?
		GROUP BY t.id
		ORDER BY cycles DESC, effective_plays DESC, t.id DESC
		LIMIT 1
	`, yearStart, yearEnd)

	if err := champRow.Scan(&bestTitleID, &isMovie, &totalEffectivePlays, &distinctEps, &cycles); err == nil && bestTitleID > 0 {
		champItems, _ := r.queryWrappedTitleItems(ctx, selectTitleFields+`
			AND t.id = ?
			GROUP BY t.id
			LIMIT 1
		`, yearStart, yearEnd, bestTitleID)

		if len(champItems) > 0 {
			topItem := champItems[0]
			calcPlays := totalEffectivePlays
			if !isMovie {
				calcPlays = int(math.Round(cycles))
				if calcPlays < 1 {
					calcPlays = 1
				}
			}

			rewatchChamp = &model.WrappedRewatch{
				Title:            topItem,
				TotalPlays:       calcPlays,
				IsMovie:          isMovie,
				DistinctEpisodes: distinctEps,
				TotalEpisodes:    totalEffectivePlays,
			}
		}
	}

	// 5. Top Genres in Year
	genreRows, err := r.db.QueryContext(ctx, `
		SELECT tg.genre, COUNT(DISTINCT t.id) AS count
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id
		JOIN title_genres tg ON tg.title_id = t.id
		WHERE we.created_at >= ? AND we.created_at < ?
		GROUP BY tg.genre
		ORDER BY count DESC
		LIMIT 5
	`, yearStart, yearEnd)
	var topGenres []model.GenreStat
	if err == nil {
		for genreRows.Next() {
			var g model.GenreStat
			if err := genreRows.Scan(&g.Genre, &g.Count); err == nil {
				topGenres = append(topGenres, g)
			}
		}
		genreRows.Close()
	}

	// 6. Top Actors in Year
	actorRows, err := r.db.QueryContext(ctx, `
		SELECT
			json_extract(je.value, '$.name') AS name,
			COUNT(DISTINCT t.id) AS count
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id,
		json_each(t.credits) je
		WHERE we.created_at >= ? AND we.created_at < ?
		  AND t.credits IS NOT NULL AND t.credits != '' AND t.credits != '[]'
		  AND json_valid(t.credits) = 1
		  AND json_extract(je.value, '$.role') != 'Director'
		  AND json_extract(je.value, '$.name') IS NOT NULL
		  AND trim(json_extract(je.value, '$.name')) != ''
		GROUP BY name
		ORDER BY count DESC, name ASC
		LIMIT 5
	`, yearStart, yearEnd)
	var topActors []model.PersonStat
	if err == nil {
		for actorRows.Next() {
			var p model.PersonStat
			if err := actorRows.Scan(&p.Name, &p.Count); err == nil {
				topActors = append(topActors, p)
			}
		}
		actorRows.Close()
	}

	// 7. Top Directors in Year
	directorRows, err := r.db.QueryContext(ctx, `
		SELECT
			json_extract(je.value, '$.name') AS name,
			COUNT(DISTINCT t.id) AS count
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id,
		json_each(t.credits) je
		WHERE we.created_at >= ? AND we.created_at < ?
		  AND t.credits IS NOT NULL AND t.credits != '' AND t.credits != '[]'
		  AND json_valid(t.credits) = 1
		  AND json_extract(je.value, '$.role') = 'Director'
		  AND json_extract(je.value, '$.name') IS NOT NULL
		  AND trim(json_extract(je.value, '$.name')) != ''
		GROUP BY name
		ORDER BY count DESC, name ASC
		LIMIT 5
	`, yearStart, yearEnd)
	var topDirectors []model.PersonStat
	if err == nil {
		for directorRows.Next() {
			var p model.PersonStat
			if err := directorRows.Scan(&p.Name, &p.Count); err == nil {
				topDirectors = append(topDirectors, p)
			}
		}
		directorRows.Close()
	}

	// 8. Raw Rhythm Metrics (Night owl %, Peak day of week, Peak month, Longest binge, Best streak)
	var nightCount, allEvents int
	_ = r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN CAST(strftime('%H', created_at) AS INTEGER) >= 20 OR CAST(strftime('%H', created_at) AS INTEGER) < 6 THEN 1 ELSE 0 END), 0),
			COUNT(*)
		FROM watch_events
		WHERE created_at >= ? AND created_at < ?
	`, yearStart, yearEnd).Scan(&nightCount, &allEvents)
	nightOwlPct := 0
	if allEvents > 0 {
		nightOwlPct = int(math.Round(float64(nightCount) / float64(allEvents) * 100))
	}

	var peakDay string
	var peakDayCount int
	_ = r.db.QueryRowContext(ctx, `
		SELECT
			CASE CAST(strftime('%w', created_at) AS INTEGER)
				WHEN 0 THEN 'Sunday'
				WHEN 1 THEN 'Monday'
				WHEN 2 THEN 'Tuesday'
				WHEN 3 THEN 'Wednesday'
				WHEN 4 THEN 'Thursday'
				WHEN 5 THEN 'Friday'
				WHEN 6 THEN 'Saturday'
			END AS dow,
			COUNT(*) AS cnt
		FROM watch_events
		WHERE created_at >= ? AND created_at < ?
		GROUP BY strftime('%w', created_at)
		ORDER BY cnt DESC
		LIMIT 1
	`, yearStart, yearEnd).Scan(&peakDay, &peakDayCount)

	var peakMonthStr string
	var peakMonthCount int
	_ = r.db.QueryRowContext(ctx, `
		SELECT strftime('%Y-%m', created_at) AS m, COUNT(*) AS cnt
		FROM watch_events
		WHERE created_at >= ? AND created_at < ?
		GROUP BY m
		ORDER BY cnt DESC
		LIMIT 1
	`, yearStart, yearEnd).Scan(&peakMonthStr, &peakMonthCount)

	peakMonth := peakMonthStr
	if t, err := time.Parse("2006-01", peakMonthStr); err == nil {
		peakMonth = t.Month().String()
	}

	var bingeEps int
	var bingeTitle string
	_ = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) AS cnt, `+displayNameExpr+` AS name
		FROM watch_events we
		JOIN titles t ON we.title_id = t.id
		WHERE we.created_at >= ? AND we.created_at < ? AND we.episode_id IS NOT NULL
		GROUP BY t.id, DATE(we.created_at)
		ORDER BY cnt DESC
		LIMIT 1
	`, yearStart, yearEnd).Scan(&bingeEps, &bingeTitle)

	streakRows, _ := r.db.QueryContext(ctx, `
		SELECT DISTINCT DATE(created_at) AS day
		FROM watch_events
		WHERE created_at >= ? AND created_at < ?
		ORDER BY day ASC
	`, yearStart, yearEnd)
	var streakDays []string
	if streakRows != nil {
		for streakRows.Next() {
			var d string
			if err := streakRows.Scan(&d); err == nil {
				streakDays = append(streakDays, d)
			}
		}
		streakRows.Close()
	}
	bestStreak := computeBestStreak(streakDays)

	var topGenreNames []string
	for _, g := range topGenres {
		topGenreNames = append(topGenreNames, g.Genre)
	}
	var topActorNames []string
	for _, a := range topActors {
		topActorNames = append(topActorNames, a.Name)
	}
	var topDirectorNames []string
	for _, d := range topDirectors {
		topDirectorNames = append(topDirectorNames, d.Name)
	}

	rawStats := &model.WrappedRawStats{
		Year:              targetYear,
		TotalTitles:       totalTitles,
		TotalMovies:       totalMovies,
		TotalSeries:       totalSeries,
		TotalAnime:        totalAnime,
		EpisodesWatched:   episodesWatched,
		TotalWatchMinutes: totalWatchMinutes,
		AverageRating:     averageRating,
		NightOwlPct:       nightOwlPct,
		PeakDayOfWeek:     peakDay,
		PeakMonth:         peakMonth,
		LongestBingeEps:   bingeEps,
		LongestBingeTitle: bingeTitle,
		BestStreakDays:    bestStreak,
		TopGenres:         topGenreNames,
		TopActors:         topActorNames,
		TopDirectors:      topDirectorNames,
		TopFavorites:      topFavorites,
		TopReleases:       topReleases,
		RewatchChampion:   rewatchChamp,
	}

	response := &model.WrappedResponse{
		Year:              targetYear,
		AvailableYears:    availableYears,
		Overview:          overview,
		TotalWatchMinutes: totalWatchMinutes,
		TopFavorites:      topFavorites,
		TopReleases:       topReleases,
		RewatchChampion:   rewatchChamp,
		TopGenres:         topGenres,
		TopActors:         topActors,
		TopDirectors:      topDirectors,
	}

	return rawStats, response, nil
}
