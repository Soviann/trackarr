package model

import "time"

type TitleType string

const (
	TitleTypeMovie  TitleType = "movie"
	TitleTypeSeries TitleType = "series"
)

type TitleStatus string

const (
	TitleStatusWatching    TitleStatus = "watching"
	TitleStatusCompleted   TitleStatus = "completed"
	TitleStatusDropped     TitleStatus = "dropped"
	TitleStatusPlanToWatch TitleStatus = "plan_to_watch"
)

// CombineMergedStatus reconciles the watch statuses of two titles being merged
// into one. older/newest are the statuses of the title blocks owning the lower
// and higher season numbers after the merge offset is applied. Rules (agreed
// with the PO):
//   - dropped is sticky: if either block is dropped, the series is dropped;
//   - otherwise the newest started season defines the series (completed/watching);
//   - if the newest season is plan_to_watch, fall back to the older block, but a
//     completed older becomes watching because newer unwatched content exists.
func CombineMergedStatus(older, newest TitleStatus) TitleStatus {
	switch {
	case older == TitleStatusDropped || newest == TitleStatusDropped:
		return TitleStatusDropped
	case newest == TitleStatusCompleted:
		return TitleStatusCompleted
	case newest == TitleStatusWatching:
		return TitleStatusWatching
	default: // newest == plan_to_watch
		if older == TitleStatusPlanToWatch {
			return TitleStatusPlanToWatch
		}
		return TitleStatusWatching
	}
}

type SeriesStatus string

const (
	SeriesStatusReturning SeriesStatus = "returning"
	SeriesStatusEnded     SeriesStatus = "ended"
	SeriesStatusCancelled SeriesStatus = "cancelled"
	// SeriesStatusInProduction is the "announced but not yet aired" bucket. The
	// stored value is "in_production" (fixed by the migration-001 CHECK
	// constraint) but it covers every pre-air TMDB status — In Production,
	// Planned, and Pilot — surfaced to users as the "Not started" filter chip.
	SeriesStatusInProduction SeriesStatus = "in_production"
)

type MatchStatus string

const (
	MatchStatusConfirmed     MatchStatus = "confirmed"
	MatchStatusPendingReview MatchStatus = "pending_review"
	MatchStatusUnconfirmed   MatchStatus = "unconfirmed"
)

// WatchProvider is a streaming service that carries a title at no extra cost
// (subscription-included / TMDB "flatrate") in the configured region.
type WatchProvider struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Title struct {
	ID                int64           `json:"id"`
	Type              TitleType       `json:"type"`
	IsAnime           bool            `json:"is_anime"`
	Year              int             `json:"year"`
	CoverURL          *string         `json:"cover_url"`
	IMDBID            *string         `json:"imdb_id"`
	AniListID         *int64          `json:"anilist_id"`
	TMDBID            *int64          `json:"tmdb_id"`
	TVDBID            *int64          `json:"tvdb_id"`
	ExternalSourceID  *string         `json:"external_source_id"`
	MyRating          *int            `json:"my_rating"`
	Status            TitleStatus     `json:"status"`
	SeriesStatus      *SeriesStatus   `json:"series_status"`
	MatchStatus       MatchStatus     `json:"match_status"`
	OriginalTitle     *string         `json:"original_title"`
	MatchSource       *string         `json:"match_source"`
	Overview          *string         `json:"overview"`
	Genres            []string        `json:"genres,omitempty"`
	WatchProviders    []WatchProvider `json:"watch_providers,omitempty"`
	Runtime           *int            `json:"runtime"`
	TotalWatchMinutes int             `json:"total_watch_minutes"`
	TMDBRating        *float64        `json:"tmdb_rating"`
	Credits           *string         `json:"credits"`
	AniListRating     *int            `json:"anilist_rating"`
	ReleaseDate       *string         `json:"release_date"`
	NextAirDate       *string         `json:"next_air_date,omitempty"`
	NextAirEpisode    *string         `json:"next_air_episode,omitempty"`
	FirstWatchedAt    *time.Time      `json:"first_watched_at,omitempty"`
	LastWatchedAt     *time.Time      `json:"last_watched_at,omitempty"`
	LastRefreshedAt   *time.Time      `json:"last_refreshed_at,omitempty"`
	AccentHex         *string         `json:"accent_hex,omitempty"`
	SimklID           *int64          `json:"simkl_id,omitempty"`
	SimklSlug         *string         `json:"simkl_slug,omitempty"`
	RadarrID          *int64          `json:"radarr_id,omitempty"`
	SonarrID          *int64          `json:"sonarr_id,omitempty"`
	ArrIgnored        bool            `json:"arr_ignored"`
	PersonalNotes     *string         `json:"personal_notes,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`

	// CaughtUp is true when this watching series has watched every aired
	// episode. Derived at query time by the list/search SQL (not stored);
	// false on responses that don't compute it (e.g. single-title GET).
	CaughtUp bool `json:"caught_up"`

	// Loaded relations
	Names     []TitleName     `json:"names,omitempty"`
	Seasons   []Season        `json:"seasons,omitempty"`
	Relations []TitleRelation `json:"relations,omitempty"`

	// Listing-only: next unwatched episode for quick mark
	NextEpisode *NextEpisode `json:"next_episode,omitempty"`

	// Search-only fields
	MatchedName     *string `json:"matched_name,omitempty"`
	MatchedLanguage *string `json:"matched_language,omitempty"`
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

// NextEpisode represents the first unwatched episode (for quick-mark in listing).
type NextEpisode struct {
	ID           int64   `json:"id"`
	SeasonID     int64   `json:"season_id"`
	Episode      int     `json:"episode"`
	SeasonNumber int     `json:"season_number"`
	Name         *string `json:"name,omitempty"`
	AirDate      *string `json:"air_date,omitempty"`
	IsTBA        bool    `json:"is_tba,omitempty"`
}

type TitleName struct {
	ID        int64  `json:"id"`
	TitleID   int64  `json:"title_id"`
	Name      string `json:"name"`
	Language  string `json:"language"`
	IsPrimary bool   `json:"is_primary"`
}
