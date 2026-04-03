package model

import "time"

type WatchEventSource string

const (
	WatchEventSourcePlex   WatchEventSource = "plex"
	WatchEventSourceManual WatchEventSource = "manual"
)

type WatchEvent struct {
	ID          int64            `json:"id"`
	TitleID     int64            `json:"title_id"`
	EpisodeID   *int64           `json:"episode_id"`
	Source      WatchEventSource `json:"source"`
	PlexPayload *string          `json:"plex_payload"`
	CreatedAt   time.Time        `json:"created_at"`
}
