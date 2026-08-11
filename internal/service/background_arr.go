package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"log/slog"
)

func (w *TaskQueueWorker) handleArrPush(ctx context.Context, task model.Task, logger *slog.Logger, app string) error {
	var payload PushPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode arr_push payload: %w", err)
	}

	if w.arrSvc == nil {
		return fmt.Errorf("arr service not configured")
	}

	title, err := w.titles.GetByID(payload.TitleID)
	if err != nil {
		return fmt.Errorf("get title %d: %w", payload.TitleID, err)
	}

	externalID := ""
	var lookupEndpoint string
	switch app {
	case "radarr":
		if title.TMDBID == nil || *title.TMDBID == 0 {
			return fmt.Errorf("TMDB ID not found for title %d (required for Radarr)", payload.TitleID)
		}
		externalID = fmt.Sprintf("tmdbId=%d", *title.TMDBID)
		lookupEndpoint = "/api/v3/movie/lookup?"
	case "sonarr":
		if title.TVDBID == nil || *title.TVDBID == 0 {
			return fmt.Errorf("TVDB ID not found for title %d (required for Sonarr)", payload.TitleID)
		}
		externalID = fmt.Sprintf("term=tvdb:%d", *title.TVDBID)
		lookupEndpoint = "/api/v3/series/lookup?"
	default:
		return fmt.Errorf("unknown app: %s", app)
	}

	// Fetch existing entry from Arr
	resp, err := w.arrSvc.ProxyRequest(ctx, app, "GET", lookupEndpoint+externalID, nil)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", app, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s lookup returned %d: %s", app, resp.StatusCode, string(body))
	}

	var lookupResults []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&lookupResults); err != nil {
		return fmt.Errorf("decode lookup %s: %w", app, err)
	}

	if len(lookupResults) == 0 {
		return fmt.Errorf("%s lookup returned no results for %s", app, externalID)
	}

	lookupResult := lookupResults[0]

	arrIDFloat, exists := lookupResult["id"].(float64)
	if exists && arrIDFloat > 0 {
		arrID := int64(arrIDFloat)

		// Update existing title with user's requested options
		lookupResult["monitored"] = payload.Monitored
		lookupResult["qualityProfileId"] = payload.QualityProfile
		lookupResult["rootFolderPath"] = payload.RootFolder

		updateBody, err := json.Marshal(lookupResult)
		if err != nil {
			return fmt.Errorf("marshal %s update payload: %w", app, err)
		}

		endpoint := fmt.Sprintf("/api/v3/movie/%d", arrID)
		if app == "sonarr" {
			endpoint = fmt.Sprintf("/api/v3/series/%d", arrID)
		}

		updateResp, err := w.arrSvc.ProxyRequest(ctx, app, "PUT", endpoint, bytes.NewBuffer(updateBody))
		if err != nil {
			return fmt.Errorf("update %s: %w", app, err)
		}
		defer updateResp.Body.Close()

		if updateResp.StatusCode != http.StatusAccepted && updateResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(updateResp.Body)
			return fmt.Errorf("%s update returned %d: %s", app, updateResp.StatusCode, string(body))
		}

		return w.saveArrID(ctx, payload.TitleID, app, arrID)
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
		return fmt.Errorf("marshal %s add payload: %w", app, err)
	}

	endpoint := "/api/v3/movie"
	if app == "sonarr" {
		endpoint = "/api/v3/series"
	}

	addResp, err := w.arrSvc.ProxyRequest(ctx, app, "POST", endpoint, bytes.NewBuffer(addBody))
	if err != nil {
		return fmt.Errorf("add %s: %w", app, err)
	}
	defer addResp.Body.Close()

	if addResp.StatusCode != http.StatusCreated && addResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(addResp.Body)
		return fmt.Errorf("%s add returned %d: %s", app, addResp.StatusCode, string(body))
	}

	var addedResult map[string]interface{}
	if err := json.NewDecoder(addResp.Body).Decode(&addedResult); err != nil {
		return fmt.Errorf("decode added %s: %w", app, err)
	}

	arrIDFloat, exists = addedResult["id"].(float64)
	if !exists || arrIDFloat == 0 {
		return fmt.Errorf("%s add did not return an ID", app)
	}

	arrID := int64(arrIDFloat)
	return w.saveArrID(ctx, payload.TitleID, app, arrID)
}

func (w *TaskQueueWorker) saveArrID(ctx context.Context, titleID int64, app string, arrID int64) error {
	tx, err := w.writeDB.BeginTx(ctx, nil)
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
