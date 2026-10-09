package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/version"
)

// Client handles HTTP communication with the Trackarr REST API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient initializes a Trackarr API client with the given base URL and API key.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) do(ctx context.Context, method, endpoint string, body any, target any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	reqURL := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "trackarr-mcp/"+version.Version)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
			Code    string `json:"code"`
		}
		if jsonErr := json.Unmarshal(respBytes, &errResp); jsonErr == nil && (errResp.Error != "" || errResp.Message != "") {
			errMsg := errResp.Message
			if errMsg == "" {
				errMsg = errResp.Error
			}
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errMsg)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	if target != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, target); err != nil {
			return fmt.Errorf("unmarshal response body: %w", err)
		}
	}
	return nil
}

// CompactTitle represents a token-compacted summary of a title for LLM context efficiency.
type CompactTitle struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Year      int     `json:"year,omitempty"`
	Status    string  `json:"status"`
	MyRating  *int    `json:"my_rating,omitempty"`
	CoverURL  *string `json:"cover_url,omitempty"`
	IMDBID    *string `json:"imdb_id,omitempty"`
	TMDBID    *int64  `json:"tmdb_id,omitempty"`
	TVDBID    *int64  `json:"tvdb_id,omitempty"`
	AniListID *int64  `json:"anilist_id,omitempty"`
	Progress  string  `json:"progress,omitempty"`
}

// RawTitle matches the JSON returned by /api/titles endpoints.
type RawTitle struct {
	ID              int64          `json:"id"`
	Type            string         `json:"type"`
	IsAnime         bool           `json:"is_anime"`
	Year            int            `json:"year"`
	Status          string         `json:"status"`
	MyRating        *int           `json:"my_rating"`
	PersonalNotes   *string        `json:"personal_notes"`
	Overview        *string        `json:"overview"`
	Genres          *string        `json:"genres"`
	CoverURL        *string        `json:"cover_url"`
	IMDBID          *string        `json:"imdb_id"`
	TMDBID          *int64         `json:"tmdb_id"`
	TVDBID          *int64         `json:"tvdb_id"`
	AniListID       *int64         `json:"anilist_id"`
	Names           []RawTitleName `json:"names"`
	Seasons         []RawSeason    `json:"seasons"`
	WatchedEpisodes int            `json:"watched_episodes"`
	TotalEpisodes   int            `json:"total_episodes"`
}

type RawTitleName struct {
	Name      string `json:"name"`
	Language  string `json:"language"`
	IsPrimary bool   `json:"is_primary"`
}

type RawSeason struct {
	ID           int64        `json:"id"`
	SeasonNumber int          `json:"season_number"`
	Name         *string      `json:"name"`
	Episodes     []RawEpisode `json:"episodes"`
}

type RawEpisode struct {
	ID            int64   `json:"id"`
	EpisodeNumber int     `json:"episode_number"`
	Name          *string `json:"name"`
	AirDate       *string `json:"air_date"`
	Watched       bool    `json:"watched"`
}

