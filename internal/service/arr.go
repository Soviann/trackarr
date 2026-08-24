package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

type ArrService struct {
	cfg      *config.Config
	client   *http.Client
	settings *repository.SettingRepository
	titles   *repository.TitleRepository
	writeDB  *sql.DB
}

func NewArrService(cfg *config.Config, settings *repository.SettingRepository, titles *repository.TitleRepository, writeDB *sql.DB) *ArrService {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if len(via) > 0 {
				if apiKey := via[0].Header.Get("X-Api-Key"); apiKey != "" {
					req.Header.Set("X-Api-Key", apiKey)
				}
			}
			return nil
		},
	}
	return &ArrService{
		cfg:      cfg,
		client:   client,
		settings: settings,
		titles:   titles,
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

func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.Contains(raw, ".") && !strings.Contains(raw, ":") && !strings.HasPrefix(raw, "127.") && !strings.HasPrefix(raw, "192.168.") && !strings.HasPrefix(raw, "10.") {
		return "https://" + raw
	}
	return "http://" + raw
}

// ProxyRequest proxies a request to Radarr or Sonarr API.
func (s *ArrService) ProxyRequest(ctx context.Context, app string, method string, path string, body io.Reader) (*http.Response, error) {
	baseURL, apiKey := s.getAppConfig(app)
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("%s URL or API key not configured", app)
	}

	baseURL = normalizeBaseURL(baseURL)

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

// PushTitle pushes a title to Radarr or Sonarr directly and updates its radarr_id / sonarr_id.
func (s *ArrService) PushTitle(ctx context.Context, titleID int64, payload PushPayload) (int64, error) {
	if s.titles == nil {
		return 0, fmt.Errorf("titles repository not configured on arr service")
	}

	title, err := s.titles.GetByID(titleID)
	if err != nil {
		return 0, fmt.Errorf("get title %d: %w", titleID, err)
	}

	var app string
	externalID := ""
	var lookupEndpoint string
	if title.Type == model.TitleTypeMovie {
		app = "radarr"
		if title.TMDBID == nil || *title.TMDBID == 0 {
			return 0, fmt.Errorf("TMDB ID not found for title %d (required for Radarr)", titleID)
		}
		externalID = fmt.Sprintf("term=tmdb:%d", *title.TMDBID)
		lookupEndpoint = "/api/v3/movie/lookup?"
	} else {
		app = "sonarr"
		if title.TVDBID == nil || *title.TVDBID == 0 {
			return 0, fmt.Errorf("TVDB ID not found for title %d (required for Sonarr)", titleID)
		}
		externalID = fmt.Sprintf("term=tvdb:%d", *title.TVDBID)
		lookupEndpoint = "/api/v3/series/lookup?"
	}

	// Fetch existing entry from Arr
	resp, err := s.ProxyRequest(ctx, app, "GET", lookupEndpoint+externalID, nil)
	if err != nil {
		return 0, fmt.Errorf("lookup %s: %w", app, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("%s lookup returned %d: %s", app, resp.StatusCode, string(body))
	}

	var lookupResults []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&lookupResults); err != nil {
		return 0, fmt.Errorf("decode lookup %s: %w", app, err)
	}
	if len(lookupResults) == 0 {
		return 0, fmt.Errorf("%s lookup returned no results for %s", app, externalID)
	}
	lookupResult := lookupResults[0]
	arrIDFloat, exists := lookupResult["id"].(float64)
	if exists && arrIDFloat > 0 {
		arrID := int64(arrIDFloat)
		// Title already exists in Radarr/Sonarr
		if err := s.saveArrID(ctx, titleID, app, arrID); err != nil {
			return 0, err
		}
		return arrID, nil
	}

	// Prepare add payload
	lookupResult["monitored"] = payload.Monitored
	lookupResult["qualityProfileId"] = payload.QualityProfile
	lookupResult["rootFolderPath"] = payload.RootFolder
	lookupResult["addOptions"] = map[string]interface{}{
		"searchForMovie":           payload.Search,
		"searchForMissingEpisodes": payload.Search,
	}

	addBody, err := json.Marshal(lookupResult)
	if err != nil {
		return 0, fmt.Errorf("marshal %s add payload: %w", app, err)
	}

	endpoint := "/api/v3/movie"
	if app == "sonarr" {
		endpoint = "/api/v3/series"
	}

	addResp, err := s.ProxyRequest(ctx, app, "POST", endpoint, bytes.NewBuffer(addBody))
	if err != nil {
		return 0, fmt.Errorf("add %s: %w", app, err)
	}
	defer addResp.Body.Close()

	if addResp.StatusCode != http.StatusCreated && addResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(addResp.Body)
		return 0, fmt.Errorf("%s add returned %d: %s", app, addResp.StatusCode, string(body))
	}

	var addedResult map[string]interface{}
	if err := json.NewDecoder(addResp.Body).Decode(&addedResult); err != nil {
		return 0, fmt.Errorf("decode added %s: %w", app, err)
	}

	arrIDFloat, exists = addedResult["id"].(float64)
	if !exists || arrIDFloat == 0 {
		return 0, fmt.Errorf("%s add did not return an ID", app)
	}

	arrID := int64(arrIDFloat)
	if err := s.saveArrID(ctx, titleID, app, arrID); err != nil {
		return 0, err
	}
	return arrID, nil
}

