package handler

import (
	"net/http"

	"github.com/Soviann/trackarr/internal/cache"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

// PeopleHandler serves cast and crew search requests across the library.
type PeopleHandler struct {
	people *repository.PeopleRepository
	cache  *cache.Cache
}

// NewPeopleHandler creates a PeopleHandler with an optional read cache.
func NewPeopleHandler(people *repository.PeopleRepository, c ...*cache.Cache) *PeopleHandler {
	h := &PeopleHandler{people: people}
	if len(c) > 0 {
		h.cache = c[0]
	}
	return h
}

// SetCache injects the in-memory read cache.
func (h *PeopleHandler) SetCache(c *cache.Cache) {
	if h != nil {
		h.cache = c
	}
}

// Search handles GET /api/people?search=...&limit=...
func (h *PeopleHandler) Search(w http.ResponseWriter, r *http.Request) error {
	cacheKey := "people:search:" + r.URL.RawQuery
	if h.cache != nil {
		if val, ok := h.cache.Get(cacheKey); ok {
			if results, ok := val.([]model.PersonSearchResult); ok {
				httputil.WriteJSON(w, http.StatusOK, results)
				return nil
			}
		}
	}

	limit := httputil.ParseQueryInt(r, "limit", repository.DefaultPeopleSearchLimit)
	if limit <= 0 {
		limit = repository.DefaultPeopleSearchLimit
	}
	if limit > repository.MaxPeopleSearchLimit {
		limit = repository.MaxPeopleSearchLimit
	}

	search := r.URL.Query().Get("search")
	results, err := h.people.Search(r.Context(), search, limit)
	if err != nil {
		return httputil.InternalError("Failed to search people", err)
	}
	if results == nil {
		results = []model.PersonSearchResult{}
	}

	if h.cache != nil {
		h.cache.Set(cacheKey, results, cache.TagTitles)
	}

	httputil.WriteJSON(w, http.StatusOK, results)
	return nil
}
