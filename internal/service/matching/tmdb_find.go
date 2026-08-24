package matching

import (
	"context"
	"fmt"
	"net/url"
)

type tmdbFindResponse struct {
	MovieResults []TMDBSearchResult `json:"movie_results"`
	TVResults    []TMDBSearchResult `json:"tv_results"`
}

// FindByID looks up a title on TMDB by an external ID (e.g. IMDb ID).
// Returns the result and its media type ("movie" or "tv").
func (c *TMDBClient) FindByID(ctx context.Context, externalID string, source string) (*TMDBSearchResult, string, error) {
	params := url.Values{
		"external_source": {source},
	}

	var resp tmdbFindResponse
	if err := c.get(ctx, fmt.Sprintf("/find/%s", externalID), params, &resp); err != nil {
		return nil, "", fmt.Errorf("find by id: %w", err)
	}

	if len(resp.MovieResults) > 0 {
		return &resp.MovieResults[0], "movie", nil
	}
	if len(resp.TVResults) > 0 {
		return &resp.TVResults[0], "tv", nil
	}

	return nil, "", nil
}
