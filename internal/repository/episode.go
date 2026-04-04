package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
)

type EpisodeRepository struct {
	db database.DBTX
}

func NewEpisodeRepository(db database.DBTX) *EpisodeRepository {
	return &EpisodeRepository{db: db}
}

// GetOrCreate returns the episode for the given season and number, creating it if needed.
func (r *EpisodeRepository) GetOrCreate(seasonID int64, episodeNumber int) (*model.Episode, error) {
	var e model.Episode
	err := r.db.QueryRow(`SELECT id, season_id, episode, name, air_date, watched, watched_at, plex_rating_key FROM episodes WHERE season_id = ? AND episode = ?`,
		seasonID, episodeNumber).Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.WatchedAt, &e.PlexRatingKey)
	if err == nil {
		return &e, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("get episode: %w", err)
	}

	res, err := r.db.Exec(`INSERT INTO episodes (season_id, episode) VALUES (?, ?)`, seasonID, episodeNumber)
	if err != nil {
		return nil, fmt.Errorf("create episode: %w", err)
	}

	id, _ := res.LastInsertId()
	return &model.Episode{ID: id, SeasonID: seasonID, Episode: episodeNumber}, nil
}

func (r *EpisodeRepository) ToggleWatched(id int64) (*model.Episode, error) {
	var watched bool
	err := r.db.QueryRow(`SELECT watched FROM episodes WHERE id = ?`, id).Scan(&watched)
	if err != nil {
		return nil, fmt.Errorf("get episode: %w", err)
	}

	if watched {
		_, err = r.db.Exec(`UPDATE episodes SET watched = 0, watched_at = NULL WHERE id = ?`, id)
	} else {
		_, err = r.db.Exec(`UPDATE episodes SET watched = 1, watched_at = ? WHERE id = ?`, time.Now().UTC(), id)
	}
	if err != nil {
		return nil, fmt.Errorf("toggle episode: %w", err)
	}

	var e model.Episode
	err = r.db.QueryRow(`SELECT id, season_id, episode, name, air_date, watched, watched_at, plex_rating_key FROM episodes WHERE id = ?`, id).
		Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.WatchedAt, &e.PlexRatingKey)
	return &e, err
}

func (r *EpisodeRepository) BatchMarkWatched(ids []int64, watchedAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, watchedAt.UTC())
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := fmt.Sprintf(`UPDATE episodes SET watched = 1, watched_at = ? WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("batch mark watched: %w", err)
	}
	return nil
}

// UpdateMetadata sets name and air_date on an episode, only if the new value is non-empty.
func (r *EpisodeRepository) UpdateMetadata(id int64, name, airDate string) error {
	var sets []string
	var args []interface{}

	if name != "" {
		sets = append(sets, "name = ?")
		args = append(args, name)
	}
	if airDate != "" {
		sets = append(sets, "air_date = ?")
		args = append(args, airDate)
	}
	if len(sets) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE episodes SET %s WHERE id = ?", strings.Join(sets, ", "))
	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update episode metadata: %w", err)
	}
	return nil
}

func (r *EpisodeRepository) GetBySeasonID(seasonID int64) ([]model.Episode, error) {
	rows, err := r.db.Query(`SELECT id, season_id, episode, name, air_date, watched, watched_at, plex_rating_key FROM episodes WHERE season_id = ? ORDER BY episode`, seasonID)
	if err != nil {
		return nil, fmt.Errorf("get episodes: %w", err)
	}
	defer rows.Close()

	var episodes []model.Episode
	for rows.Next() {
		var e model.Episode
		if err := rows.Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.WatchedAt, &e.PlexRatingKey); err != nil {
			return nil, fmt.Errorf("scan episode: %w", err)
		}
		episodes = append(episodes, e)
	}
	return episodes, nil
}

func (r *EpisodeRepository) MarkWatched(id int64, watchedAt time.Time) error {
	_, err := r.db.Exec(`UPDATE episodes SET watched = 1, watched_at = ? WHERE id = ?`, watchedAt.UTC(), id)
	if err != nil {
		return fmt.Errorf("mark watched: %w", err)
	}
	return nil
}
