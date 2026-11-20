package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

// JellyfinSyncConfig contains parameters for communicating with Jellyfin during rating sync.
type JellyfinSyncConfig struct {
	BaseURL       string
	APIKey        string
	UserID        string
	DryRun        bool
	HTTPClient    *http.Client
	ThrottleDelay time.Duration
}

// JellyfinSyncSummary tracks the outcome of an outbound rating sync run.
type JellyfinSyncSummary struct {
	Matched int
	Updated int
	Skipped int
	Missing int
	Errors  int
}

// JellyfinMovieSyncSummary tracks the outcome of an inbound unwatched movie sync run.
type JellyfinMovieSyncSummary struct {
	DryRun    bool `json:"dry_run"`
	Fetched   int  `json:"fetched"`
	Unwatched int  `json:"unwatched"`
	Created   int  `json:"created"`
	Skipped   int  `json:"skipped"`
	Errors    int  `json:"errors"`
}

// JellyfinUserData represents user-specific rating and playback state returned by Jellyfin.
type JellyfinUserData struct {
	Rating *float64 `json:"Rating,omitempty"`
	Likes  *bool    `json:"Likes,omitempty"`
	Played bool     `json:"Played,omitempty"`
}

// JellyfinLibraryItem represents a movie or series in the user's Jellyfin library.
type JellyfinLibraryItem struct {
	ID             string            `json:"Id"`
	Name           string            `json:"Name"`
	Type           string            `json:"Type"`
	ProductionYear int               `json:"ProductionYear,omitempty"`
	ProviderIds    map[string]string `json:"ProviderIds"`
	UserData       *JellyfinUserData `json:"UserData,omitempty"`
}

type jellyfinItemsResponse struct {
	Items            []JellyfinLibraryItem `json:"Items"`
	TotalRecordCount int                   `json:"TotalRecordCount"`
}

// JellyfinSyncService handles synchronizing ratings and unwatched movies with Jellyfin.
type JellyfinSyncService struct {
	db            *sql.DB
	titles        *repository.TitleRepository
	cfg           JellyfinSyncConfig
	log           *slog.Logger
	limiter       *APILimiter
	throttleDelay time.Duration
	sleepFn       func(time.Duration)
}

// NewJellyfinSyncService creates an outbound Jellyfin rating sync service.
func NewJellyfinSyncService(db *sql.DB, cfg JellyfinSyncConfig, log *slog.Logger) *JellyfinSyncService {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if log == nil {
		log = slog.Default()
	}
	delay := 50 * time.Millisecond
	if cfg.ThrottleDelay > 0 {
		delay = cfg.ThrottleDelay
	} else if cfg.ThrottleDelay < 0 {
		delay = 0
	}
	var titles *repository.TitleRepository
	if db != nil {
		titles = repository.NewTitleRepository(db)
	}
	return &JellyfinSyncService{
		db:            db,
		titles:        titles,
		cfg:           cfg,
		log:           log,
		throttleDelay: delay,
		sleepFn:       time.Sleep,
	}
}

// SetTitleRepo replaces the title repository (useful for testing).
func (s *JellyfinSyncService) SetTitleRepo(repo *repository.TitleRepository) {
	s.titles = repo
}

// SetSleepFn replaces the sleep function used for throttling (useful for testing).
func (s *JellyfinSyncService) SetSleepFn(fn func(time.Duration)) {
	s.sleepFn = fn
}

// SetAPILimiter configures the shared external API rate limiter.
func (s *JellyfinSyncService) SetAPILimiter(limiter *APILimiter) {
	if s == nil || limiter == nil {
		return
	}
	s.limiter = limiter
}

func (s *JellyfinSyncService) resolveConfig() (baseURL, apiKey, userID string) {
	baseURL = strings.TrimRight(strings.TrimSpace(s.cfg.BaseURL), "/")
	apiKey = strings.TrimSpace(s.cfg.APIKey)
	userID = strings.TrimSpace(s.cfg.UserID)

	if (baseURL == "" || apiKey == "" || userID == "") && s.db != nil {
		sr := repository.NewSettingRepository(s.db)
		if baseURL == "" {
			if v, err := sr.Get(repository.SettingKeyJellyfinURL); err == nil {
				baseURL = strings.TrimRight(strings.TrimSpace(v), "/")
			}
		}
		if apiKey == "" {
			if v, err := sr.Get(repository.SettingKeyJellyfinAPIKey); err == nil {
				apiKey = strings.TrimSpace(v)
			}
		}
		if userID == "" {
			if v, err := sr.Get(repository.SettingKeyJellyfinUserID); err == nil {
				userID = strings.TrimSpace(v)
			}
		}
	}
	return baseURL, apiKey, userID
}

