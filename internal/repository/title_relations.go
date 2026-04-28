package repository

import (
	"fmt"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/model"
)

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
	if err := nameRows.Err(); err != nil {
		nameRows.Close()
		return nil, fmt.Errorf("iterate title names bulk: %w", err)
	}
	nameRows.Close()

	// 2. Bulk load seasons
	seasonRows, err := r.db.Query(`SELECT id, title_id, season_number, total_episodes FROM seasons WHERE title_id IN (`+inClause+`) ORDER BY title_id, season_number`, args...)
	if err != nil {
		return nil, fmt.Errorf("get seasons bulk: %w", err)
	}

	seasonMap := make(map[int64]*model.Season)
	var seasonIDs []int64
	var seasonPlaceholders []string
	var seasonArgs []interface{}

	for seasonRows.Next() {
		var s model.Season
		if err := seasonRows.Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes); err != nil {
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
	if err := seasonRows.Err(); err != nil {
		seasonRows.Close()
		return nil, fmt.Errorf("iterate seasons bulk: %w", err)
	}
	seasonRows.Close()

	// 3. Bulk load episodes
	if len(seasonIDs) > 0 {
		epInClause := strings.Join(seasonPlaceholders, ",")
		epRows, err := r.db.Query(`SELECT id, season_id, episode, name, air_date, watched, first_watched_at, last_watched_at, plex_rating_key FROM episodes WHERE season_id IN (`+epInClause+`) ORDER BY season_id, episode`, seasonArgs...)
		if err != nil {
			return nil, fmt.Errorf("get episodes bulk: %w", err)
		}
		for epRows.Next() {
			var e model.Episode
			if err := epRows.Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.FirstWatchedAt, &e.LastWatchedAt, &e.PlexRatingKey); err != nil {
				epRows.Close()
				return nil, fmt.Errorf("scan episode: %w", err)
			}
			if s, ok := seasonMap[e.SeasonID]; ok {
				s.Episodes = append(s.Episodes, e)
			}
		}
		if err := epRows.Err(); err != nil {
			epRows.Close()
			return nil, fmt.Errorf("iterate episodes bulk: %w", err)
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
	if err := nameRows.Err(); err != nil {
		nameRows.Close()
		return nil, fmt.Errorf("iterate title names bulk light: %w", err)
	}
	nameRows.Close()

	// 2. Bulk load seasons with counts
	seasonRows, err := r.db.Query(`
		SELECT s.id, s.title_id, s.season_number, s.total_episodes,
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
		if err := seasonRows.Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes, &episodeCount, &watchedCount); err != nil {
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
	if err := seasonRows.Err(); err != nil {
		seasonRows.Close()
		return nil, fmt.Errorf("iterate seasons bulk light: %w", err)
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
