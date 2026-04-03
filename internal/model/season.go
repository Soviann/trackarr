package model

type Season struct {
	ID            int64 `json:"id"`
	TitleID       int64 `json:"title_id"`
	SeasonNumber  int   `json:"season_number"`
	TotalEpisodes *int  `json:"total_episodes"`
	MyRating      *int  `json:"my_rating"`

	// Loaded relations
	Episodes []Episode `json:"episodes,omitempty"`
}
