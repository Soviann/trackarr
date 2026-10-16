package handler

import (
	"fmt"
	"net/http"

	"github.com/Soviann/trackarr/internal/cache"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
)

// GenreHandler serves genre data for filter UI.
type GenreHandler struct {
	genres *repository.GenreRepository
	cache  *cache.Cache
}

func NewGenreHandler(genres *repository.GenreRepository, c ...*cache.Cache) *GenreHandler {
	h := &GenreHandler{genres: genres}
	if len(c) > 0 {
		h.cache = c[0]
	}
	return h
}

// SetCache injects the in-memory read cache.
func (h *GenreHandler) SetCache(c *cache.Cache) {
	h.cache = c
}

// List returns all genres with title counts, ordered by count descending.
func (h *GenreHandler) List(w http.ResponseWriter, r *http.Request) error {
	cacheKey := "genres:list:" + r.URL.RawQuery
	if h.cache != nil {
		if val, ok := h.cache.Get(cacheKey); ok {
			if genres, ok := val.([]repository.GenreCount); ok {
				httputil.WriteJSON(w, http.StatusOK, genres)
				return nil
			}
		}
	}

	genres, err := h.genres.ListWithCounts(r.Context())
	if err != nil {
		return fmt.Errorf("genre: list: %w", err)
	}

	if h.cache != nil {
		h.cache.Set(cacheKey, genres, cache.TagTitles)
	}

	httputil.WriteJSON(w, http.StatusOK, genres)
	return nil
}
