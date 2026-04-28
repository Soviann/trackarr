package matching

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	tmdbBaseURL  = "https://api.themoviedb.org/3"
	tmdbImageURL = "https://image.tmdb.org/t/p/w500"
)

type TMDBClient struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string // overridable for tests
}

func NewTMDBClient(apiKey string) *TMDBClient {
	return &TMDBClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: tmdbBaseURL,
	}
}

// SetBaseURL overrides the TMDB base URL (for tests).
func (c *TMDBClient) SetBaseURL(u string) { c.baseURL = u }

func (c *TMDBClient) get(ctx context.Context, path string, params url.Values, dest any) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("api_key", c.apiKey)

	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, path, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("TMDB: read error response: %w", err)
		}
		return newAPIError("TMDB", resp, body)
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}
