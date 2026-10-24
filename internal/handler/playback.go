package handler

import (
	"encoding/json"
	"net/http"

	"github.com/Soviann/trackarr/internal/service"
)

type PlaybackHandler struct {
	hub *service.PlaybackHubService
}

func NewPlaybackHandler(hub *service.PlaybackHubService) *PlaybackHandler {
	return &PlaybackHandler{hub: hub}
}

// GetActivePlayback handles GET /api/playback/active.
// Returns 204 No Content if no media is currently playing.
// Returns 200 OK with the active session JSON if active.
func (h *PlaybackHandler) GetActivePlayback(w http.ResponseWriter, r *http.Request) error {
	if h.hub == nil {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	session := h.hub.GetActiveSession()
	if session == nil {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(session)
}
