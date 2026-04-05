package handler

import (
	"crypto/subtle"
	"io"
	"log"
	"net/http"
	"strings"

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
	// Plex sends multipart/form-data with a "payload" field,
	// but some setups (reverse proxies) may alter the Content-Type.
	contentType := r.Header.Get("Content-Type")
	var raw string

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			log.Printf("plex webhook: ParseMultipartForm failed: %v (Content-Type: %s)", err, contentType)
			return httputil.BadRequest("Invalid request")
		}
		raw = r.FormValue("payload")
	} else {
		// Fallback: read raw body (proxy may have altered Content-Type)
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			log.Printf("plex webhook: body read failed: %v", err)
			return httputil.BadRequest("Invalid request")
		}
		raw = strings.TrimSpace(string(body))
		log.Printf("plex webhook: non-multipart request (Content-Type: %s, body length: %d)", contentType, len(raw))
	}

	if raw == "" {
		log.Printf("plex webhook: empty payload (Content-Type: %s)", contentType)
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
