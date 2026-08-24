package matching

import (
	"context"
	"fmt"
	"net/url"
)

type tvdbSearchResult struct {
	ObjectID string  `json:"objectID"`
	Type     string  `json:"type"` // "series" or "movie"
	Name     string  `json:"name"`
	Year     string  `json:"year"`
	Image    string  `json:"image_url"`
	TVDBID   int64   `json:"tvdb_id"`
	Score    float64 `json:"score"`
}

type tvdbSearchResponse struct {
	Data []tvdbSearchResult `json:"data"`
}

// SearchSeries searches TVDB for series by title.
func (c *TVDBClient) SearchSeries(ctx context.Context, title string, year int) ([]tvdbSearchResult, error) {
	params := url.Values{
		"query": {title},
		"type":  {"series"},
	}
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}
	var resp tvdbSearchResponse
	if err := c.get(ctx, "/search", params, &resp); err != nil {
		return nil, fmt.Errorf("tvdb search series: %w", err)
	}
	return resp.Data, nil
}

// SearchMovies searches TVDB for movies by title.
func (c *TVDBClient) SearchMovies(ctx context.Context, title string, year int) ([]tvdbSearchResult, error) {
	params := url.Values{
		"query": {title},
		"type":  {"movie"},
	}
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}
	var resp tvdbSearchResponse
	if err := c.get(ctx, "/search", params, &resp); err != nil {
		return nil, fmt.Errorf("tvdb search movies: %w", err)
	}
	return resp.Data, nil
}
