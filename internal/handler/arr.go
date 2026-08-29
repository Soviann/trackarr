package handler

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/go-chi/chi/v5"
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

func (h *ArrHandler) GetTitleArr(w http.ResponseWriter, r *http.Request) error {
	titleID, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return err
	}

	details, err := h.arrSvc.GetTitleArrDetails(r.Context(), titleID)
	if err != nil {
		return httputil.InternalError("Failed to get title Arr details", err)
	}

	httputil.WriteJSON(w, http.StatusOK, details)
	return nil
}

type arrPushRequest struct {
	Monitored      bool   `json:"monitored"`
	Search         bool   `json:"search"`
	RootFolder     string `json:"root_folder"`
	QualityProfile int    `json:"quality_profile"`
}

func readPushPayload(r *http.Request, titleID int64) (service.PushPayload, error) {
	var req arrPushRequest
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		return service.PushPayload{}, httputil.BadRequest("Invalid JSON")
	}
	return service.PushPayload{
		TitleID:        titleID,
		Monitored:      req.Monitored,
		Search:         req.Search,
		RootFolder:     req.RootFolder,
		QualityProfile: req.QualityProfile,
	}, nil
}

func (h *ArrHandler) UpdateTitleArr(w http.ResponseWriter, r *http.Request) error {
	titleID, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return err
	}

	payload, err := readPushPayload(r, titleID)
	if err != nil {
		return err
	}

	details, err := h.arrSvc.UpdateTitle(r.Context(), titleID, payload)
	if err != nil {
		return httputil.InternalError(fmt.Sprintf("Failed to update on Arr: %v", err), err)
	}

	httputil.WriteJSON(w, http.StatusOK, details)
	return nil
}

func (h *ArrHandler) PushToArr(w http.ResponseWriter, r *http.Request) error {
	titleID, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return err
	}

	payload, err := readPushPayload(r, titleID)
	if err != nil {
		return err
	}

	arrID, err := h.arrSvc.PushTitle(r.Context(), titleID, payload)
	if err != nil {
		return httputil.InternalError(fmt.Sprintf("Failed to push to Arr: %v", err), err)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"arr_id": arrID,
	})
	return nil
}
