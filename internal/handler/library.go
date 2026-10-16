package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Soviann/trackarr/internal/cache"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
)

// LibraryHandler serves the library-specific endpoints (continue watching, upcoming).
type LibraryHandler struct {
	titles *repository.TitleRepository
	cache  *cache.Cache
}

// NewLibraryHandler creates a new LibraryHandler with optional cache.
func NewLibraryHandler(titles *repository.TitleRepository, c ...*cache.Cache) *LibraryHandler {
	h := &LibraryHandler{titles: titles}
	if len(c) > 0 {
		h.cache = c[0]
	}
	return h
}

// SetCache injects the in-memory read cache.
func (h *LibraryHandler) SetCache(c *cache.Cache) {
	h.cache = c
}

// ContinueWatching returns Watching titles with ≥1 unwatched episode, ordered by last_watched_at DESC.
func (h *LibraryHandler) ContinueWatching(w http.ResponseWriter, r *http.Request) error {
	const cacheKey = "library:continue_watching"
	if h.cache != nil {
		if val, ok := h.cache.Get(cacheKey); ok {
			if items, ok := val.([]repository.ContinueWatchingItem); ok {
				httputil.WriteJSON(w, http.StatusOK, items)
				return nil
			}
		}
	}

	items, err := h.titles.ListContinueWatching()
	if err != nil {
		return fmt.Errorf("library: continue watching: %w", err)
	}
	if items == nil {
		items = []repository.ContinueWatchingItem{}
	}

	if h.cache != nil {
		h.cache.Set(cacheKey, items, cache.TagTitles)
	}

	httputil.WriteJSON(w, http.StatusOK, items)
	return nil
}

// Upcoming returns Watching/PlanToWatch titles with next_air_date >= today, ordered by next_air_date ASC.
func (h *LibraryHandler) Upcoming(w http.ResponseWriter, r *http.Request) error {
	today := time.Now().Format("2006-01-02")
	cacheKey := "library:upcoming:" + today
	if h.cache != nil {
		if val, ok := h.cache.Get(cacheKey); ok {
			if items, ok := val.([]repository.UpcomingItem); ok {
				httputil.WriteJSON(w, http.StatusOK, items)
				return nil
			}
		}
	}

	items, err := h.titles.ListUpcoming(today)
	if err != nil {
		return fmt.Errorf("library: upcoming: %w", err)
	}
	if items == nil {
		items = []repository.UpcomingItem{}
	}

	if h.cache != nil {
		h.cache.Set(cacheKey, items, cache.TagTitles)
	}

	httputil.WriteJSON(w, http.StatusOK, items)
	return nil
}
