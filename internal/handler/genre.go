package handler

import (
	"fmt"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

// GenreHandler serves genre data for filter UI.
type GenreHandler struct {
	genres *repository.GenreRepository
}

func NewGenreHandler(genres *repository.GenreRepository) *GenreHandler {
	return &GenreHandler{genres: genres}
}

// List returns all genres with title counts, ordered by count descending.
func (h *GenreHandler) List(w http.ResponseWriter, r *http.Request) error {
	genres, err := h.genres.ListWithCounts(r.Context())
	if err != nil {
		return fmt.Errorf("genre: list: %w", err)
	}
	httputil.WriteJSON(w, http.StatusOK, genres)
	return nil
}
