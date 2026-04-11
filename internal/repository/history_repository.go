package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/database"
)

// EpisodeHistory represents the watch history for an episode (or movie-level) within a title.
type EpisodeHistory struct {
	EpisodeID     *int64   `json:"episode_id,omitempty"`
	EpisodeName   *string  `json:"episode_name,omitempty"`
	SeasonNumber  *int     `json:"season_number,omitempty"`
	EpisodeNumber *int     `json:"episode_number,omitempty"`
	WatchCount    int      `json:"watch_count"`
	LastWatchedAt string   `json:"last_watched_at"`
	AllWatches    []string `json:"watches"` // all created_at timestamps for this episode
}

// HistoryRepository provides access to per-title watch history.
type HistoryRepository struct {
	db database.DBTX
}

// NewHistoryRepository creates a new HistoryRepository.
func NewHistoryRepository(db database.DBTX) *HistoryRepository {
	return &HistoryRepository{db: db}
}

// GetByTitleID returns watch history grouped by episode for a given title.
func (r *HistoryRepository) GetByTitleID(_ context.Context, titleID int64) ([]EpisodeHistory, error) {
	rows, err := r.db.Query(`
		SELECT
			we.episode_id,
			e.name,
			s.season_number,
			e.episode AS episode_number,
			COUNT(*) AS watch_count,
			MAX(we.created_at) AS last_watched_at,
			GROUP_CONCAT(we.created_at, '|') AS all_watches
		FROM watch_events we
		LEFT JOIN episodes e ON e.id = we.episode_id
		LEFT JOIN seasons s ON s.id = e.season_id
		WHERE we.title_id = ?
		GROUP BY we.episode_id
		ORDER BY last_watched_at DESC
	`, titleID)
	if err != nil {
		return nil, fmt.Errorf("history: get: %w", err)
	}
	defer rows.Close()

	var results []EpisodeHistory
	for rows.Next() {
		var h EpisodeHistory
		var allWatches string
		if err := rows.Scan(
			&h.EpisodeID, &h.EpisodeName, &h.SeasonNumber, &h.EpisodeNumber,
			&h.WatchCount, &h.LastWatchedAt, &allWatches,
		); err != nil {
			return nil, fmt.Errorf("history: scan: %w", err)
		}
		if allWatches != "" {
			h.AllWatches = strings.Split(allWatches, "|")
		}
		results = append(results, h)
	}
	return results, rows.Err()
}