// TitleDetail is a formatted title detail for deep inspection.
type TitleDetail struct {
	ID            int64           `json:"id"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	Year          int             `json:"year,omitempty"`
	Status        string          `json:"status"`
	MyRating      *int            `json:"my_rating,omitempty"`
	PersonalNotes *string         `json:"personal_notes,omitempty"`
	Overview      *string         `json:"overview,omitempty"`
	Genres        *string         `json:"genres,omitempty"`
	CoverURL      *string         `json:"cover_url,omitempty"`
	ExternalIDs   map[string]any  `json:"external_ids,omitempty"`
	Aliases       []string        `json:"aliases,omitempty"`
	Progress      string          `json:"progress,omitempty"`
	Seasons       []CompactSeason `json:"seasons,omitempty"`
}

type CompactSeason struct {
	SeasonNumber    int              `json:"season_number"`
	Name            *string          `json:"name,omitempty"`
	TotalEpisodes   int              `json:"total_episodes"`
	WatchedEpisodes int              `json:"watched_episodes"`
	Episodes        []CompactEpisode `json:"episodes,omitempty"`
}

type CompactEpisode struct {
	ID            int64   `json:"id"`
	EpisodeNumber int     `json:"episode_number"`
	Name          *string `json:"name,omitempty"`
	AirDate       *string `json:"air_date,omitempty"`
	Watched       bool    `json:"watched"`
}

func (r *RawTitle) PrimaryName() string {
	for _, n := range r.Names {
		if n.IsPrimary {
			return n.Name
		}
	}
	if len(r.Names) > 0 {
		return r.Names[0].Name
	}
	return "Untitled"
}

func (r *RawTitle) ToCompact() CompactTitle {
	c := CompactTitle{
		ID:        r.ID,
		Name:      r.PrimaryName(),
		Type:      r.Type,
		Year:      r.Year,
		Status:    r.Status,
		MyRating:  r.MyRating,
		CoverURL:  r.CoverURL,
		IMDBID:    r.IMDBID,
		TMDBID:    r.TMDBID,
		TVDBID:    r.TVDBID,
		AniListID: r.AniListID,
	}
	if r.Type == "series" && r.TotalEpisodes > 0 {
		c.Progress = fmt.Sprintf("%d/%d episodes", r.WatchedEpisodes, r.TotalEpisodes)
	}
	return c
}

func (r *RawTitle) ToDetail() TitleDetail {
	ext := make(map[string]any)
	if r.IMDBID != nil && *r.IMDBID != "" {
		ext["imdb_id"] = *r.IMDBID
	}
	if r.TMDBID != nil && *r.TMDBID != 0 {
		ext["tmdb_id"] = *r.TMDBID
	}
	if r.TVDBID != nil && *r.TVDBID != 0 {
		ext["tvdb_id"] = *r.TVDBID
	}
	if r.AniListID != nil && *r.AniListID != 0 {
		ext["anilist_id"] = *r.AniListID
	}

	var aliases []string
	primary := r.PrimaryName()
	for _, n := range r.Names {
		if n.Name != primary {
			aliases = append(aliases, n.Name)
		}
	}

	var seasons []CompactSeason
	for _, s := range r.Seasons {
		cs := CompactSeason{
			SeasonNumber:  s.SeasonNumber,
			Name:          s.Name,
			TotalEpisodes: len(s.Episodes),
		}
		watchedCount := 0
		for _, ep := range s.Episodes {
			if ep.Watched {
				watchedCount++
			}
			cs.Episodes = append(cs.Episodes, CompactEpisode(ep))
		}
		cs.WatchedEpisodes = watchedCount
		seasons = append(seasons, cs)
	}

	progress := ""
	if r.Type == "series" && r.TotalEpisodes > 0 {
		progress = fmt.Sprintf("%d/%d episodes watched", r.WatchedEpisodes, r.TotalEpisodes)
	}

	return TitleDetail{
		ID:            r.ID,
		Name:          primary,
		Type:          r.Type,
		Year:          r.Year,
		Status:        r.Status,
		MyRating:      r.MyRating,
		PersonalNotes: r.PersonalNotes,
		Overview:      r.Overview,
		Genres:        r.Genres,
		CoverURL:      r.CoverURL,
		ExternalIDs:   ext,
		Aliases:       aliases,
		Progress:      progress,
		Seasons:       seasons,
	}
}

// SearchResult wraps list response from Trackarr.
type SearchResult struct {
	Titles []CompactTitle `json:"titles"`
	Total  int            `json:"total"`
}

// SearchTitles queries titles with optional filtering and pagination.
func (c *Client) SearchTitles(ctx context.Context, query, titleType, status string, limit int) (*SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	if query != "" {
		params.Set("search", query)
	}
	if titleType != "" {
		params.Set("type", titleType)
	}
	if status != "" {
		params.Set("status", status)
	}
	params.Set("limit", strconv.Itoa(limit))

	endpoint := "/api/titles?" + params.Encode()

	var raw struct {
		Titles []RawTitle `json:"titles"`
		Total  int        `json:"total"`
	}

	if err := c.do(ctx, http.MethodGet, endpoint, nil, &raw); err != nil {
		return nil, err
	}

	res := &SearchResult{
		Total:  raw.Total,
		Titles: make([]CompactTitle, len(raw.Titles)),
	}
	for i := range raw.Titles {
		res.Titles[i] = raw.Titles[i].ToCompact()
	}
	return res, nil
}

// GetTitle fetches a complete title by its Trackarr ID.
func (c *Client) GetTitle(ctx context.Context, id int64) (*TitleDetail, error) {
	endpoint := fmt.Sprintf("/api/titles/%d", id)
	var raw RawTitle
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &raw); err != nil {
		return nil, err
	}
	detail := raw.ToDetail()
	return &detail, nil
}

// ContinueWatchingItem represents an in-progress series.
type ContinueWatchingItem struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	WatchedEpisodes int     `json:"watched_episodes"`
	TotalEpisodes   int     `json:"total_episodes"`
	LastWatchedAt   *string `json:"last_watched_at,omitempty"`
	NextEpisode     *struct {
		ID            int64   `json:"id"`
		SeasonNumber  int     `json:"season_number"`
		EpisodeNumber int     `json:"episode_number"`
		Name          *string `json:"name,omitempty"`
		AirDate       *string `json:"air_date,omitempty"`
	} `json:"next_episode,omitempty"`
}

// GetContinueWatching returns the list of in-progress series.
func (c *Client) GetContinueWatching(ctx context.Context) ([]ContinueWatchingItem, error) {
	var items []ContinueWatchingItem
	if err := c.do(ctx, http.MethodGet, "/api/titles/continue-watching", nil, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// ResolveResult represents resolution of an external URL.
type ResolveResult struct {
	FoundInLibrary bool          `json:"found_in_library"`
	LibraryTitle   *CompactTitle `json:"library_title,omitempty"`
	ResolvedInfo   any           `json:"resolved_info,omitempty"`
}

// ResolveURL looks up an external URL in the library first, falling back to online pipeline resolution.
func (c *Client) ResolveURL(ctx context.Context, rawURL string) (*ResolveResult, error) {
	// First check if already present in library via search
	searchRes, err := c.SearchTitles(ctx, rawURL, "", "", 1)
	if err == nil && len(searchRes.Titles) > 0 {
		return &ResolveResult{
			FoundInLibrary: true,
			LibraryTitle:   &searchRes.Titles[0],
		}, nil
	}

	// Not in library; resolve online via pipeline endpoint
	params := url.Values{}
	params.Set("q", rawURL)
	endpoint := "/api/titles/resolve?" + params.Encode()

	var resolved any
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &resolved); err != nil {
		return nil, err
	}

	return &ResolveResult{
		FoundInLibrary: false,
		ResolvedInfo:   resolved,
	}, nil
}

// GetStats returns summary library statistics.
func (c *Client) GetStats(ctx context.Context) (map[string]any, error) {
	var stats map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/stats", nil, &stats); err != nil {
		return nil, err
	}
	return stats, nil
}

// BatchCreateItem represents one item to ingest.
type BatchCreateItem struct {
	Title     string  `json:"title"`
	Type      string  `json:"type,omitempty"`
	Year      int     `json:"year,omitempty"`
	URL       string  `json:"url,omitempty"`
	IMDBID    *string `json:"imdb_id,omitempty"`
	TMDBID    *int64  `json:"tmdb_id,omitempty"`
	TVDBID    *int64  `json:"tvdb_id,omitempty"`
	AniListID *int64  `json:"anilist_id,omitempty"`
	IsAnime   bool    `json:"is_anime,omitempty"`
	Status    string  `json:"status,omitempty"`
}

// BatchCreateResult represents the response of /api/titles/batch.
type BatchCreateResult struct {
	Total    int                     `json:"total"`
	Created  int                     `json:"created"`
	Existing int                     `json:"existing"`
	Failed   int                     `json:"failed"`
	Items    []BatchCreateResultItem `json:"items"`
}

// BatchCreateResultItem is one entry in BatchCreateResult.
type BatchCreateResultItem struct {
	Index   int    `json:"index"`
	TitleID *int64 `json:"title_id,omitempty"`
	Title   string `json:"title,omitempty"`
	Status  string `json:"status"` // "created", "existing", "error"
	Error   string `json:"error,omitempty"`
}

// BatchAddTitles ingests up to 100 titles in a single atomic request.
func (c *Client) BatchAddTitles(ctx context.Context, items []BatchCreateItem) (*BatchCreateResult, error) {
	wrapper := struct {
		Items []BatchCreateItem `json:"items"`
	}{
		Items: items,
	}

	var res BatchCreateResult
	if err := c.do(ctx, http.MethodPost, "/api/titles/batch", wrapper, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// AddTitle adds a single title using the atomic batch ingestion endpoint.
func (c *Client) AddTitle(ctx context.Context, item BatchCreateItem) (*BatchCreateResultItem, error) {
	res, err := c.BatchAddTitles(ctx, []BatchCreateItem{item})
	if err != nil {
		return nil, err
	}
	if len(res.Items) == 0 {
		return nil, fmt.Errorf("server returned empty result")
	}
	return &res.Items[0], nil
}

// TitleUpdatePayload contains fields for title PATCH updates.
type TitleUpdatePayload struct {
	Status        *string `json:"status,omitempty"`
	MyRating      *int    `json:"my_rating,omitempty"`
	PersonalNotes *string `json:"personal_notes,omitempty"`
	IsAnime       *bool   `json:"is_anime,omitempty"`
	ArrIgnored    *bool   `json:"arr_ignored,omitempty"`
}

// UpdateTitle updates properties of an existing title.
func (c *Client) UpdateTitle(ctx context.Context, id int64, payload TitleUpdatePayload) (*CompactTitle, error) {
	endpoint := fmt.Sprintf("/api/titles/%d", id)
	var raw RawTitle
	if err := c.do(ctx, http.MethodPatch, endpoint, payload, &raw); err != nil {
		return nil, err
	}
	compact := raw.ToCompact()
	return &compact, nil
}

// SetEpisodeWatched marks an episode watched or unwatched.
func (c *Client) SetEpisodeWatched(ctx context.Context, titleID, episodeID int64, watched bool) error {
	endpoint := fmt.Sprintf("/api/titles/%d/episodes/batch-watch", titleID)
	payload := struct {
		EpisodeIDs []int64 `json:"episode_ids"`
		Watched    bool    `json:"watched"`
	}{
		EpisodeIDs: []int64{episodeID},
		Watched:    watched,
	}
	return c.do(ctx, http.MethodPost, endpoint, payload, nil)
}

// DeleteTitle removes a title from the library.
func (c *Client) DeleteTitle(ctx context.Context, id int64) error {
	endpoint := fmt.Sprintf("/api/titles/%d", id)
	return c.do(ctx, http.MethodDelete, endpoint, nil, nil)
}

// PushToArr pushes a title to Radarr or Sonarr.
func (c *Client) PushToArr(ctx context.Context, id int64) (int64, error) {
	endpoint := fmt.Sprintf("/api/arr/push/%d", id)
	var res struct {
		Status string `json:"status"`
		ArrID  int64  `json:"arr_id"`
	}
	if err := c.do(ctx, http.MethodPost, endpoint, nil, &res); err != nil {
		return 0, err
	}
	return res.ArrID, nil
}
