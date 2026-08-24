package handler

import (
	"fmt"
	"net/http"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
)

// MatchEventHandler handles requests for auto-match events.
type MatchEventHandler struct {
	repo *repository.MatchEventRepository
}

// NewMatchEventHandler creates a new MatchEventHandler.
func NewMatchEventHandler(repo *repository.MatchEventRepository) *MatchEventHandler {
	return &MatchEventHandler{repo: repo}
}

// List handles GET /api/match-events — returns recent auto-match events.
func (h *MatchEventHandler) List(w http.ResponseWriter, r *http.Request) error {
	limit := httputil.ParseQueryInt(r, "limit", 30)
	limit = min(limit, 100)
	events, err := h.repo.ListRecent(r.Context(), limit)
	if err != nil {
		return fmt.Errorf("match events: list: %w", err)
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"events": events})
	return nil
}
