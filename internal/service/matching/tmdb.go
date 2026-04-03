package matching

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
	ISO639  string `json:"iso_639_1"`
	Data    struct {
		Title string `json:"title"` // movies
		Name  string `json:"name"`  // TV shows
	} `json:"data"`
}

type tmdbTranslationsResponse struct {
	Translations []TMDBTranslation `json:"translations"`
}

func (c *TMDBClient) SearchMovie(title string, year int) ([]TMDBSearchResult, error) {
	params := url.Values{
		"query": {title},
	}
	if year > 0 {
		params.Set("year", strconv.Itoa(year))
	}

	var resp tmdbSearchResponse
	if err := c.get("/search/movie", params, &resp); err != nil {
		return nil, fmt.Errorf("search movie: %w", err)
	}
	return resp.Results, nil
}

func (c *TMDBClient) SearchTV(title string, year int) ([]TMDBSearchResult, error) {
	params := url.Values{
		"query": {title},
	}
	if year > 0 {
		params.Set("first_air_date_year", strconv.Itoa(year))
	}

	var resp tmdbSearchResponse
	if err := c.get("/search/tv", params, &resp); err != nil {
		return nil, fmt.Errorf("search tv: %w", err)
	}
	return resp.Results, nil
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

// DownloadCover downloads a poster from TMDB and saves it to destDir.
// Returns the local filename (not the full path).
func (c *TMDBClient) DownloadCover(posterPath string, destDir string) (string, error) {
	if posterPath == "" {
		return "", fmt.Errorf("empty poster path")
	}

	imageURL := tmdbImageURL + posterPath
	if c.baseURL != tmdbBaseURL {
		// Test mode: use baseURL for image requests too
		imageURL = c.baseURL + "/image" + posterPath
	}

	resp, err := c.httpClient.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("download cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download cover: status %d", resp.StatusCode)
	}

	filename := filepath.Base(posterPath)
	destPath := filepath.Join(destDir, filename)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create cover dir: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create cover file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write cover: %w", err)
	}

	return filename, nil
}

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
