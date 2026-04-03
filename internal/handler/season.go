package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type SeasonHandler struct {
	seasons *repository.SeasonRepository
}

func NewSeasonHandler(seasons *repository.SeasonRepository) *SeasonHandler {
	return &SeasonHandler{seasons: seasons}
}

func (h *SeasonHandler) UpdateRating(w http.ResponseWriter, r *http.Request) {
	seasonID, err := strconv.ParseInt(chi.URLParam(r, "seasonID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid season ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Rating int `json:"rating"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.seasons.UpdateRating(seasonID, body.Rating); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
