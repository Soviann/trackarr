package model

type Season struct {
	ID            int64 `json:"id"`
	TitleID       int64 `json:"title_id"`
	SeasonNumber  int   `json:"season_number"`
	TotalEpisodes *int  `json:"total_episodes"`

	// Per-season AniList enrichment (populated on detail path only —
	// loadTitleRelationsLight skips them to avoid the JOIN cost).
	AniListID           *string `json:"anilist_id,omitempty"`
	AniListAverageScore *int    `json:"anilist_community_score,omitempty"`

	// Listing counters (populated by loadTitleRelationsLight, omitted on detail)
	EpisodeCount *int `json:"episode_count,omitempty"`
	WatchedCount *int `json:"watched_count,omitempty"`

	// Loaded relations
	Episodes []Episode `json:"episodes"`
}
