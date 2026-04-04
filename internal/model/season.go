package model

type Season struct {
	ID            int64 `json:"id"`
	TitleID       int64 `json:"title_id"`
	SeasonNumber  int   `json:"season_number"`
	TotalEpisodes *int  `json:"total_episodes"`
	MyRating      *int  `json:"my_rating"`

	// Listing counters (populated by loadTitleRelationsLight, omitted on detail)
	EpisodeCount *int `json:"episode_count,omitempty"`
	WatchedCount *int `json:"watched_count,omitempty"`

	// Loaded relations
	Episodes []Episode `json:"episodes"`
}