func (s *ArrService) saveArrID(ctx context.Context, titleID int64, app string, arrID int64) error {
	tx, err := s.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	update := repository.TitleUpdate{}
	switch app {
	case "radarr":
		update.RadarrID = &arrID
	case "sonarr":
		update.SonarrID = &arrID
	default:
		return fmt.Errorf("unknown app %q", app)
	}

	if err := repository.NewTitleWriter(tx).Update(ctx, titleID, update); err != nil {
		return fmt.Errorf("update %s_id: %w", app, err)
	}

	return tx.Commit()
}

func (s *ArrService) clearArrID(ctx context.Context, titleID int64, app string) error {
	tx, err := s.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	column := "radarr_id"
	if app == "sonarr" {
		column = "sonarr_id"
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE titles SET %s = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?", column), titleID); err != nil {
		return fmt.Errorf("clear %s: %w", column, err)
	}

	return tx.Commit()
}

// ArrTitleDetails contains live status and options of a title in Radarr or Sonarr.
type ArrTitleDetails struct {
	App              string `json:"app"`
	Exists           bool   `json:"exists"`
	ArrID            int64  `json:"arr_id,omitempty"`
	TitleSlug        string `json:"title_slug,omitempty"`
	WebURL           string `json:"web_url,omitempty"`
	Monitored        bool   `json:"monitored"`
	QualityProfileID int    `json:"quality_profile_id"`
	RootFolderPath   string `json:"root_folder_path"`
	HasFile          bool   `json:"has_file"`
	SizeOnDisk       int64  `json:"size_on_disk,omitempty"`
}

// GetTitleArrDetails retrieves the live status, options, and direct web URL for a title from Radarr or Sonarr.
func (s *ArrService) GetTitleArrDetails(ctx context.Context, titleID int64) (*ArrTitleDetails, error) {
	if s.titles == nil {
		return nil, fmt.Errorf("titles repository not configured on arr service")
	}

	title, err := s.titles.GetByID(titleID)
	if err != nil {
		return nil, fmt.Errorf("get title %d: %w", titleID, err)
	}

	isRadarr := title.Type == model.TitleTypeMovie
	app := "sonarr"
	if isRadarr {
		app = "radarr"
	}

	baseURL, _ := s.getAppConfig(app)
	baseURL = normalizeBaseURL(baseURL)

	var arrData map[string]interface{}
	var arrID int64

	// 1. Try to fetch by saved arr ID if present
	if isRadarr && title.RadarrID != nil && *title.RadarrID > 0 {
		arrID = *title.RadarrID
		resp, err := s.ProxyRequest(ctx, "radarr", "GET", fmt.Sprintf("/api/v3/movie/%d", arrID), nil)
		if err == nil {
			defer resp.Body.Close()
			switch resp.StatusCode {
			case http.StatusOK:
				_ = json.NewDecoder(resp.Body).Decode(&arrData)
			case http.StatusNotFound:
				// Cleared on Radarr -> clear locally
				_ = s.clearArrID(ctx, titleID, "radarr")
				arrID = 0
			}
		}
	} else if !isRadarr && title.SonarrID != nil && *title.SonarrID > 0 {
		arrID = *title.SonarrID
		resp, err := s.ProxyRequest(ctx, "sonarr", "GET", fmt.Sprintf("/api/v3/series/%d", arrID), nil)
		if err == nil {
			defer resp.Body.Close()
			switch resp.StatusCode {
			case http.StatusOK:
				_ = json.NewDecoder(resp.Body).Decode(&arrData)
			case http.StatusNotFound:
				// Cleared on Sonarr -> clear locally
				_ = s.clearArrID(ctx, titleID, "sonarr")
				arrID = 0
			}
		}
	}

	// 2. If not found by ID, attempt lookup by TMDB / TVDB ID
	if arrData == nil {
		var lookupEndpoint string
		if isRadarr && title.TMDBID != nil && *title.TMDBID > 0 {
			lookupEndpoint = fmt.Sprintf("/api/v3/movie/lookup?term=tmdb:%d", *title.TMDBID)
		} else if !isRadarr && title.TVDBID != nil && *title.TVDBID > 0 {
			lookupEndpoint = fmt.Sprintf("/api/v3/series/lookup?term=tvdb:%d", *title.TVDBID)
		}

		if lookupEndpoint != "" {
			resp, err := s.ProxyRequest(ctx, app, "GET", lookupEndpoint, nil)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var results []map[string]interface{}
					if err := json.NewDecoder(resp.Body).Decode(&results); err == nil && len(results) > 0 {
						item := results[0]
						if idF, ok := item["id"].(float64); ok && idF > 0 {
							arrID = int64(idF)
							arrData = item
							_ = s.saveArrID(ctx, titleID, app, arrID)
						}
					}
				}
			}
		}
	}

	// If title doesn't exist on Arr
	if arrData == nil || arrID == 0 {
		return &ArrTitleDetails{
			App:    app,
			Exists: false,
		}, nil
	}

	// 3. Extract properties
	slug, _ := arrData["titleSlug"].(string)
	monitored, _ := arrData["monitored"].(bool)
	qpIDFloat, _ := arrData["qualityProfileId"].(float64)
	rootFolder, _ := arrData["rootFolderPath"].(string)
	if rootFolder == "" {
		rootFolder, _ = arrData["path"].(string)
	}

	var hasFile bool
	var sizeOnDisk int64

	if isRadarr {
		hasFile, _ = arrData["hasFile"].(bool)
		if sizeF, ok := arrData["sizeOnDisk"].(float64); ok {
			sizeOnDisk = int64(sizeF)
		}
	} else {
		if stats, ok := arrData["statistics"].(map[string]interface{}); ok {
			if countF, ok := stats["episodeFileCount"].(float64); ok && countF > 0 {
				hasFile = true
			}
			if sizeF, ok := stats["sizeOnDisk"].(float64); ok {
				sizeOnDisk = int64(sizeF)
			}
		}
	}

	// 4. Construct direct web URL
	var webURL string
	if baseURL != "" {
		if isRadarr {
			ref := slug
			if ref == "" && title.TMDBID != nil {
				ref = strconv.FormatInt(*title.TMDBID, 10)
			}
			if ref != "" {
				webURL = strings.TrimRight(baseURL, "/") + "/movie/" + ref
			}
		} else {
			ref := slug
			if ref != "" {
				webURL = strings.TrimRight(baseURL, "/") + "/series/" + ref
			}
		}
	}

	return &ArrTitleDetails{
		App:              app,
		Exists:           true,
		ArrID:            arrID,
		TitleSlug:        slug,
		WebURL:           webURL,
		Monitored:        monitored,
		QualityProfileID: int(qpIDFloat),
		RootFolderPath:   rootFolder,
		HasFile:          hasFile,
		SizeOnDisk:       sizeOnDisk,
	}, nil
}