func validateJellyfinBaseURL(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", fmt.Errorf("jellyfin base URL is required")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid jellyfin base URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("invalid jellyfin base URL: must use http or https with a non-empty host")
	}
	return trimmed, nil
}

func (s *JellyfinSyncService) fetchItems(ctx context.Context, baseURL, apiKey, userID, includeItemTypes string) ([]JellyfinLibraryItem, error) {
	escapedUserID := url.PathEscape(strings.TrimSpace(userID))
	reqURL := fmt.Sprintf("%s/Users/%s/Items?includeItemTypes=%s&recursive=true&fields=ProviderIds", baseURL, escapedUserID, includeItemTypes)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create fetch items request: %w", err)
	}

	req.Header.Set("X-Emby-Token", apiKey)
	req.Header.Set("Accept", "application/json")

	client := s.cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jellyfin items: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("jellyfin fetch items failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var res jellyfinItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decode jellyfin items response: %w", err)
	}

	return res.Items, nil
}

// FetchUserItems queries Jellyfin for movies and series in the user's library with external provider IDs.
func (s *JellyfinSyncService) FetchUserItems(ctx context.Context) ([]JellyfinLibraryItem, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.BaseURL), "/")
	return s.fetchItems(ctx, baseURL, strings.TrimSpace(s.cfg.APIKey), strings.TrimSpace(s.cfg.UserID), "Movie,Series")
}

// FetchUserMovies queries Jellyfin for movies in the user's library with external provider IDs.
func (s *JellyfinSyncService) FetchUserMovies(ctx context.Context) ([]JellyfinLibraryItem, error) {
	baseURL, apiKey, userID := s.resolveConfig()
	validatedURL, err := validateJellyfinBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if apiKey == "" {
		return nil, fmt.Errorf("jellyfin API key is required")
	}
	if userID == "" {
		return nil, fmt.Errorf("jellyfin user ID is required")
	}
	if s.limiter != nil {
		if err := s.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("jellyfin rate limiter: %w", err)
		}
	}
	return s.fetchItems(ctx, validatedURL, apiKey, userID, "Movie")
}

// MatchItem finds a Jellyfin library item matching a Trackarr title.
// Checks media type (Movie vs Series), then checks TMDB, TVDB, and IMDb IDs with case-insensitive keys.
func (s *JellyfinSyncService) MatchItem(title model.Title, jfItems []JellyfinLibraryItem) *JellyfinLibraryItem {
	var targetType string
	switch title.Type {
	case model.TitleTypeMovie:
		targetType = "Movie"
	case model.TitleTypeSeries:
		targetType = "Series"
	default:
		return nil
	}

	// 1. Match by TMDB ID
	if title.TMDBID != nil && *title.TMDBID > 0 {
		targetTMDB := strconv.FormatInt(*title.TMDBID, 10)
		for i := range jfItems {
			if strings.EqualFold(jfItems[i].Type, targetType) {
				if getJellyfinProvider(jfItems[i].ProviderIds, "Tmdb") == targetTMDB {
					item := jfItems[i]
					return &item
				}
			}
		}
	}

	// 2. Match by TVDB ID
	if title.TVDBID != nil && *title.TVDBID > 0 {
		targetTVDB := strconv.FormatInt(*title.TVDBID, 10)
		for i := range jfItems {
			if strings.EqualFold(jfItems[i].Type, targetType) {
				if getJellyfinProvider(jfItems[i].ProviderIds, "Tvdb") == targetTVDB {
					item := jfItems[i]
					return &item
				}
			}
		}
	}

	// 3. Match by IMDb ID
	if title.IMDBID != nil && strings.TrimSpace(*title.IMDBID) != "" {
		targetIMDB := strings.ToLower(strings.TrimSpace(*title.IMDBID))
		for i := range jfItems {
			if strings.EqualFold(jfItems[i].Type, targetType) {
				providerIMDB := strings.ToLower(getJellyfinProvider(jfItems[i].ProviderIds, "Imdb"))
				if providerIMDB != "" && (providerIMDB == targetIMDB || strings.TrimPrefix(providerIMDB, "tt") == strings.TrimPrefix(targetIMDB, "tt")) {
					item := jfItems[i]
					return &item
				}
			}
		}
	}

	return nil
}

