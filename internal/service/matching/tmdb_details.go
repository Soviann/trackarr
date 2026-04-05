package matching

import (
	"fmt"
	"net/url"
)

type TMDBMovieDetails struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	ReleaseDate string  `json:"release_date"`
	PosterPath  *string `json:"poster_path"`
	IMDBID      string  `json:"imdb_id"`
	ExternalIDs *struct {
		IMDBID string `json:"imdb_id"`
		TVDBID int64  `json:"tvdb_id"`
	} `json:"external_ids"`
}

type TMDBTVDetails struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	FirstAirDate string  `json:"first_air_date"`
	PosterPath   *string `json:"poster_path"`
	Seasons      []struct {
		SeasonNumber int `json:"season_number"`
		EpisodeCount int `json:"episode_count"`
	} `json:"seasons"`
	ExternalIDs *struct {
		IMDBID string `json:"imdb_id"`
		TVDBID int64  `json:"tvdb_id"`
	} `json:"external_ids"`
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
	ISO639 string `json:"iso_639_1"`
	Data   struct {
		Title string `json:"title"` // movies
		Name  string `json:"name"`  // TV shows
	} `json:"data"`
}

type tmdbTranslationsResponse struct {
	Translations []TMDBTranslation `json:"translations"`
}

func (c *TMDBClient) GetMovieDetails(tmdbID int64) (*TMDBMovieDetails, error) {
	var details TMDBMovieDetails
	params := url.Values{"append_to_response": {"external_ids"}}
	if err := c.get(fmt.Sprintf("/movie/%d", tmdbID), params, &details); err != nil {
		return nil, fmt.Errorf("get movie details: %w", err)
	}
	return &details, nil
}

func (c *TMDBClient) GetTVDetails(tmdbID int64) (*TMDBTVDetails, error) {
	var details TMDBTVDetails
	params := url.Values{"append_to_response": {"external_ids"}}
	if err := c.get(fmt.Sprintf("/tv/%d", tmdbID), params, &details); err != nil {
		return nil, fmt.Errorf("get tv details: %w", err)
	}
	return &details, nil
}

func (c *TMDBClient) GetTVSeasonEpisodes(tmdbID int64, seasonNumber int) ([]TMDBEpisode, error) {
	var resp tmdbSeasonResponse
	if err := c.get(fmt.Sprintf("/tv/%d/season/%d", tmdbID, seasonNumber), nil, &resp); err != nil {
		return nil, fmt.Errorf("get season episodes: %w", err)
	}
	return resp.Episodes, nil
}

// GetTitleNames returns multilingual names for a title (en, fr).
// mediaType should be "movie" or "tv".
func (c *TMDBClient) GetTitleNames(tmdbID int64, mediaType string) (map[string]string, error) {
	var resp tmdbTranslationsResponse
	if err := c.get(fmt.Sprintf("/%s/%d/translations", mediaType, tmdbID), nil, &resp); err != nil {
		return nil, fmt.Errorf("get translations: %w", err)
	}

	names := make(map[string]string)
	for _, t := range resp.Translations {
		name := t.Data.Title
		if name == "" {
			name = t.Data.Name
		}
		if name != "" && (t.ISO639 == "en" || t.ISO639 == "fr") {
			names[t.ISO639] = name
		}
	}
	return names, nil
}
