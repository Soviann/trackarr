package matching

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type TMDBGenre struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type TMDBCredits struct {
	Cast []TMDBCastMember `json:"cast"`
	Crew []TMDBCrewMember `json:"crew"`
}

type TMDBCastMember struct {
	Name      string `json:"name"`
	Character string `json:"character"`
	Order     int    `json:"order"`
}

type TMDBCrewMember struct {
	Name       string `json:"name"`
	Job        string `json:"job"`
	Department string `json:"department"`
}

type TMDBMovieDetails struct {
	ID          int64        `json:"id"`
	Title       string       `json:"title"`
	Overview    string       `json:"overview"`
	ReleaseDate string       `json:"release_date"`
	PosterPath  *string      `json:"poster_path"`
	IMDBID      string       `json:"imdb_id"`
	Genres      []TMDBGenre  `json:"genres"`
	Runtime     *int         `json:"runtime"`
	VoteAverage float64      `json:"vote_average"`
	Credits     *TMDBCredits `json:"credits"`
	ExternalIDs *struct {
		IMDBID string `json:"imdb_id"`
		TVDBID int64  `json:"tvdb_id"`
	} `json:"external_ids"`
}

type TMDBTVDetails struct {
	ID             int64        `json:"id"`
	Name           string       `json:"name"`
	Overview       string       `json:"overview"`
	Status         string       `json:"status"`
	FirstAirDate   string       `json:"first_air_date"`
	PosterPath     *string      `json:"poster_path"`
	Genres         []TMDBGenre  `json:"genres"`
	EpisodeRunTime []int        `json:"episode_run_time"`
	VoteAverage    float64      `json:"vote_average"`
	Credits        *TMDBCredits `json:"credits"`
	Seasons        []struct {
		SeasonNumber int `json:"season_number"`
		EpisodeCount int `json:"episode_count"`
	} `json:"seasons"`
	ExternalIDs *struct {
		IMDBID string `json:"imdb_id"`
		TVDBID int64  `json:"tvdb_id"`
	} `json:"external_ids"`
	NextEpisodeToAir *struct {
		AirDate       string `json:"air_date"`
		SeasonNumber  int    `json:"season_number"`
		EpisodeNumber int    `json:"episode_number"`
	} `json:"next_episode_to_air"`
}

type TMDBEpisode struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	AirDate       string `json:"air_date"`
}

type tmdbSeasonResponse struct {
	Episodes []TMDBEpisode `json:"episodes"`
}

type TMDBTranslation struct {
	ISO639  string `json:"iso_639_1"`
	ISO3166 string `json:"iso_3166_1"`
	Data    struct {
		Title string `json:"title"` // movies
		Name  string `json:"name"`  // TV shows
	} `json:"data"`
}

type tmdbTranslationsResponse struct {
	Translations []TMDBTranslation `json:"translations"`
}

func (c *TMDBClient) GetMovieDetails(ctx context.Context, tmdbID int64) (*TMDBMovieDetails, error) {
	var details TMDBMovieDetails
	params := url.Values{"append_to_response": {"external_ids,credits"}}
	if err := c.get(ctx, fmt.Sprintf("/movie/%d", tmdbID), params, &details); err != nil {
		return nil, fmt.Errorf("get movie details: %w", err)
	}
	return &details, nil
}

func (c *TMDBClient) GetTVDetails(ctx context.Context, tmdbID int64) (*TMDBTVDetails, error) {
	var details TMDBTVDetails
	params := url.Values{"append_to_response": {"external_ids,credits"}}
	if err := c.get(ctx, fmt.Sprintf("/tv/%d", tmdbID), params, &details); err != nil {
		return nil, fmt.Errorf("get tv details: %w", err)
	}
	return &details, nil
}

func (c *TMDBClient) GetTVSeasonEpisodes(ctx context.Context, tmdbID int64, seasonNumber int) ([]TMDBEpisode, error) {
	var resp tmdbSeasonResponse
	if err := c.get(ctx, fmt.Sprintf("/tv/%d/season/%d", tmdbID, seasonNumber), nil, &resp); err != nil {
		return nil, fmt.Errorf("get season episodes: %w", err)
	}
	return resp.Episodes, nil
}

// GetTitleNames returns multilingual names for a title (en, fr).
// For French, only the strict fr-FR variant is accepted — other regional
// variants (fr-CA, fr-BE, fr-CH, …) are ignored on purpose so the UI never
// surfaces a Quebec/Belgian/Swiss title to a France-based user. When fr-FR
// is missing, no "fr" entry is returned and the display layer falls back
// to English.
// mediaType should be "movie" or "tv".
func (c *TMDBClient) GetTitleNames(ctx context.Context, tmdbID int64, mediaType string) (map[string]string, error) {
	var resp tmdbTranslationsResponse
	if err := c.get(ctx, fmt.Sprintf("/%s/%d/translations", mediaType, tmdbID), nil, &resp); err != nil {
		return nil, fmt.Errorf("get translations: %w", err)
	}

	names := make(map[string]string)
	for _, t := range resp.Translations {
		name := t.Data.Title
		if name == "" {
			name = t.Data.Name
		}
		if name == "" {
			continue
		}
		switch t.ISO639 {
		case "en":
			names["en"] = name
		case "fr":
			if t.ISO3166 == "FR" {
				names["fr"] = name
			}
		}
	}
	return names, nil
}

// ExtractMovieMetadata builds JSON strings for genres and credits from TMDB movie details.
func ExtractMovieMetadata(d *TMDBMovieDetails) (genres, credits string, runtime *int, rating *float64) {
	genres = marshalGenres(d.Genres)
	credits = marshalCredits(d.Credits)
	if d.Runtime != nil && *d.Runtime > 0 {
		runtime = d.Runtime
	}
	if d.VoteAverage > 0 {
		rating = &d.VoteAverage
	}
	return
}

// ExtractTVMetadata builds JSON strings for genres and credits from TMDB TV details.
func ExtractTVMetadata(d *TMDBTVDetails) (genres, credits string, runtime *int, rating *float64) {
	genres = marshalGenres(d.Genres)
	credits = marshalCredits(d.Credits)
	if len(d.EpisodeRunTime) > 0 && d.EpisodeRunTime[0] > 0 {
		runtime = &d.EpisodeRunTime[0]
	}
	if d.VoteAverage > 0 {
		rating = &d.VoteAverage
	}
	return
}

func marshalGenres(genres []TMDBGenre) string {
	b, _ := json.Marshal(extractGenreNames(genres))
	return string(b)
}

// extractGenreNames returns genre names as a plain slice, without JSON marshaling.
func extractGenreNames(genres []TMDBGenre) []string {
	names := make([]string, 0, len(genres))
	for _, g := range genres {
		names = append(names, g.Name)
	}
	return names
}

func marshalCredits(c *TMDBCredits) string {
	if c == nil {
		return "[]"
	}
	type entry struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	var entries []entry
	for _, crew := range c.Crew {
		if crew.Job == "Director" {
			entries = append(entries, entry{Name: crew.Name, Role: "Director"})
		}
	}
	limit := 5
	if len(c.Cast) < limit {
		limit = len(c.Cast)
	}
	for _, cast := range c.Cast[:limit] {
		entries = append(entries, entry{Name: cast.Name, Role: cast.Character})
	}
	b, _ := json.Marshal(entries)
	return string(b)
}
