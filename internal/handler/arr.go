package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
)

type ArrHandler struct {
	arrSvc  *service.ArrService
	titles  *repository.TitleRepository
	writeDB *sql.DB
}

func NewArrHandler(arrSvc *service.ArrService, titles *repository.TitleRepository, writeDB *sql.DB) *ArrHandler {
	return &ArrHandler{arrSvc: arrSvc, titles: titles, writeDB: writeDB}
}

func (h *ArrHandler) ProxyRootFolder(w http.ResponseWriter, r *http.Request) error {
	app := chi.URLParam(r, "app")
	return h.proxy(r.Context(), w, app, "/api/v3/rootfolder")
}

func (h *ArrHandler) ProxyQualityProfile(w http.ResponseWriter, r *http.Request) error {
	app := chi.URLParam(r, "app")
	return h.proxy(r.Context(), w, app, "/api/v3/qualityprofile")
}

func (h *ArrHandler) proxy(ctx context.Context, w http.ResponseWriter, app, path string) error {
	resp, err := h.arrSvc.ProxyRequest(ctx, app, "GET", path, nil)
	if err != nil {
		return httputil.InternalError("Failed to proxy request", err)
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))

	statusCode := resp.StatusCode
	if statusCode == http.StatusUnauthorized {
		statusCode = http.StatusBadGateway
	}

	w.WriteHeader(statusCode)
	_, _ = io.Copy(w, resp.Body)
	return nil
}

func (h *ArrHandler) ListArrQueue(w http.ResponseWriter, r *http.Request) error {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	items, hasMore, err := h.titles.ListPaginatedArrQueue(limit, offset)
	if err != nil {
		return httputil.InternalError(fmt.Sprintf("Failed to list arr queue: %v", err), err)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"items":    items,
		"has_more": hasMore,
	})
	return nil
}

func (h *ArrHandler) PushToArr(w http.ResponseWriter, r *http.Request) error {
	titleIDStr := chi.URLParam(r, "id")
	titleID, err := strconv.ParseInt(titleIDStr, 10, 64)
	if err != nil {
		return httputil.BadRequest("Invalid title ID")
	}

	var req struct {
		Monitored      bool   `json:"monitored"`
		Search         bool   `json:"search"`
		RootFolder     string `json:"root_folder"`
		QualityProfile int    `json:"quality_profile"`
	}

	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		return httputil.BadRequest("Invalid JSON")
	}

	title, err := h.titles.GetByID(titleID)
	if err != nil {
		return httputil.InternalError("Title not found", err)
	}

	if title.Type == model.TitleTypeMovie {
		if title.TMDBID == nil || *title.TMDBID == 0 {
			return httputil.BadRequest("Title has no TMDB ID")
		}

		taskData := map[string]interface{}{
			"title_id":        titleID,
			"tmdb_id":         *title.TMDBID,
			"monitored":       req.Monitored,
			"search":          req.Search,
			"root_folder":     req.RootFolder,
			"quality_profile": req.QualityProfile,
		}
		payload, _ := json.Marshal(taskData)
		if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
			dedup := fmt.Sprintf("arr_push_%d", titleID)
			_, err := repository.NewTaskWriter(tx).Enqueue(r.Context(), model.TaskTypeRadarrPush, string(payload), &dedup)
			return err
		}); err != nil {
			return httputil.InternalError("Failed to enqueue task", err)
		}
	} else {
		if title.TVDBID == nil || *title.TVDBID == 0 {
			return httputil.BadRequest("Title has no TVDB ID")
		}

		taskData := map[string]interface{}{
			"title_id":        titleID,
			"tvdb_id":         *title.TVDBID,
			"monitored":       req.Monitored,
			"search":          req.Search,
			"root_folder":     req.RootFolder,
			"quality_profile": req.QualityProfile,
		}
		payload, _ := json.Marshal(taskData)
		if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
			dedup := fmt.Sprintf("arr_push_%d", titleID)
			_, err := repository.NewTaskWriter(tx).Enqueue(r.Context(), model.TaskTypeSonarrPush, string(payload), &dedup)
			return err
		}); err != nil {
			return httputil.InternalError("Failed to enqueue task", err)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "queued"})
	return nil
}
