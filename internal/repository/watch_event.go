package repository

import (
	"fmt"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
)

type WatchEventRepository struct {
	db database.DBTX
}

func NewWatchEventRepository(db database.DBTX) *WatchEventRepository {
	return &WatchEventRepository{db: db}
}

func (r *WatchEventRepository) Create(event *model.WatchEvent) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO watch_events (title_id, episode_id, source, plex_payload) VALUES (?, ?, ?, ?)`,
		event.TitleID, event.EpisodeID, event.Source, event.PlexPayload)
	if err != nil {
		return 0, fmt.Errorf("create watch event: %w", err)
	}
	return res.LastInsertId()
}

// BatchCreate inserts multiple watch events in a single statement.
func (r *WatchEventRepository) BatchCreate(events []model.WatchEvent) error {
	if len(events) == 0 {
		return nil
	}

	placeholders := make([]string, len(events))
	args := make([]interface{}, 0, len(events)*4)
	for i, e := range events {
		placeholders[i] = "(?, ?, ?, ?)"
		args = append(args, e.TitleID, e.EpisodeID, e.Source, e.PlexPayload)
	}

	query := fmt.Sprintf("INSERT INTO watch_events (title_id, episode_id, source, plex_payload) VALUES %s",
		strings.Join(placeholders, ", "))
	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("batch create watch events: %w", err)
	}
	return nil
}

func (r *WatchEventRepository) ListByTitle(titleID int64) ([]model.WatchEvent, error) {
	rows, err := r.db.Query(`SELECT id, title_id, episode_id, source, plex_payload, created_at FROM watch_events WHERE title_id = ? ORDER BY created_at DESC LIMIT 500`, titleID)
	if err != nil {
		return nil, fmt.Errorf("list watch events: %w", err)
	}
	defer rows.Close()

	var events []model.WatchEvent
	for rows.Next() {
		var e model.WatchEvent
		if err := rows.Scan(&e.ID, &e.TitleID, &e.EpisodeID, &e.Source, &e.PlexPayload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan watch event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}
