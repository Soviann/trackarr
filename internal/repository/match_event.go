package repository

import (
	"context"
	"fmt"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
)

// MatchEventRepository provides read access to match events.
type MatchEventRepository struct {
	db database.DBTX
}

// NewMatchEventRepository creates a new MatchEventRepository.
func NewMatchEventRepository(db database.DBTX) *MatchEventRepository {
	return &MatchEventRepository{db: db}
}

// ListRecent returns the most recent match events, newest first.
func (r *MatchEventRepository) ListRecent(ctx context.Context, limit int) ([]model.MatchEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.title_id, e.kind, e.detail, e.created_at, t.cover_url
		FROM match_events e
		LEFT JOIN titles t ON t.id = e.title_id
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("match_event: list recent: %w", err)
	}
	defer rows.Close()

	events := make([]model.MatchEvent, 0)
	for rows.Next() {
		var ev model.MatchEvent
		var createdAtStr string
		if err := rows.Scan(
			&ev.ID, &ev.TitleID, &ev.Kind, &ev.Detail, &createdAtStr, &ev.CoverURL,
		); err != nil {
			return nil, fmt.Errorf("match_event: scan: %w", err)
		}
		ev.CreatedAt = parseSQLiteTimeVal(createdAtStr)
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("match_event: iterate: %w", err)
	}
	return events, nil
}
