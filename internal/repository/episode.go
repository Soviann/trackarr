package repository

import (
	"fmt"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
)

type EpisodeRepository struct {
	db database.DBTX
}

func NewEpisodeRepository(db database.DBTX) *EpisodeRepository {
	return &EpisodeRepository{db: db}
}

// EpisodeUpsert holds episode data for a batch upsert operation.
type EpisodeUpsert struct {
	EpisodeNumber int
	Name          string
	AirDate       string
}

func (r *EpisodeRepository) GetBySeasonID(seasonID int64) ([]model.Episode, error) {
	rows, err := r.db.Query(`SELECT id, season_id, episode, name, air_date, watched, first_watched_at, last_watched_at, external_source_id FROM episodes WHERE season_id = ? ORDER BY episode`, seasonID)
	if err != nil {
		return nil, fmt.Errorf("get episodes: %w", err)
	}
	defer rows.Close()

	var episodes []model.Episode
	for rows.Next() {
		var e model.Episode
		if err := rows.Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.FirstWatchedAt, &e.LastWatchedAt, &e.ExternalSourceID); err != nil {
			return nil, fmt.Errorf("scan episode: %w", err)
		}
		episodes = append(episodes, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate episodes: %w", err)
	}
	return episodes, nil
}
