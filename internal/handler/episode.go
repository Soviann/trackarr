package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type EpisodeHandler struct {
	titles   *repository.TitleRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
}

func NewEpisodeHandler(titles *repository.TitleRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository) *EpisodeHandler {
	return &EpisodeHandler{titles: titles, episodes: episodes, events: events}
}

func (h *EpisodeHandler) ToggleWatched(w http.ResponseWriter, r *http.Request) {
	titleID, err := strconv.ParseInt(chi.URLParam(r, "titleID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid title ID", http.StatusBadRequest)
		return
	}

	episodeID, err := strconv.ParseInt(chi.URLParam(r, "episodeID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid episode ID", http.StatusBadRequest)
		return
	}

	ep, err := h.episodes.ToggleWatched(episodeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log watch event if toggled on
	if ep.Watched {
		h.events.Create(&model.WatchEvent{
			TitleID:   titleID,
			EpisodeID: &episodeID,
			Source:    model.WatchEventSourceManual,
		})
	}

	// Return updated title for status auto-update
	title, _ := h.titles.GetByID(titleID)
	writeJSON(w, title)
}

func (h *EpisodeHandler) BatchMarkWatched(w http.ResponseWriter, r *http.Request) {
	titleID, err := strconv.ParseInt(chi.URLParam(r, "titleID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid title ID", http.StatusBadRequest)
		return
	}

	var body struct {
		EpisodeIDs []int64 `json:"episode_ids"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	if err := h.episodes.BatchMarkWatched(body.EpisodeIDs, now); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log watch events
	for _, epID := range body.EpisodeIDs {
		id := epID
		h.events.Create(&model.WatchEvent{
			TitleID:   titleID,
			EpisodeID: &id,
			Source:    model.WatchEventSourceManual,
		})
	}

	title, _ := h.titles.GetByID(titleID)
	writeJSON(w, title)
}
