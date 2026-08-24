package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
)

type WatchEventRepository struct {
	db database.DBTX
}

func NewWatchEventRepository(db database.DBTX) *WatchEventRepository {
	return &WatchEventRepository{db: db}
}

func (r *WatchEventRepository) CountByTitleID(titleID int64) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM watch_events WHERE title_id = ?`, titleID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("watch_event: count by title: %w", err)
	}
	return count, nil
}

func (r *WatchEventRepository) ListByTitle(titleID int64) ([]model.WatchEvent, error) {
	rows, err := r.db.Query(`SELECT id, title_id, episode_id, source, raw_payload, created_at FROM watch_events WHERE title_id = ? ORDER BY created_at DESC LIMIT 500`, titleID)
	if err != nil {
		return nil, fmt.Errorf("list watch events: %w", err)
	}
	defer rows.Close()

	var events []model.WatchEvent
	for rows.Next() {
		var e model.WatchEvent
		if err := rows.Scan(&e.ID, &e.TitleID, &e.EpisodeID, &e.Source, &e.RawPayload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan watch event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate watch events: %w", err)
	}
	return events, nil
}

func (r *WatchEventRepository) GetLatestCreatedAtBySource(source model.WatchEventSource) (*time.Time, error) {
	var t time.Time
	err := r.db.QueryRow(`SELECT created_at FROM watch_events WHERE source = ? ORDER BY created_at DESC LIMIT 1`, source).Scan(&t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("watch_event: get latest by source: %w", err)
	}
	return &t, nil
}
