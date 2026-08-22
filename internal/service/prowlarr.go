package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2"
	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type ProwlarrRelease struct {
	GUID            string    `json:"guid"`
	Title           string    `json:"title"`
	CleanTitle      string    `json:"clean_title"`
	Year            int       `json:"year"`
	Type            string    `json:"type"` // "movie" or "series"
	Size            int64     `json:"size"`
	PublishDate     time.Time `json:"publish_date"`
	Seeders         int       `json:"seeders"`
	Leechers        int       `json:"leechers"`
	Grabs           int       `json:"grabs"`
	Indexer         string    `json:"indexer"`
	IndexerID       int       `json:"indexer_id"`
	DownloadURL     string    `json:"download_url"`
	InfoURL         string    `json:"info_url"`
	TMDBID          int64     `json:"tmdb_id"`
	IMDBID          string    `json:"imdb_id"`
	PosterURL       string    `json:"poster_url"`
	ExistingTitleID *int64    `json:"existing_title_id,omitempty"`
	ExistingStatus  *string   `json:"existing_status,omitempty"`
}

type prowlarrRawItem struct {
	GUID        string    `json:"guid"`
	Title       string    `json:"title"`
	Size        int64     `json:"size"`
	PublishDate time.Time `json:"publishDate"`
	Seeders     int       `json:"seeders"`
	Leechers    int       `json:"leechers"`
	Grabs       int       `json:"grabs"`
	Indexer     string    `json:"indexer"`
	IndexerID   int       `json:"indexerId"`
	DownloadURL string    `json:"downloadUrl"`
	InfoURL     string    `json:"infoUrl"`
	IMDBID      any       `json:"imdbId"`
	TMDBID      any       `json:"tmdbId"`
	Categories  []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"categories"`
}

type prowlarrCacheEntry struct {
	releases  []ProwlarrRelease
	fetchedAt time.Time
}

type ProwlarrService struct {
	cfg         *config.Config
	settings    *repository.SettingRepository
	titlesRepo  *repository.TitleRepository
	tmdbClient  *matching.TMDBClient
	httpClient  *http.Client
	cacheTTL    time.Duration
	mu          sync.RWMutex
	cache       map[string]prowlarrCacheEntry
	posterCache *lru.Cache[string, string]
}

func NewProwlarrService(cfg *config.Config, settings *repository.SettingRepository, titlesRepo *repository.TitleRepository, tmdbClient *matching.TMDBClient) *ProwlarrService {
	posterCache, _ := lru.New[string, string](1000)
	return &ProwlarrService{
		cfg:        cfg,
		settings:   settings,
		titlesRepo: titlesRepo,
		tmdbClient: tmdbClient,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		cacheTTL:    10 * time.Minute,
		cache:       make(map[string]prowlarrCacheEntry),
		posterCache: posterCache,
	}
}

func (s *ProwlarrService) getProwlarrConfig() (baseURL string, apiKey string) {
	baseURL = s.cfg.ProwlarrURL
	apiKey = s.cfg.ProwlarrAPIKey
	if baseURL == "" && s.settings != nil {
		baseURL, _ = s.settings.Get("prowlarr_url")
	}
	if apiKey == "" && s.settings != nil {
		apiKey, _ = s.settings.Get("prowlarr_api_key")
	}
	return strings.TrimSpace(baseURL), strings.TrimSpace(apiKey)
}

// GetReleases fetches releases from Prowlarr filtered by type ("all", "movie", "series").
func (s *ProwlarrService) GetReleases(ctx context.Context, releaseType string, forceRefresh bool) ([]ProwlarrRelease, error) {
	releaseType = strings.ToLower(strings.TrimSpace(releaseType))
	if releaseType == "" {
		releaseType = "all"
	}

	// Check in-memory cache
	if !forceRefresh {
		s.mu.RLock()
		cached, exists := s.cache[releaseType]
		s.mu.RUnlock()
		if exists && time.Since(cached.fetchedAt) < s.cacheTTL {
			// Update existing status dynamically from DB in case user added/updated items
			return s.enrichWithLocalDB(ctx, cached.releases), nil
		}
	}

	baseURL, apiKey := s.getProwlarrConfig()
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("prowlarr URL or API key not configured")
	}

	baseURL = normalizeBaseURL(baseURL)

	// Torznab standard categories:
	// Movies: 2000, 2010, 2020, 2030, 2040, 2045, 2050, 2060, 2070, 2080, 2090
	// TV: 5000, 5010, 5020, 5030, 5040, 5060, 5070, 5080
	var cats []string
	switch releaseType {
	case "movie":
		cats = []string{"2000", "2010", "2020", "2030", "2040", "2045", "2050", "2060", "2070", "2080", "2090"}
	case "series":
		cats = []string{"5000", "5010", "5020", "5030", "5040", "5060", "5070", "5080"}
	default:
		cats = []string{"2000", "2010", "2020", "2030", "2040", "2045", "2050", "2060", "2070", "2080", "2090", "5000", "5010", "5020", "5030", "5040", "5060", "5070", "5080"}
	}

	queryParams := url.Values{}
	queryParams.Set("type", "search")
	queryParams.Set("limit", "60")
	for _, c := range cats {
		queryParams.Add("categories", c)
	}

	reqURL := fmt.Sprintf("%s/api/v1/search?%s", strings.TrimRight(baseURL, "/"), queryParams.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create prowlarr request: %w", err)
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute prowlarr request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prowlarr API returned status %d: %s", resp.StatusCode, string(body))
	}

	var rawItems []prowlarrRawItem
	if err := json.NewDecoder(resp.Body).Decode(&rawItems); err != nil {
		return nil, fmt.Errorf("decode prowlarr response: %w", err)
	}

	var releases []ProwlarrRelease
	for _, raw := range rawItems {
		rel := s.convertRawItem(raw)
		if releaseType == "movie" && rel.Type != "movie" {
			continue
		}
		if releaseType == "series" && rel.Type != "series" {
			continue
		}
		releases = append(releases, rel)
	}

	// Enrich with TMDB posters and existing titles
	releases = s.enrichPosters(ctx, releases)
	releases = s.enrichWithLocalDB(ctx, releases)

	// Save in cache
	s.mu.Lock()
	s.cache[releaseType] = prowlarrCacheEntry{
		releases:  releases,
		fetchedAt: time.Now(),
	}
	s.mu.Unlock()

	return releases, nil
}

func (s *ProwlarrService) convertRawItem(raw prowlarrRawItem) ProwlarrRelease {
	cleanTitle, year, detectedType := ParseReleaseTitle(raw.Title)

	// Determine type based on categories if available, else detectedType
	relType := detectedType
	for _, c := range raw.Categories {
		if c.ID >= 2000 && c.ID < 3000 {
			relType = "movie"
			break
		} else if c.ID >= 5000 && c.ID < 6000 {
			relType = "series"
			break
		}
	}

	var tmdbID int64
	if raw.TMDBID != nil {
		switch v := raw.TMDBID.(type) {
		case float64:
			tmdbID = int64(v)
		case int:
			tmdbID = int64(v)
		case string:
			tmdbID, _ = strconv.ParseInt(v, 10, 64)
		}
	}

	var imdbID string
	if raw.IMDBID != nil {
		switch v := raw.IMDBID.(type) {
		case string:
			imdbID = v
			if !strings.HasPrefix(imdbID, "tt") && imdbID != "" {
				imdbID = "tt" + imdbID
			}
		case float64:
			if v > 0 {
				imdbID = fmt.Sprintf("tt%07d", int(v))
			}
		case int:
			if v > 0 {
				imdbID = fmt.Sprintf("tt%07d", v)
			}
		}
	}

	return ProwlarrRelease{
		GUID:        raw.GUID,
		Title:       raw.Title,
		CleanTitle:  cleanTitle,
		Year:        year,
		Type:        relType,
		Size:        raw.Size,
		PublishDate: raw.PublishDate,
		Seeders:     raw.Seeders,
		Leechers:    raw.Leechers,
		Grabs:       raw.Grabs,
		Indexer:     raw.Indexer,
		IndexerID:   raw.IndexerID,
		DownloadURL: raw.DownloadURL,
		InfoURL:     raw.InfoURL,
		TMDBID:      tmdbID,
		IMDBID:      imdbID,
	}
}

func (s *ProwlarrService) enrichPosters(ctx context.Context, releases []ProwlarrRelease) []ProwlarrRelease {
	if s.tmdbClient == nil {
		return releases
	}

	// Gather unique tmdb IDs that need posters
	type lookupKey struct {
		tmdbID  int64
		relType string
	}
	needed := make(map[lookupKey]bool)

	for _, r := range releases {
		if r.TMDBID > 0 {
			cacheKey := fmt.Sprintf("%s:%d", r.Type, r.TMDBID)
			if s.posterCache == nil || !s.posterCache.Contains(cacheKey) {
				needed[lookupKey{tmdbID: r.TMDBID, relType: r.Type}] = true
			}
		}
	}

	if len(needed) > 0 {
		// Bounded worker pool (max 5 parallel requests)
		sem := make(chan struct{}, 5)
		var wg sync.WaitGroup

		for k := range needed {
			wg.Add(1)
			go func(key lookupKey) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				cacheKey := fmt.Sprintf("%s:%d", key.relType, key.tmdbID)
				var posterPath *string

				if key.relType == "series" {
					details, err := s.tmdbClient.GetTVDetails(ctx, key.tmdbID)
					if err == nil && details != nil && details.PosterPath != nil {
						posterPath = details.PosterPath
					}
				} else {
					details, err := s.tmdbClient.GetMovieDetails(ctx, key.tmdbID)
					if err == nil && details != nil && details.PosterPath != nil {
						posterPath = details.PosterPath
					}
				}

				if posterPath != nil && *posterPath != "" {
					fullURL := "https://image.tmdb.org/t/p/w500" + *posterPath
					if s.posterCache != nil {
						s.posterCache.Add(cacheKey, fullURL)
					}
				}
			}(k)
		}
		wg.Wait()
	}

	// Apply cached poster URLs to releases
	for i := range releases {
		if releases[i].TMDBID > 0 {
			cacheKey := fmt.Sprintf("%s:%d", releases[i].Type, releases[i].TMDBID)
			if s.posterCache != nil {
				if val, ok := s.posterCache.Get(cacheKey); ok {
					releases[i].PosterURL = val
				}
			}
		}
	}

	return releases
}

func (s *ProwlarrService) enrichWithLocalDB(ctx context.Context, releases []ProwlarrRelease) []ProwlarrRelease {
	if s.titlesRepo == nil {
		return releases
	}

	result := make([]ProwlarrRelease, len(releases))
	copy(result, releases)

	for i := range result {
		var title *model.Title
		if result[i].TMDBID > 0 {
			tType := model.TitleType(result[i].Type)
			t, _ := s.titlesRepo.FindByExternalID(nil, &result[i].TMDBID, nil, nil, &tType)
			if t != nil {
				title = t
			}
		}
		if title == nil && result[i].IMDBID != "" {
			t, _ := s.titlesRepo.FindByExternalID(&result[i].IMDBID, nil, nil, nil, nil)
			if t != nil {
				title = t
			}
		}

		if title != nil {
			id := title.ID
			st := string(title.Status)
			result[i].ExistingTitleID = &id
			result[i].ExistingStatus = &st
			if result[i].PosterURL == "" && title.CoverURL != nil && *title.CoverURL != "" {
				result[i].PosterURL = *title.CoverURL
			}
		}
	}

	return result
}

var (
	yearRegex     = regexp.MustCompile(`(?i)\b(19\d\d|20\d\d)\b`)
	seasonRegex   = regexp.MustCompile(`(?i)\bS(\d{1,2})(?:E\d{1,3})?\b`)
	seasonFrRegex = regexp.MustCompile(`(?i)\bSaison\s*(\d{1,2})\b`)
	stripNoise    = regexp.MustCompile(`(?i)\b(1080p|720p|2160p|4k|uhd|bluray|blu-ray|bdrip|brrip|web-dl|webrip|web|hdrip|hdtv|multi|vostfr|subfrench|french|truefrench|vff|vfq|ac3|dts|ddp|dts-hd|eac3|aac|x264|x265|hevc|h264|h265|avc|remux|hdr|hdr10|dv|dolby\s*vision|proper|repack|complete|integral|integrale|pack)\b.*$`)
)

// ParseReleaseTitle extracts a clean title, year and title type from a raw torrent release name.
func ParseReleaseTitle(raw string) (cleanTitle string, year int, titleType string) {
	titleType = "movie"
	s := strings.ReplaceAll(raw, ".", " ")
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.TrimSpace(s)

	if seasonRegex.MatchString(s) || seasonFrRegex.MatchString(s) {
		titleType = "series"
	}

	// Look for year
	if match := yearRegex.FindStringSubmatch(s); len(match) > 1 {
		year, _ = strconv.Atoi(match[1])
	}

	// Cut off at year, season, or noise tags
	cutIdx := len(s)
	if loc := seasonRegex.FindStringIndex(s); len(loc) > 0 && loc[0] < cutIdx {
		cutIdx = loc[0]
	}
	if loc := seasonFrRegex.FindStringIndex(s); len(loc) > 0 && loc[0] < cutIdx {
		cutIdx = loc[0]
	}
	if loc := yearRegex.FindStringIndex(s); len(loc) > 0 && loc[0] < cutIdx {
		cutIdx = loc[0]
	}
	if loc := stripNoise.FindStringIndex(s); len(loc) > 0 && loc[0] < cutIdx {
		cutIdx = loc[0]
	}

	cleanTitle = strings.TrimSpace(s[:cutIdx])
	cleanTitle = strings.Trim(cleanTitle, "-: ")
	if cleanTitle == "" {
		cleanTitle = raw
	}

	return cleanTitle, year, titleType
}
