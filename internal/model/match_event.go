package model

import "time"

type MatchEventKind string

const (
	MatchEventAutoConfirmed  MatchEventKind = "auto_confirmed"
	MatchEventSeasonAttached MatchEventKind = "season_attached"
)

// MatchEvent records a matching decision taken without user review, so the
// "Recently auto-matched" section can show what happened after the fact.
// TitleID points at the surviving title (the parent for season_attached) and
// is nil once that title is deleted (FK cascade keeps the table consistent).
type MatchEvent struct {
	ID        int64          `json:"id"`
	TitleID   *int64         `json:"title_id"`
	Kind      MatchEventKind `json:"kind"`
	Detail    string         `json:"detail"`
	CreatedAt time.Time      `json:"created_at"`
	// Joined for display (nil when the title is gone)
	CoverURL *string `json:"cover_url,omitempty"`
}
