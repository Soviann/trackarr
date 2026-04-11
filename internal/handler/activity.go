package handler

import (
	"fmt"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

// ActivityHandler handles requests for the activity feed.
type ActivityHandler struct {
	activity *repository.ActivityRepository
}

// NewActivityHandler creates a new ActivityHandler.
func NewActivityHandler(activity *repository.ActivityRepository) *ActivityHandler {
	return &ActivityHandler{activity: activity}
}

// List handles GET /api/stats/activity — returns paginated watch events.
func (h *ActivityHandler) List(w http.ResponseWriter, r *http.Request) error {
	limit := httputil.ParseQueryInt(r, "limit", 50)
	offset := httputil.ParseQueryInt(r, "offset", 0)
	if limit > 100 {
		limit = 100
	}
	events, err := h.activity.List(r.Context(), limit, offset)
	if err != nil {
		return fmt.Errorf("activity: list: %w", err)
	}
	httputil.WriteJSON(w, http.StatusOK, events)
	return nil
}
