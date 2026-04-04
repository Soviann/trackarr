package handler

import (
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type SeasonHandler struct {
	seasons *repository.SeasonRepository
}

func NewSeasonHandler(seasons *repository.SeasonRepository) *SeasonHandler {
	return &SeasonHandler{seasons: seasons}
}

func (h *SeasonHandler) UpdateRating(w http.ResponseWriter, r *http.Request) error {
	seasonID, err := httputil.ParseIDParam(r, "seasonID")
	if err != nil {
		return httputil.BadRequest("Invalid season ID")
	}

	var body struct {
		Rating int `json:"rating"`
	}
	if err := httputil.ReadJSON(r, &body, 1024); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	if err := h.seasons.UpdateRating(seasonID, body.Rating); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