// UpdateTitle updates an existing title on Radarr or Sonarr (or pushes it if it didn't exist).
func (s *ArrService) UpdateTitle(ctx context.Context, titleID int64, payload PushPayload) (*ArrTitleDetails, error) {
	current, err := s.GetTitleArrDetails(ctx, titleID)
	if err != nil {
		return nil, err
	}

	if !current.Exists {
		// Not on Arr yet -> Push it
		if _, err := s.PushTitle(ctx, titleID, payload); err != nil {
			return nil, err
		}
		return s.GetTitleArrDetails(ctx, titleID)
	}

	app := current.App
	isRadarr := app == "radarr"
	endpoint := fmt.Sprintf("/api/v3/movie/%d", current.ArrID)
	if !isRadarr {
		endpoint = fmt.Sprintf("/api/v3/series/%d", current.ArrID)
	}

	// Fetch existing full JSON object from Arr
	resp, err := s.ProxyRequest(ctx, app, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch %s item %d: %w", app, current.ArrID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s get returned %d: %s", app, resp.StatusCode, string(body))
	}

	var item map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("decode %s item: %w", app, err)
	}

	// Update configurable options
	item["monitored"] = payload.Monitored
	if payload.QualityProfile > 0 {
		item["qualityProfileId"] = payload.QualityProfile
	}
	if payload.RootFolder != "" {
		item["rootFolderPath"] = payload.RootFolder
	}

	updateBody, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("marshal %s update: %w", app, err)
	}

	putEndpoint := "/api/v3/movie"
	if !isRadarr {
		putEndpoint = "/api/v3/series"
	}

	putResp, err := s.ProxyRequest(ctx, app, "PUT", putEndpoint, bytes.NewBuffer(updateBody))
	if err != nil {
		return nil, fmt.Errorf("put %s item: %w", app, err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK && putResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(putResp.Body)
		return nil, fmt.Errorf("%s update returned %d: %s", app, putResp.StatusCode, string(body))
	}

	// If search requested, trigger Arr search command
	if payload.Search {
		var cmdBody map[string]interface{}
		if isRadarr {
			cmdBody = map[string]interface{}{
				"name":     "MoviesSearch",
				"movieIds": []int64{current.ArrID},
			}
		} else {
			cmdBody = map[string]interface{}{
				"name":     "SeriesSearch",
				"seriesId": current.ArrID,
			}
		}
		cmdJSON, _ := json.Marshal(cmdBody)
		if cmdResp, err := s.ProxyRequest(ctx, app, "POST", "/api/v3/command", bytes.NewBuffer(cmdJSON)); err == nil {
			_ = cmdResp.Body.Close()
		}
	}

	return s.GetTitleArrDetails(ctx, titleID)
}
