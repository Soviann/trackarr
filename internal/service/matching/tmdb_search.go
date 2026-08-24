package matching

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// TMDBSearchResult represents a single search result from TMDB.
type TMDBSearchResult struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`        // movies
	Name         string  `json:"name"`         // TV shows
	ReleaseDate  string  `json:"release_date"` // movies
	FirstAirDate string  `json:"first_air_date"`
	PosterPath   *string `json:"poster_path"`
	Overview     string  `json:"overview"`
}

func (r TMDBSearchResult) DisplayTitle() string {
	if r.Title != "" {
		return r.Title
	}
	return r.Name
}

func (r TMDBSearchResult) Year() int {
	date := r.ReleaseDate
	if date == "" {
		date = r.FirstAirDate
	}
	if len(date) >= 4 {
		y, _ := strconv.Atoi(date[:4])
		return y
	}
	return 0
}

type tmdbSearchResponse struct {
	Results    []TMDBSearchResult `json:"results"`
	TotalPages int                `json:"total_pages"`
}

func (c *TMDBClient) SearchMovie(ctx context.Context, title string, year int) ([]TMDBSearchResult, error) {
	params := url.Values{
		"query": {title},
	}
	if year > 0 {
		params.Set("year", strconv.Itoa(year))
	}

	var resp tmdbSearchResponse
	if err := c.get(ctx, "/search/movie", params, &resp); err != nil {
		return nil, fmt.Errorf("search movie: %w", err)
	}
	return resp.Results, nil
}

func (c *TMDBClient) SearchTV(ctx context.Context, title string, year int) ([]TMDBSearchResult, error) {
	params := url.Values{
		"query": {title},
	}
	if year > 0 {
		params.Set("first_air_date_year", strconv.Itoa(year))
	}

	var resp tmdbSearchResponse
	if err := c.get(ctx, "/search/tv", params, &resp); err != nil {
		return nil, fmt.Errorf("search tv: %w", err)
	}
	return resp.Results, nil
}
