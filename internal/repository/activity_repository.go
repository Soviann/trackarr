package repository

import (
	"context"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/database"
)

// ActivityEvent represents a single watch event entry for the activity feed.
type ActivityEvent struct {
	TitleID       int64   `json:"title_id"`
	TitleName     string  `json:"title_name"`
	CoverURL      *string `json:"cover_url"`
	TitleType     string  `json:"title_type"`
	EpisodeID     *int64  `json:"episode_id,omitempty"`
	EpisodeName   *string `json:"episode_name,omitempty"`
	SeasonNumber  *int    `json:"season_number,omitempty"`
	EpisodeNumber *int    `json:"episode_number,omitempty"`
	WatchedAt     string  `json:"watched_at"`
	IsCompletion  bool    `json:"is_completion"`
}

// ActivityRepository provides read access to activity feed data.
type ActivityRepository struct {
	db database.DBTX
}

// NewActivityRepository creates a new ActivityRepository.
func NewActivityRepository(db database.DBTX) *ActivityRepository {
	return &ActivityRepository{db: db}
}

// List returns paginated watch events ordered by most recent first.
func (r *ActivityRepository) List(_ context.Context, limit, offset int) ([]ActivityEvent, error) {
	rows, err := r.db.Query(`
		SELECT
			we.title_id,
			`+displayNameExpr+` AS name,
			t.cover_url,
			t.type,
			we.episode_id,
			e.name AS episode_name,
			s.season_number,
			e.episode AS episode_number,
			we.created_at,
			CASE
				WHEN t.status = 'completed'
				  AND we.episode_id IS NOT NULL
				  AND NOT EXISTS (
					SELECT 1 FROM episodes e2
					JOIN seasons s2 ON e2.season_id = s2.id
					WHERE s2.title_id = t.id AND e2.watched = 0
				  )
				THEN 1 ELSE 0
			END AS is_completion
		FROM watch_events we
		JOIN titles t ON t.id = we.title_id
		LEFT JOIN episodes e ON e.id = we.episode_id
		LEFT JOIN seasons s ON s.id = e.season_id
		ORDER BY we.created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("activity: list: %w", err)
	}
	defer rows.Close()

	var events []ActivityEvent
	for rows.Next() {
		var ev ActivityEvent
		var isCompletion int
		if err := rows.Scan(
			&ev.TitleID, &ev.TitleName, &ev.CoverURL, &ev.TitleType,
			&ev.EpisodeID, &ev.EpisodeName, &ev.SeasonNumber, &ev.EpisodeNumber,
			&ev.WatchedAt, &isCompletion,
		); err != nil {
			return nil, fmt.Errorf("activity: scan: %w", err)
		}
		ev.IsCompletion = isCompletion == 1
		events = append(events, ev)
	}
	return events, rows.Err()
}
