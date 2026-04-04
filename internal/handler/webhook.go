package handler

import (
	"crypto/subtle"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/service"
)

type WebhookHandler struct {
	plex   *service.PlexService
	secret string
}

func NewWebhookHandler(plex *service.PlexService, secret string) *WebhookHandler {
	return &WebhookHandler{plex: plex, secret: secret}
}

func (h *WebhookHandler) HandlePlex(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "secret")
	if h.secret == "" || subtle.ConstantTimeCompare([]byte(token), []byte(h.secret)) != 1 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
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
		log.Printf("webhook: process scrobble: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
