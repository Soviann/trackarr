package repository

import (
	"database/sql"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/model"
)

type WatchEventRepository struct {
	db *sql.DB
}

func NewWatchEventRepository(db *sql.DB) *WatchEventRepository {
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

func (r *WatchEventRepository) ListByTitle(titleID int64) ([]model.WatchEvent, error) {
	rows, err := r.db.Query(`SELECT id, title_id, episode_id, source, plex_payload, created_at FROM watch_events WHERE title_id = ? ORDER BY created_at DESC`, titleID)
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
