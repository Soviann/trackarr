package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
)

type StatsHandler struct {
	stats    *repository.StatsRepository
	wrapped  *repository.WrappedRepository
	pipeline *matching.Pipeline
}

func NewStatsHandler(stats *repository.StatsRepository, wrapped *repository.WrappedRepository, pipeline *matching.Pipeline) *StatsHandler {
	return &StatsHandler{
		stats:    stats,
		wrapped:  wrapped,
		pipeline: pipeline,
	}
}

func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) error {
	timeframe := r.URL.Query().Get("timeframe")
	if timeframe == "" {
		timeframe = "all"
	}
	var year int
	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
			year = y
		}
	}
	mediaType := r.URL.Query().Get("media_type")
	if mediaType == "" {
		mediaType = r.URL.Query().Get("type")
	}
	if mediaType == "" {
		mediaType = "all"
	}

	filter := model.StatsFilter{
		Timeframe: timeframe,
		Year:      year,
		MediaType: mediaType,
	}

	resp, err := h.stats.GetFiltered(r.Context(), filter)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.Header().Set("Cache-Control", "private, max-age=300")
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *StatsHandler) GetWrapped(w http.ResponseWriter, r *http.Request) error {
	yearStr := r.URL.Query().Get("year")
	targetYear := time.Now().Year()
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
			targetYear = y
		}
	}

	// 1. Check if snapshot exists in database
	if h.wrapped != nil {
		snap, _, err := h.wrapped.GetSnapshot(r.Context(), targetYear)
		if err == nil && snap != nil {
			// Refresh available_years list in case new years were added since snapshot
			if years, yErr := h.stats.AvailableYears(r.Context()); yErr == nil && len(years) > 0 {
				snap.AvailableYears = years
			}
			httputil.WriteJSON(w, http.StatusOK, snap)
			return nil
		}
	}

	// 2. Generate on-the-fly if not snapshotted
	rawStats, resp, err := h.stats.GetWrappedData(r.Context(), targetYear)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	var isFallback bool
	var persona *model.WrappedAIPersona
	if h.pipeline != nil && h.pipeline.AI() != nil {
		aiCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		var aiErr error
		persona, aiErr = h.pipeline.AI().GenerateWrappedStory(aiCtx, rawStats)
		cancel()
		if aiErr != nil {
			persona = nil
			isFallback = true
		}
	}
	if persona == nil {
		persona = matching.FallbackWrappedPersona(rawStats)
	}
	resp.Persona = *persona

	// If it's a past year, freeze and save the snapshot only if AI succeeded or AI is not configured.
	// This prevents permanently freezing an offline fallback if Gemini was temporarily down.
	aiConfigured := h.pipeline != nil && h.pipeline.AI() != nil
	if h.wrapped != nil && targetYear < time.Now().Year() && resp.Overview.TotalTitles > 0 && (!aiConfigured || !isFallback) {
		_ = h.wrapped.SaveSnapshot(r.Context(), targetYear, resp)
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// GetWrappedArchives returns the list of all stored Wrapped snapshots for the gallery.
func (h *StatsHandler) GetWrappedArchives(w http.ResponseWriter, r *http.Request) error {
	if h.wrapped == nil {
		httputil.WriteJSON(w, http.StatusOK, []model.WrappedArchiveItem{})
		return nil
	}

	archives, err := h.wrapped.ListArchives(r.Context())
	if err != nil {
		return httputil.InternalError("Failed to list wrapped archives", err)
	}

	if archives == nil {
		archives = []model.WrappedArchiveItem{}
	}

	httputil.WriteJSON(w, http.StatusOK, archives)
	return nil
}

// RegenerateWrapped forces re-computation of a Wrapped snapshot and persists it.
func (h *StatsHandler) RegenerateWrapped(w http.ResponseWriter, r *http.Request) error {
	yearStr := r.URL.Query().Get("year")
	targetYear := time.Now().Year()
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
			targetYear = y
		}
	}

	rawStats, resp, err := h.stats.GetWrappedData(r.Context(), targetYear)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	var regenPersona *model.WrappedAIPersona
	if h.pipeline != nil && h.pipeline.AI() != nil {
		var pErr error
		regenPersona, pErr = h.pipeline.AI().GenerateWrappedStory(r.Context(), rawStats)
		if pErr != nil {
			regenPersona = nil
		}
	}
	if regenPersona == nil {
		regenPersona = matching.FallbackWrappedPersona(rawStats)
	}
	resp.Persona = *regenPersona

	if h.wrapped != nil && resp.Overview.TotalTitles > 0 {
		if err := h.wrapped.SaveSnapshot(r.Context(), targetYear, resp); err != nil {
			return httputil.InternalError(fmt.Sprintf("Failed to save snapshot for %d", targetYear), err)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}
