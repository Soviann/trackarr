package model

import "time"

// ActivePlaybackSession represents a currently active media playback session (Plex or Jellyfin).
// Held exclusively in memory in PlaybackHubService to avoid SQLite write churn.
type ActivePlaybackSession struct {
	Source        string    `json:"source"`           // "plex" | "jellyfin"
	Player        string    `json:"player,omitempty"` // device or client name
	User          string    `json:"user,omitempty"`   // username if known
	TitleID       int64     `json:"title_id"`         // resolved Trackarr title ID
	TitleName     string    `json:"title_name"`       // primary title name
	TitleType     TitleType `json:"title_type"`       // "movie" | "series"
	Year          int       `json:"year"`
	CoverURL      *string   `json:"cover_url,omitempty"`
	AccentHex     *string   `json:"accent_hex,omitempty"`
	SeasonNumber  *int      `json:"season_number,omitempty"`
	EpisodeNumber *int      `json:"episode_number,omitempty"`
	EpisodeName   *string   `json:"episode_name,omitempty"`
	Overview      string    `json:"overview,omitempty"`
	DurationSec   int64     `json:"duration_sec,omitempty"`
	ViewOffsetSec int64     `json:"view_offset_sec,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
