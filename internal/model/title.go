package model

import "time"

type TitleType string

const (
	TitleTypeMovie  TitleType = "movie"
	TitleTypeSeries TitleType = "series"
	TitleTypeAnime  TitleType = "anime"
)

type TitleStatus string

const (
	TitleStatusWatching    TitleStatus = "watching"
	TitleStatusCompleted   TitleStatus = "completed"
	TitleStatusDropped     TitleStatus = "dropped"
	TitleStatusPlanToWatch TitleStatus = "plan_to_watch"
)

type SeriesStatus string

const (
	SeriesStatusReturning    SeriesStatus = "returning"
	SeriesStatusEnded        SeriesStatus = "ended"
	SeriesStatusCancelled    SeriesStatus = "cancelled"
	SeriesStatusInProduction SeriesStatus = "in_production"
)

type MatchStatus string

const (
	MatchStatusConfirmed     MatchStatus = "confirmed"
	MatchStatusPendingReview MatchStatus = "pending_review"
	MatchStatusUnconfirmed   MatchStatus = "unconfirmed"
)

type Title struct {
	ID            int64         `json:"id"`
	Type          TitleType     `json:"type"`
	Year          int           `json:"year"`
	CoverURL      *string       `json:"cover_url"`
	IMDBID        *string       `json:"imdb_id"`
	AniListID     *int64        `json:"anilist_id"`
	TMDBID        *int64        `json:"tmdb_id"`
	TVDBID        *int64        `json:"tvdb_id"`
	PlexRatingKey *string       `json:"plex_rating_key"`
	MyRating      *int          `json:"my_rating"`
	Status        TitleStatus   `json:"status"`
	SeriesStatus  *SeriesStatus `json:"series_status"`
	MatchStatus   MatchStatus   `json:"match_status"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`

	// Loaded relations
	Names   []TitleName `json:"names,omitempty"`
	Seasons []Season    `json:"seasons,omitempty"`
}

// PrimaryName returns the primary display name, or the first name if none is primary.
func (t *Title) PrimaryName() string {
	for _, n := range t.Names {
		if n.IsPrimary {
			return n.Name
		}
	}
	if len(t.Names) > 0 {
		return t.Names[0].Name
	}
	return ""
}

type TitleName struct {
	ID        int64  `json:"id"`
	TitleID   int64  `json:"title_id"`
	Name      string `json:"name"`
	Language  string `json:"language"`
	IsPrimary bool   `json:"is_primary"`
}
