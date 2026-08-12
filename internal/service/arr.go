package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type ArrService struct {
	cfg      *config.Config
	client   *http.Client
	settings *repository.SettingRepository
	writeDB  *sql.DB
}

func NewArrService(cfg *config.Config, settings *repository.SettingRepository, writeDB *sql.DB) *ArrService {
	return &ArrService{
		cfg:      cfg,
		client:   &http.Client{},
		settings: settings,
		writeDB:  writeDB,
	}
}

// PushPayload is the queue payload for arr push tasks.
type PushPayload struct {
	TitleID        int64  `json:"title_id"`
	Monitored      bool   `json:"monitored"`
	Search         bool   `json:"search"`
	RootFolder     string `json:"root_folder"`
	QualityProfile int    `json:"quality_profile"`
}

// EnqueuePush enqueues an arr push task and saves the arr_ignored state if pushing is bypassed.
func (s *ArrService) EnqueuePush(ctx context.Context, app string, payload PushPayload) error {
	tx, err := s.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if app == "ignore" {
		arrIgnored := true
		update := repository.TitleUpdate{
			ArrIgnored: &arrIgnored,
		}
		if err := repository.NewTitleWriter(tx).Update(ctx, payload.TitleID, update); err != nil {
			return fmt.Errorf("update arr_ignored: %w", err)
		}
		return tx.Commit()
	}

	taskType := model.TaskTypeRadarrPush
	if app == "sonarr" {
		taskType = model.TaskTypeSonarrPush
	} else if app != "radarr" {
		return fmt.Errorf("unknown arr app: %s", app)
	}

	b, _ := json.Marshal(payload)
	dedup := strconv.FormatInt(payload.TitleID, 10)
	_, err = repository.NewTaskWriter(tx).Enqueue(ctx, taskType, string(b), &dedup)
	if err != nil {
		return fmt.Errorf("enqueue push task: %w", err)
	}

	return tx.Commit()
}

func (s *ArrService) getAppConfig(app string) (baseURL string, apiKey string) {
	switch app {
	case "radarr":
		baseURL = s.cfg.RadarrURL
		apiKey = s.cfg.RadarrAPIKey
		if baseURL == "" && s.settings != nil {
			baseURL, _ = s.settings.Get("radarr_url")
		}
		if apiKey == "" && s.settings != nil {
			apiKey, _ = s.settings.Get("radarr_api_key")
		}
	case "sonarr":
		baseURL = s.cfg.SonarrURL
		apiKey = s.cfg.SonarrAPIKey
		if baseURL == "" && s.settings != nil {
			baseURL, _ = s.settings.Get("sonarr_url")
		}
		if apiKey == "" && s.settings != nil {
			apiKey, _ = s.settings.Get("sonarr_api_key")
		}
	}
	return strings.TrimSpace(baseURL), strings.TrimSpace(apiKey)
}

// ProxyRequest proxies a request to Radarr or Sonarr API.
func (s *ArrService) ProxyRequest(ctx context.Context, app string, method string, path string, body io.Reader) (*http.Response, error) {
	baseURL, apiKey := s.getAppConfig(app)
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("%s URL or API key not configured", app)
	}

	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	reqURL, err := url.Parse(fullURL)
	if err != nil {
		return nil, fmt.Errorf("invalid %s URL (%s): %w", app, fullURL, err)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return s.client.Do(req)
}

// Ensure RadarrPush and SonarrPush logic here, or we can just leave it for background_arr.go.
