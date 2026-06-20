package model

// AniListPart represents one AniList entry mapped to a season. A season may
// have multiple parts (split-cour releases: Part 1, Part 2, …).
type AniListPart struct {
	ExternalID   string  `json:"external_id"`
	Score        *int    `json:"score"`
	EpisodeCount *int    `json:"episode_count"`
	StartDate    *string `json:"start_date"`
	SortOrder    *int    `json:"sort_order"`
}

type Season struct {
	ID            int64 `json:"id"`
	TitleID       int64 `json:"title_id"`
	SeasonNumber  int   `json:"season_number"`
	TotalEpisodes *int  `json:"total_episodes"`

	// Per-season AniList enrichment (populated on detail path only —
	// loadTitleRelationsLight skips them to avoid the JOIN cost).
	AniListID           *string `json:"anilist_id,omitempty"`
	AniListAverageScore *int    `json:"anilist_community_score,omitempty"`

	// AniListParts is the ordered list of AniList entries mapped to this
	// season (multiple for split-cour seasons). Detail path only.
	AniListParts []AniListPart `json:"anilist_parts"`

	// Listing counters (populated by loadTitleRelationsLight, omitted on detail)
	EpisodeCount *int `json:"episode_count,omitempty"`
	WatchedCount *int `json:"watched_count,omitempty"`

	// Loaded relations
	Episodes []Episode `json:"episodes"`
}
