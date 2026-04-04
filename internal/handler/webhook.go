package handler

import (
	"crypto/subtle"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/service"
)

type WebhookHandler struct {
	plex   *service.PlexService
	secret string
}

func NewWebhookHandler(plex *service.PlexService, secret string) *WebhookHandler {
	return &WebhookHandler{plex: plex, secret: secret}
}

func (h *WebhookHandler) HandlePlex(w http.ResponseWriter, r *http.Request) error {
	token := chi.URLParam(r, "secret")
	if h.secret == "" || subtle.ConstantTimeCompare([]byte(token), []byte(h.secret)) != 1 {
		return httputil.NewAPIError(http.StatusUnauthorized, "Unauthorized")
	}
	// Plex sends multipart/form-data with a "payload" field
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	raw := r.FormValue("payload")
	if raw == "" {
		return httputil.BadRequest("Missing payload")
	}

	payload, err := service.ParsePlexPayload([]byte(raw))
	if err != nil {
		return httputil.BadRequest("Invalid payload")
	}

	if err := h.plex.ProcessScrobble(payload, raw); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