func getJellyfinProvider(ids map[string]string, key string) string {
	for k, v := range ids {
		if strings.EqualFold(k, key) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// PushRating sends a user rating to Jellyfin for a given item.
func (s *JellyfinSyncService) PushRating(ctx context.Context, itemID string, rating int) error {
	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.BaseURL), "/")
	userID := url.PathEscape(strings.TrimSpace(s.cfg.UserID))
	escapedItemID := url.PathEscape(strings.TrimSpace(itemID))
	likes := rating >= 6

	reqURL := fmt.Sprintf("%s/Users/%s/Items/%s/Rating?rating=%d&likes=%t", baseURL, userID, escapedItemID, rating, likes)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create push rating request: %w", err)
	}

	req.Header.Set("X-Emby-Token", strings.TrimSpace(s.cfg.APIKey))
	req.Header.Set("Accept", "application/json")

	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("push rating to jellyfin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("jellyfin push rating failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

// SyncRatings coordinates the entire outbound ratings sync from Trackarr to Jellyfin.
func (s *JellyfinSyncService) SyncRatings(ctx context.Context) (JellyfinSyncSummary, error) {
	var summary JellyfinSyncSummary

	if strings.TrimSpace(s.cfg.BaseURL) == "" || strings.TrimSpace(s.cfg.APIKey) == "" || strings.TrimSpace(s.cfg.UserID) == "" {
		return summary, fmt.Errorf("jellyfin sync: missing required configuration (base URL, API key, or user ID)")
	}

	if s.titles == nil {
		return summary, fmt.Errorf("jellyfin sync: title repository is not initialized")
	}

	ratedTitles, err := s.titles.ListRatedTitles(ctx)
	if err != nil {
		return summary, fmt.Errorf("jellyfin sync: list rated titles: %w", err)
	}

	if len(ratedTitles) == 0 {
		s.log.Info("no rated titles found in Trackarr")
		return summary, nil
	}

	jfItems, err := s.FetchUserItems(ctx)
	if err != nil {
		return summary, fmt.Errorf("jellyfin sync: fetch user items: %w", err)
	}

	for _, title := range ratedTitles {
		if ctx.Err() != nil {
			return summary, ctx.Err()
		}

		if title.MyRating == nil || *title.MyRating <= 0 {
			continue
		}

		targetRating := *title.MyRating
		matched := s.MatchItem(title, jfItems)
		if matched == nil {
			summary.Missing++
			s.log.Debug("no matching Jellyfin item found for rated title",
				"titleID", title.ID,
				"name", title.PrimaryName(),
				"type", title.Type,
			)
			continue
		}

		summary.Matched++

		// If Jellyfin already reflects this exact rating, skip write
		if matched.UserData != nil && matched.UserData.Rating != nil && int(math.Round(*matched.UserData.Rating)) == targetRating {
			summary.Skipped++
			s.log.Debug("rating already up to date in Jellyfin",
				"name", title.PrimaryName(),
				"jellyfinID", matched.ID,
				"rating", targetRating,
			)
			continue
		}

		if s.cfg.DryRun {
			summary.Skipped++
			s.log.Info("[DRY-RUN] would update Jellyfin rating",
				"name", title.PrimaryName(),
				"jellyfinID", matched.ID,
				"rating", targetRating,
				"likes", targetRating >= 6,
			)
			continue
		}

		if err := s.PushRating(ctx, matched.ID, targetRating); err != nil {
			summary.Errors++
			s.log.Error("failed to push rating to Jellyfin",
				"name", title.PrimaryName(),
				"jellyfinID", matched.ID,
				"rating", targetRating,
				"err", err,
			)
			continue
		}

		summary.Updated++
		s.log.Info("updated Jellyfin rating",
			"name", title.PrimaryName(),
			"jellyfinID", matched.ID,
			"rating", targetRating,
			"likes", targetRating >= 6,
		)

		if s.throttleDelay > 0 {
			s.sleepFn(s.throttleDelay)
		}
	}

	return summary, nil
}

// TestConnection verifies the Jellyfin URL, API key, and user ID by querying 1 item.
func (s *JellyfinSyncService) TestConnection(ctx context.Context) error {
	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.BaseURL), "/")
	userID := url.PathEscape(strings.TrimSpace(s.cfg.UserID))
	if baseURL == "" || strings.TrimSpace(s.cfg.APIKey) == "" || userID == "" {
		return fmt.Errorf("jellyfin url, api key, and user id are required")
	}

	reqURL := fmt.Sprintf("%s/Users/%s/Items?limit=1", baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create jellyfin test request: %w", err)
	}
	req.Header.Set("X-Emby-Token", strings.TrimSpace(s.cfg.APIKey))
	req.Header.Set("Accept", "application/json")

	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("jellyfin connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("jellyfin returned %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return nil
}

// SyncSingleTitle pushes the Trackarr rating of a single title to Jellyfin if matched.
func (s *JellyfinSyncService) SyncSingleTitle(ctx context.Context, titleID int64) error {
	if s.titles == nil {
		return fmt.Errorf("title repository not configured")
	}
	title, err := s.titles.GetByID(titleID)
	if err != nil {
		return fmt.Errorf("get title %d: %w", titleID, err)
	}
	if title == nil || title.MyRating == nil || *title.MyRating <= 0 {
		return nil
	}

	jfItems, err := s.FetchUserItems(ctx)
	if err != nil {
		return err
	}
	matched := s.MatchItem(*title, jfItems)
	if matched == nil {
		s.log.Debug("no matching Jellyfin item found for title", "titleID", titleID, "name", title.PrimaryName())
		return nil
	}
	if matched.UserData != nil && matched.UserData.Rating != nil && int(math.Round(*matched.UserData.Rating)) == *title.MyRating {
		return nil
	}
	return s.PushRating(ctx, matched.ID, *title.MyRating)
}

