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

// JellyfinUserData represents user-specific rating data returned by Jellyfin.
type JellyfinUserData struct {
	Rating *float64 `json:"Rating,omitempty"`
	Likes  *bool    `json:"Likes,omitempty"`
}

// JellyfinLibraryItem represents a movie or series in the user's Jellyfin library.
type JellyfinLibraryItem struct {
	ID          string            `json:"Id"`
	Name        string            `json:"Name"`
	Type        string            `json:"Type"`
	ProviderIds map[string]string `json:"ProviderIds"`
	UserData    *JellyfinUserData `json:"UserData,omitempty"`
}

type jellyfinItemsResponse struct {
	Items            []JellyfinLibraryItem `json:"Items"`
	TotalRecordCount int                   `json:"TotalRecordCount"`
}

// JellyfinSyncService handles synchronizing ratings from Trackarr into Jellyfin.
type JellyfinSyncService struct {
	db            *sql.DB
	titles        *repository.TitleRepository
	cfg           JellyfinSyncConfig
	log           *slog.Logger
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

// FetchUserItems queries Jellyfin for movies and series in the user's library with external provider IDs.
func (s *JellyfinSyncService) FetchUserItems(ctx context.Context) ([]JellyfinLibraryItem, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.BaseURL), "/")
	userID := url.PathEscape(strings.TrimSpace(s.cfg.UserID))
	reqURL := fmt.Sprintf("%s/Users/%s/Items?includeItemTypes=Movie,Series&recursive=true&fields=ProviderIds", baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create fetch items request: %w", err)
	}

	req.Header.Set("X-Emby-Token", strings.TrimSpace(s.cfg.APIKey))
	req.Header.Set("Accept", "application/json")

	resp, err := s.cfg.HTTPClient.Do(req)
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

	getProvider := func(ids map[string]string, key string) string {
		for k, v := range ids {
			if strings.EqualFold(k, key) {
				return strings.TrimSpace(v)
			}
		}
		return ""
	}

	// 1. Match by TMDB ID
	if title.TMDBID != nil && *title.TMDBID > 0 {
		targetTMDB := strconv.FormatInt(*title.TMDBID, 10)
		for i := range jfItems {
			if strings.EqualFold(jfItems[i].Type, targetType) {
				if getProvider(jfItems[i].ProviderIds, "Tmdb") == targetTMDB {
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
				if getProvider(jfItems[i].ProviderIds, "Tvdb") == targetTVDB {
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
				providerIMDB := strings.ToLower(getProvider(jfItems[i].ProviderIds, "Imdb"))
				if providerIMDB != "" && (providerIMDB == targetIMDB || strings.TrimPrefix(providerIMDB, "tt") == strings.TrimPrefix(targetIMDB, "tt")) {
					item := jfItems[i]
					return &item
				}
			}
		}
	}

	return nil
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
