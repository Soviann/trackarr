package model

import "time"

type Episode struct {
	ID               int64      `json:"id"`
	SeasonID         int64      `json:"season_id"`
	Episode          int        `json:"episode"`
	Name             *string    `json:"name"`
	AirDate          *string    `json:"air_date"`
	Watched          bool       `json:"watched"`
	FirstWatchedAt   *time.Time `json:"first_watched_at"`
	LastWatchedAt    *time.Time `json:"last_watched_at"`
	ExternalSourceID *string    `json:"external_source_id"`
}
