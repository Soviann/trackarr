package repository

import (
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
)

type StatsRepository struct {
	db database.DBTX
}

func NewStatsRepository(db database.DBTX) *StatsRepository {
	return &StatsRepository{db: db}
}

func (r *StatsRepository) GetAll() (*model.StatsResponse, error) {
	overview, err := r.overview()
	if err != nil {
		return nil, fmt.Errorf("stats overview: %w", err)
	}

	ratings, err := r.ratings()
	if err != nil {
		return nil, fmt.Errorf("stats ratings: %w", err)
	}

	breakdown, err := r.breakdown()
	if err != nil {
		return nil, fmt.Errorf("stats breakdown: %w", err)
	}

	funStats, err := r.funStats()
	if err != nil {
		return nil, fmt.Errorf("stats fun: %w", err)
	}

	yearSummary, err := r.yearSummary(time.Now().Year())
	if err != nil {
		return nil, fmt.Errorf("stats year: %w", err)
	}

	return &model.StatsResponse{
		Overview:  *overview,
		Ratings:   *ratings,
		Breakdown: *breakdown,
		FunStats:  funStats,
		Year:      *yearSummary,
	}, nil
}

func (r *StatsRepository) overview() (*model.StatsOverview, error) {
	o := &model.StatsOverview{}

	err := r.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN type = 'movie' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN type = 'series' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_anime = 1 THEN 1 ELSE 0 END), 0)
		FROM titles
	`).Scan(&o.TotalTitles, &o.TotalMovies, &o.TotalSeries, &o.TotalAnime)
	if err != nil {
		return nil, fmt.Errorf("count titles: %w", err)
	}

	err = r.db.QueryRow(`SELECT COUNT(*) FROM episodes WHERE watched = 1`).Scan(&o.EpisodesWatched)
	if err != nil {
		return nil, fmt.Errorf("count episodes: %w", err)
	}

	if o.TotalTitles > 0 {
		var completed int
		err = r.db.QueryRow(`SELECT COUNT(*) FROM titles WHERE status = 'completed'`).Scan(&completed)
		if err != nil {
			return nil, fmt.Errorf("count completed: %w", err)
		}
		o.CompletionRate = math.Round(float64(completed)/float64(o.TotalTitles)*100) / 100
	}

	var avgRating sql.NullFloat64
	err = r.db.QueryRow(`SELECT AVG(my_rating) FROM titles WHERE my_rating IS NOT NULL`).Scan(&avgRating)
	if err != nil {
		return nil, fmt.Errorf("avg rating: %w", err)
	}
	if avgRating.Valid {
		o.AverageRating = math.Round(avgRating.Float64*10) / 10
	}

	return o, nil
}

func (r *StatsRepository) ratings() (*model.StatsRatings, error) {
	s := &model.StatsRatings{
		AverageByType: make(map[string]float64),
	}

	rows, err := r.db.Query(`SELECT my_rating, COUNT(*) FROM titles WHERE my_rating IS NOT NULL GROUP BY my_rating ORDER BY my_rating`)
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
	rows.Close()

	if totalRated > 0 {
		pct := int(math.Round(float64(highRated) / float64(totalRated) * 100))
		switch {
		case pct >= 60:
			s.Insight = fmt.Sprintf("Tu notes plutôt généreusement — %d%% de tes notes sont à 7 ou plus.", pct)
		case pct <= 30:
			s.Insight = fmt.Sprintf("Tu es plutôt exigeant — seulement %d%% de tes notes sont à 7 ou plus.", pct)
		default:
			s.Insight = fmt.Sprintf("%d%% de tes notes sont à 7 ou plus.", pct)
		}
	}

	typeRows, err := r.db.Query(`SELECT type, AVG(my_rating) FROM titles WHERE my_rating IS NOT NULL GROUP BY type`)
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
	typeRows.Close()

	return s, nil
}

func (r *StatsRepository) breakdown() (*model.StatsBreakdown, error) {
	b := &model.StatsBreakdown{
		ByStatus: make(map[string]int),
		ByType:   make(map[string]int),
	}

	rows, err := r.db.Query(`SELECT status, COUNT(*) FROM titles GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("by status: %w", err)
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan status: %w", err)
		}
		b.ByStatus[status] = count
	}
	rows.Close()

	typeRows, err := r.db.Query(`SELECT type, COUNT(*) FROM titles GROUP BY type`)
	if err != nil {
		return nil, fmt.Errorf("by type: %w", err)
	}
	for typeRows.Next() {
		var t string
		var count int
		if err := typeRows.Scan(&t, &count); err != nil {
			typeRows.Close()
			return nil, fmt.Errorf("scan type: %w", err)
		}
		b.ByType[t] = count
	}
	typeRows.Close()

	return b, nil
}

