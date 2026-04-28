package matching

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const tvdbBaseURL = "https://api4.thetvdb.com/v4"

// TVDBClient is a client for the TheTVDB v4 API.
// It caches the JWT token in memory and refreshes it on 401.
type TVDBClient struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string // overridable for tests

	mu    sync.Mutex
	token string
}

// NewTVDBClient creates a new TVDBClient.
func NewTVDBClient(apiKey string) *TVDBClient {
	return &TVDBClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: tvdbBaseURL,
	}
}

// SetBaseURL overrides the TVDB base URL (for tests).
func (c *TVDBClient) SetBaseURL(u string) { c.baseURL = u }

// Login authenticates with the TVDB API and caches the JWT token.
func (c *TVDBClient) Login(ctx context.Context) error {
	body, err := json.Marshal(map[string]string{"apikey": c.apiKey})
	if err != nil {
		return fmt.Errorf("tvdb login: marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("tvdb login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return newAPIError("TVDB", resp, b)
	}

	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("tvdb login decode: %w", err)
	}
	c.mu.Lock()
	c.token = result.Data.Token
	c.mu.Unlock()
	return nil
}

// get performs an authenticated GET request, re-logging in once on 401.
func (c *TVDBClient) get(ctx context.Context, path string, params url.Values, dest interface{}) error {
	if err := c.doGet(ctx, path, params, dest); err != nil {
		if isUnauthorized(err) {
			// Re-login and retry once
			if loginErr := c.Login(ctx); loginErr != nil {
				return fmt.Errorf("tvdb re-login: %w", loginErr)
			}
			return c.doGet(ctx, path, params, dest)
		}
		return err
	}
	return nil
}

func (c *TVDBClient) doGet(ctx context.Context, path string, params url.Values, dest interface{}) error {
	reqURL := fmt.Sprintf("%s%s", c.baseURL, path)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return newAPIError("TVDB", resp, b)
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}

func isUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized
}
