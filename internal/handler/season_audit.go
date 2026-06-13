package handler

import (
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/service"
)

// SeasonAuditHandler exposes the season-audit admin endpoints.
type SeasonAuditHandler struct {
	svc *service.SeasonAuditService
}

// NewSeasonAuditHandler creates a new SeasonAuditHandler.
func NewSeasonAuditHandler(svc *service.SeasonAuditService) *SeasonAuditHandler {
	return &SeasonAuditHandler{svc: svc}
}

// List handles GET /api/admin/season-audit — returns season-attachment proposals.
func (h *SeasonAuditHandler) List(w http.ResponseWriter, r *http.Request) error {
	proposals, err := h.svc.Scan(r.Context())
	if err != nil {
		return httputil.InternalError("season audit: scan", err)
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"proposals": proposals})
	return nil
}

// Accept handles POST /api/admin/season-audit/accept — merges the source title
// into the target as the given season.
func (h *SeasonAuditHandler) Accept(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		SourceTitleID int64 `json:"source_title_id"`
		TargetTitleID int64 `json:"target_title_id"`
		SeasonNumber  int   `json:"season_number"`
	}
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		return httputil.BadRequest("Invalid JSON")
	}
	if req.SourceTitleID <= 0 || req.TargetTitleID <= 0 {
		return httputil.BadRequest("source_title_id and target_title_id are required")
	}
	if req.SourceTitleID == req.TargetTitleID {
		return httputil.BadRequest("source and target must differ")
	}

	if err := h.svc.Accept(r.Context(), req.SourceTitleID, req.TargetTitleID, req.SeasonNumber); err != nil {
		return httputil.InternalError("season audit: accept", err)
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}

// Dismiss handles POST /api/admin/season-audit/dismiss — records that the
// (source, target) attachment should not be proposed again.
func (h *SeasonAuditHandler) Dismiss(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		SourceTitleID int64 `json:"source_title_id"`
		TargetTitleID int64 `json:"target_title_id"`
	}
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		return httputil.BadRequest("Invalid JSON")
	}
	if req.SourceTitleID <= 0 || req.TargetTitleID <= 0 {
		return httputil.BadRequest("source_title_id and target_title_id are required")
	}

	if err := h.svc.Dismiss(r.Context(), req.SourceTitleID, req.TargetTitleID); err != nil {
		return httputil.InternalError("season audit: dismiss", err)
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}