func (r *StatsRepository) funStats() ([]model.FunStat, error) {
	var stats []model.FunStat

	// 1. Longest binge — day with most episodes watched for a single title
	var bingeCount int
	var bingeTitle, bingeDate string
	err := r.db.QueryRow(`
		SELECT COUNT(*) AS cnt, tn.name, DATE(e.watched_at) AS d
		FROM episodes e
		JOIN seasons s ON e.season_id = s.id
		JOIN titles t ON s.title_id = t.id
		JOIN title_names tn ON tn.title_id = t.id AND tn.is_primary = 1
		WHERE e.watched = 1 AND e.watched_at IS NOT NULL
		GROUP BY t.id, d
		ORDER BY cnt DESC
		LIMIT 1
	`).Scan(&bingeCount, &bingeTitle, &bingeDate)
	if err == nil && bingeCount >= 3 {
		d, _ := time.Parse("2006-01-02", bingeDate)
		stats = append(stats, model.FunStat{
			ID:     "longest_binge",
			Icon:   "flame",
			Title:  "Plus gros binge",
			Value:  fmt.Sprintf("%d épisodes", bingeCount),
			Detail: fmt.Sprintf("%s — %s", bingeTitle, d.Format("2 janvier 2006")),
		})
	}

	// 2. Most loyal series — longest tracking duration
	var loyalTitle string
	var loyalDays int
	err = r.db.QueryRow(`
		SELECT tn.name, CAST(julianday(MAX(e.watched_at)) - julianday(t.created_at) AS INTEGER) AS days
		FROM titles t
		JOIN seasons s ON s.title_id = t.id
		JOIN episodes e ON e.season_id = s.id AND e.watched = 1 AND e.watched_at IS NOT NULL
		JOIN title_names tn ON tn.title_id = t.id AND tn.is_primary = 1
		WHERE t.type IN ('series', 'anime')
		GROUP BY t.id
		ORDER BY days DESC
		LIMIT 1
	`).Scan(&loyalTitle, &loyalDays)
	if err == nil && loyalDays >= 30 {
		stats = append(stats, model.FunStat{
			ID:     "most_loyal",
			Icon:   "heart",
			Title:  "Série la plus fidèle",
			Value:  fmt.Sprintf("%d jours", loyalDays),
			Detail: loyalTitle,
		})
	}

	// 3. Speed completer — fastest completed title
	var speedTitle string
	var speedDays int
	err = r.db.QueryRow(`
		SELECT tn.name, CAST(julianday(MAX(e.watched_at)) - julianday(MIN(e.watched_at)) AS INTEGER) AS days
		FROM titles t
		JOIN seasons s ON s.title_id = t.id
		JOIN episodes e ON e.season_id = s.id AND e.watched = 1 AND e.watched_at IS NOT NULL
		JOIN title_names tn ON tn.title_id = t.id AND tn.is_primary = 1
		WHERE t.status = 'completed' AND t.type IN ('series', 'anime')
		GROUP BY t.id
		HAVING COUNT(e.id) >= 5
		ORDER BY days ASC
		LIMIT 1
	`).Scan(&speedTitle, &speedDays)
	if err == nil {
		value := fmt.Sprintf("%d jours", speedDays)
		if speedDays <= 1 {
			value = "1 jour"
		}
		stats = append(stats, model.FunStat{
			ID:     "speed_completer",
			Icon:   "zap",
			Title:  "Sprint complétion",
			Value:  value,
			Detail: speedTitle,
		})
	}

	// 4. Night owl / Early bird — hour distribution of watches
	var nightCount, totalCount int
	err = r.db.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN CAST(strftime('%H', watched_at) AS INTEGER) >= 20 OR CAST(strftime('%H', watched_at) AS INTEGER) < 6 THEN 1 ELSE 0 END), 0),
			COUNT(*)
		FROM episodes
		WHERE watched = 1 AND watched_at IS NOT NULL
	`).Scan(&nightCount, &totalCount)
	if err == nil && totalCount >= 10 {
		pct := int(math.Round(float64(nightCount) / float64(totalCount) * 100))
		if pct >= 55 {
			stats = append(stats, model.FunStat{
				ID:     "night_owl",
				Icon:   "moon",
				Title:  "Oiseau de nuit",
				Value:  fmt.Sprintf("%d%% après 20h", pct),
				Detail: "La majorité de tes visionnages sont en soirée.",
			})
		} else if pct <= 25 {
			stats = append(stats, model.FunStat{
				ID:     "early_bird",
				Icon:   "sun",
				Title:  "Lève-tôt",
				Value:  fmt.Sprintf("%d%% en journée", 100-pct),
				Detail: "Tu regardes surtout en journée.",
			})
		}
	}

	// 5. Plex vs Manual
	var plexCount, manualCount int
	err = r.db.QueryRow(`
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
			Title:  "Plex vs Manuel",
			Value:  fmt.Sprintf("%d%% Plex, %d%% manuels", plexPct, 100-plexPct),
			Detail: fmt.Sprintf("%d événements au total.", total),
		})
	}

	// 6. Rating gap — avg rating by type (highest vs lowest)
	typeRows, err := r.db.Query(`
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
				typeLabel := map[string]string{"movie": "films", "series": "séries"}
				stats = append(stats, model.FunStat{
					ID:     "rating_gap",
					Icon:   "bar-chart",
					Title:  "Écart de notes",
					Value:  fmt.Sprintf("%.1f vs %.1f", high.avg, low.avg),
					Detail: fmt.Sprintf("Tes %s obtiennent %.1f de moyenne, contre %.1f pour tes %s.", typeLabel[high.t], high.avg, low.avg, typeLabel[low.t]),
				})
			}
		}
	}

	// 7. Decade preference
	var topDecade, topCount int
	var totalForDecade int
	err = r.db.QueryRow(`SELECT COUNT(*) FROM titles`).Scan(&totalForDecade)
	if err == nil && totalForDecade > 0 {
		err = r.db.QueryRow(`
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
				Title:  "Préférence décennies",
				Value:  fmt.Sprintf("Années %d", topDecade),
				Detail: fmt.Sprintf("La majorité de tes titres viennent des années %d (%d%%).", topDecade, pct),
			})
		}
	}

	// 8. The graveyard — dropped count
	var droppedCount int
	err = r.db.QueryRow(`SELECT COUNT(*) FROM titles WHERE status = 'dropped'`).Scan(&droppedCount)
	if err == nil && droppedCount > 0 {
		stats = append(stats, model.FunStat{
			ID:     "graveyard",
			Icon:   "skull",
			Title:  "Le cimetière",
			Value:  fmt.Sprintf("%d titres", droppedCount),
			Detail: "R.I.P.",
		})
	}

	// 9. Backlog pressure — plan_to_watch count + estimate
	var backlogCount int
	err = r.db.QueryRow(`SELECT COUNT(*) FROM titles WHERE status = 'plan_to_watch'`).Scan(&backlogCount)
	if err == nil && backlogCount > 0 {
		// Estimate: average episodes per day over the last 90 days
		var recentEps int
		err = r.db.QueryRow(`
			SELECT COUNT(*) FROM episodes
			WHERE watched = 1 AND watched_at IS NOT NULL
			AND watched_at >= datetime('now', '-90 days')
		`).Scan(&recentEps)

		detail := fmt.Sprintf("%d titres en attente.", backlogCount)
		if err == nil && recentEps > 0 {
			// Count total episodes in plan_to_watch titles
			var backlogEps int
			err = r.db.QueryRow(`
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
					detail = fmt.Sprintf("%d titres en attente. C'est ~%d mois au rythme actuel.", backlogCount, months)
				}
			}
		}

		stats = append(stats, model.FunStat{
			ID:     "backlog_pressure",
			Icon:   "clock",
			Title:  "Pression du backlog",
			Value:  fmt.Sprintf("%d titres", backlogCount),
			Detail: detail,
		})
	}

	// 10. Peak month — month with the most watched episodes
	var peakMonth string
	var peakCount int
	err = r.db.QueryRow(`
		SELECT strftime('%Y-%m', watched_at) AS m, COUNT(*) AS cnt
		FROM episodes
		WHERE watched = 1 AND watched_at IS NOT NULL
		GROUP BY m
		ORDER BY cnt DESC
		LIMIT 1
	`).Scan(&peakMonth, &peakCount)
	if err == nil && peakCount >= 5 {
		t, _ := time.Parse("2006-01", peakMonth)
		stats = append(stats, model.FunStat{
			ID:     "peak_month",
			Icon:   "trophy",
			Title:  "Mois record",
			Value:  fmt.Sprintf("%d épisodes", peakCount),
			Detail: fmt.Sprintf("%s %d, ton record.", frenchMonth(t.Month()), t.Year()),
		})
	}

	return stats, nil
}

func (r *StatsRepository) yearSummary(year int) (*model.StatsYear, error) {
	y := &model.StatsYear{}
	yearStr := fmt.Sprintf("%d", year)

	err := r.db.QueryRow(`SELECT COUNT(*) FROM titles WHERE strftime('%Y', created_at) = ?`, yearStr).Scan(&y.TitlesAdded)
	if err != nil {
		return nil, fmt.Errorf("titles added: %w", err)
	}

	err = r.db.QueryRow(`SELECT COUNT(*) FROM episodes WHERE watched = 1 AND watched_at IS NOT NULL AND strftime('%Y', watched_at) = ?`, yearStr).Scan(&y.EpisodesWatched)
	if err != nil {
		return nil, fmt.Errorf("eps watched year: %w", err)
	}

	err = r.db.QueryRow(`SELECT COUNT(*) FROM titles WHERE status = 'completed' AND strftime('%Y', updated_at) = ?`, yearStr).Scan(&y.Completions)
	if err != nil {
		return nil, fmt.Errorf("completions year: %w", err)
	}

	return y, nil
}

func frenchMonth(m time.Month) string {
	months := [...]string{
		"janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre",
	}
	return months[m-1]
}
