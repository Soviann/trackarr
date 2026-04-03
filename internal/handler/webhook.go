package handler

import (
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/service"
)

type WebhookHandler struct {
	plex *service.PlexService
}

func NewWebhookHandler(plex *service.PlexService) *WebhookHandler {
	return &WebhookHandler{plex: plex}
}

func (h *WebhookHandler) HandlePlex(w http.ResponseWriter, r *http.Request) {
	// Plex sends multipart/form-data with a "payload" field
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	raw := r.FormValue("payload")
	if raw == "" {
		http.Error(w, "Missing payload", http.StatusBadRequest)
		return
	}

	payload, err := service.ParsePlexPayload([]byte(raw))
	if err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if err := h.plex.ProcessScrobble(payload, raw); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
