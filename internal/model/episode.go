package model

import "time"

type Episode struct {
	ID            int64      `json:"id"`
	SeasonID      int64      `json:"season_id"`
	Episode       int        `json:"episode"`
	Name          *string    `json:"name"`
	AirDate       *string    `json:"air_date"`
	Watched       bool       `json:"watched"`
	WatchedAt     *time.Time `json:"watched_at"`
	PlexRatingKey *string    `json:"plex_rating_key"`
}