// SyncUnwatchedMovies fetches movies from Jellyfin, filters to unwatched movies not yet in
// Trackarr, creates them as plan_to_watch with downloaded_at set, and enqueues enrichment tasks.
func (s *JellyfinSyncService) SyncUnwatchedMovies(ctx context.Context) (JellyfinMovieSyncSummary, error) {
	var summary JellyfinMovieSyncSummary
	if s == nil {
		return summary, nil
	}

	baseURL, apiKey, userID := s.resolveConfig()
	if baseURL == "" || apiKey == "" || userID == "" {
		return summary, nil
	}
	if s.db == nil {
		return summary, fmt.Errorf("jellyfin movie sync: database is not initialized")
	}

	items, err := s.FetchUserMovies(ctx)
	if err != nil {
		return summary, fmt.Errorf("jellyfin movie sync: fetch user movies: %w", err)
	}

	summary.DryRun = s.cfg.DryRun
	summary.Fetched = len(items)

	readTitles := s.titles
	if readTitles == nil {
		readTitles = repository.NewTitleRepository(s.db)
	}

	for _, item := range items {
		if ctx.Err() != nil {
			return summary, ctx.Err()
		}
		if item.Type != "" && !strings.EqualFold(item.Type, "Movie") {
			continue
		}
		if item.UserData != nil && item.UserData.Played {
			continue
		}
		summary.Unwatched++

		ids := extractJellyfinProviderIDs(item.ProviderIds)
		externalSourceID := strings.TrimSpace(item.ID)
		name := strings.TrimSpace(item.Name)
		year := clampJellyfinYear(item.ProductionYear)

		if name == "" && ids.TMDB == 0 && ids.IMDB == "" {
			summary.Skipped++
			continue
		}

		imdbPtr, tmdbPtr, _, extIDPtr := optionalJellyfinIDPointers(ids, externalSourceID)
		titleName := resolveSyncedMovieName(name, ids)

		existing, lookupErr := findExistingJellyfinTitle(readTitles, model.TitleTypeMovie, imdbPtr, tmdbPtr, nil, extIDPtr, name, year)
		if lookupErr != nil {
			summary.Errors++
			s.log.Error("failed to lookup existing movie during Jellyfin sync", "name", titleName, "jellyfinID", externalSourceID, "err", lookupErr)
			continue
		}
		if existing != nil {
			summary.Skipped++
			continue
		}

		if s.cfg.DryRun {
			summary.Created++
			continue
		}

		created, createErr := createJellyfinTitleIfMissing(ctx, s.db, jellyfinTitleImport{
			TitleType:        model.TitleTypeMovie,
			TitleName:        titleName,
			OriginalName:     name,
			Year:             year,
			IDs:              ids,
			ExternalSourceID: externalSourceID,
			MatchSource:      "jellyfin_sync",
			LookupTVDB:       false,
		})
		if createErr != nil {
			summary.Errors++
			s.log.Error("failed to create unwatched movie from Jellyfin", "name", titleName, "jellyfinID", externalSourceID, "err", createErr)
			continue
		}
		if !created {
			summary.Skipped++
			continue
		}
		summary.Created++
	}

	return summary, nil
}

func extractJellyfinProviderIDs(providerIDs map[string]string) ExternalIDs {
	var ids ExternalIDs
	if imdb := strings.ToLower(getJellyfinProvider(providerIDs, "Imdb")); imdb != "" {
		ids.IMDB = imdb
	}
	if tmdbStr := getJellyfinProvider(providerIDs, "Tmdb"); tmdbStr != "" {
		if v, err := strconv.ParseInt(tmdbStr, 10, 64); err == nil && v > 0 {
			ids.TMDB = v
		}
	}
	if tvdbStr := getJellyfinProvider(providerIDs, "Tvdb"); tvdbStr != "" {
		if v, err := strconv.ParseInt(tvdbStr, 10, 64); err == nil && v > 0 {
			ids.TVDB = v
		}
	}
	return ids
}

func clampJellyfinYear(year int) int {
	if year < 0 || year > 2100 {
		return 0
	}
	return year
}

func resolveSyncedMovieName(name string, ids ExternalIDs) string {
	if name != "" {
		return name
	}
	switch {
	case ids.IMDB != "":
		return ids.IMDB
	case ids.TMDB > 0:
		return fmt.Sprintf("TMDB %d", ids.TMDB)
	default:
		return ""
	}
}
