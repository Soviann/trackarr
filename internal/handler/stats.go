package handler

import (
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type StatsHandler struct {
	stats *repository.StatsRepository
}

func NewStatsHandler(stats *repository.StatsRepository) *StatsHandler {
	return &StatsHandler{stats: stats}
}

func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) error {
	resp, err := h.stats.GetAll()
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.Header().Set("Cache-Control", "private, max-age=300")
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}
