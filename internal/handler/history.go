package handler

import (
	"fmt"
	"net/http"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
)

// HistoryHandler handles requests for per-title watch history.
type HistoryHandler struct {
	history *repository.HistoryRepository
}

// NewHistoryHandler creates a new HistoryHandler.
func NewHistoryHandler(history *repository.HistoryRepository) *HistoryHandler {
	return &HistoryHandler{history: history}
}

// Get handles GET /api/titles/{id}/history — returns watch history grouped by episode.
func (h *HistoryHandler) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return err
	}
	history, err := h.history.GetByTitleID(r.Context(), id)
	if err != nil {
		return fmt.Errorf("history: get: %w", err)
	}
	httputil.WriteJSON(w, http.StatusOK, history)
	return nil
}
