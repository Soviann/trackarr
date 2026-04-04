package handler

import (
	"log"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/repository"
)

type StatsHandler struct {
	stats *repository.StatsRepository
}

func NewStatsHandler(stats *repository.StatsRepository) *StatsHandler {
	return &StatsHandler{stats: stats}
}

func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	resp, err := h.stats.GetAll()
	if err != nil {
		log.Printf("stats: get: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, resp)
}
