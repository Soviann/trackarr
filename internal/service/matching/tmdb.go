package matching

import (
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

func (c *TMDBClient) get(path string, params url.Values, dest interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("api_key", c.apiKey)

	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, path, params.Encode())
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TMDB API error %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}
