package handler

import (
	"net/http"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
)

type StatsHandler struct {
	stats *repository.StatsRepository
}

func NewStatsHandler(stats *repository.StatsRepository) *StatsHandler {
	return &StatsHandler{stats: stats}
}

func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) error {
	resp, err := h.stats.GetAll(r.Context())
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.Header().Set("Cache-Control", "private, max-age=300")
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}
