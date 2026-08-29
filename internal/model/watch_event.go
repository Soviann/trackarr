package model

import "time"

type WatchEventSource string

const (
	WatchEventSourcePlex     WatchEventSource = "plex"
	WatchEventSourceJellyfin WatchEventSource = "jellyfin"
	WatchEventSourceEmby     WatchEventSource = "emby"
	WatchEventSourceSimkl    WatchEventSource = "simkl"
	WatchEventSourceManual   WatchEventSource = "manual"
	WatchEventSourceBackfill WatchEventSource = "backfill"
)

type WatchEvent struct {
	ID         int64            `json:"id"`
	TitleID    int64            `json:"title_id"`
	EpisodeID  *int64           `json:"episode_id"`
	Source     WatchEventSource `json:"source"`
	RawPayload *string          `json:"raw_payload"`
	CreatedAt  time.Time        `json:"created_at"`
}
