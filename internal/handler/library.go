package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
)

// LibraryHandler serves the library-specific endpoints (continue watching, upcoming).
type LibraryHandler struct {
	titles *repository.TitleRepository
}

// NewLibraryHandler creates a new LibraryHandler.
func NewLibraryHandler(titles *repository.TitleRepository) *LibraryHandler {
	return &LibraryHandler{titles: titles}
}

// ContinueWatching returns Watching titles with ≥1 unwatched episode, ordered by last_watched_at DESC.
func (h *LibraryHandler) ContinueWatching(w http.ResponseWriter, r *http.Request) error {
	items, err := h.titles.ListContinueWatching()
	if err != nil {
		return fmt.Errorf("library: continue watching: %w", err)
	}
	if items == nil {
		items = []repository.ContinueWatchingItem{}
	}
	httputil.WriteJSON(w, http.StatusOK, items)
	return nil
}

// Upcoming returns Watching/PlanToWatch titles with next_air_date >= today, ordered by next_air_date ASC.
func (h *LibraryHandler) Upcoming(w http.ResponseWriter, r *http.Request) error {
	today := time.Now().Format("2006-01-02")
	items, err := h.titles.ListUpcoming(today)
	if err != nil {
		return fmt.Errorf("library: upcoming: %w", err)
	}
	if items == nil {
		items = []repository.UpcomingItem{}
	}
	httputil.WriteJSON(w, http.StatusOK, items)
	return nil
}
