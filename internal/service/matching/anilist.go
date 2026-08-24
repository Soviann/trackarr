package matching

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const anilistAPIURL = "https://graphql.anilist.co"

type AniListClient struct {
	httpClient *http.Client
	apiURL     string // overridable for tests
}

func NewAniListClient() *AniListClient {
	return &AniListClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiURL:     anilistAPIURL,
	}
}

// NewAniListClientWithURL builds a client pointed at a custom endpoint —
// used by tests with httptest servers.
func NewAniListClientWithURL(url string) *AniListClient {
	c := NewAniListClient()
	c.apiURL = url
	return c
}

// TokenInvalidError is returned when AniList rejects the OAuth token (HTTP 401).
// The caller should flag the token as invalid rather than retry the request.
type TokenInvalidError struct{}

func (TokenInvalidError) Error() string { return "anilist: token invalid (401)" }

// queryAuthenticated posts a GraphQL request with a Bearer token and returns
// TokenInvalidError on 401. It does not decode the response — mutations here
// only need success/failure signalling.
func (c *AniListClient) queryAuthenticated(ctx context.Context, gql string, variables map[string]any, accessToken string) error {
	body, err := json.Marshal(graphqlRequest{
		Query:     gql,
		Variables: variables,
	})
	if err != nil {
		return fmt.Errorf("marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return TokenInvalidError{}
	}
	if resp.StatusCode >= 400 {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("AniList: read error response: %w", err)
		}
		return newAPIError("AniList", resp, respBody)
	}
	return nil
}

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *AniListClient) query(ctx context.Context, gql string, variables map[string]any, accessToken string, dest any) error {
	body, err := json.Marshal(graphqlRequest{
		Query:     gql,
		Variables: variables,
	})
	if err != nil {
		return fmt.Errorf("marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("AniList: read error response: %w", err)
		}
		return newAPIError("AniList", resp, respBody)
	}

	var gqlResp graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("AniList GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	return json.Unmarshal(gqlResp.Data, dest)
}
