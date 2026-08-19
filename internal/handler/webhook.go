package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
)

type jellyfinProcessor interface {
	ProcessJellyfinWebhook(ctx context.Context, payload *model.JellyfinPayload, rawPayload string) error
}

const webhookProcessingTimeout = 30 * time.Second
const webhookMaxBodyBytes int64 = 1 << 20

type WebhookHandler struct {
	jellyfin       jellyfinProcessor
	jellyfinSecret string
	fallbackSecret string
}

func NewWebhookHandler(jellyfin jellyfinProcessor, secret string, fallbackSecret ...string) *WebhookHandler {
	var fallback string
	if len(fallbackSecret) > 0 {
		fallback = fallbackSecret[0]
	}
	return &WebhookHandler{
		jellyfin:       jellyfin,
		jellyfinSecret: secret,
		fallbackSecret: fallback,
	}
}

func (h *WebhookHandler) isAuthorized(token string) bool {
	if h.jellyfinSecret != "" && subtle.ConstantTimeCompare([]byte(token), []byte(h.jellyfinSecret)) == 1 {
		return true
	}
	if h.fallbackSecret != "" && subtle.ConstantTimeCompare([]byte(token), []byte(h.fallbackSecret)) == 1 {
		return true
	}
	return false
}

// HandleJellyfin ingests a notification from the Jellyfin Webhook plugin's
// Generic Destination. The body is always plain JSON.
func (h *WebhookHandler) HandleJellyfin(w http.ResponseWriter, r *http.Request) error {
	token := chi.URLParam(r, "secret")
	if h.jellyfin == nil || !h.isAuthorized(token) {
		if h.jellyfinSecret == "" && h.fallbackSecret == "" {
			log.Printf("jellyfin webhook: rejected request from %s — JELLYFIN_WEBHOOK_SECRET is not configured", r.RemoteAddr)
		} else {
			prefix := token
			if len(prefix) > 4 {
				prefix = prefix[:4] + "..."
			}
			log.Printf("jellyfin webhook: rejected unauthorized request from %s — secret token mismatch (received len=%d '%s', expected len=%d)", r.RemoteAddr, len(token), prefix, len(h.jellyfinSecret))
		}
		return httputil.NewAPIError(http.StatusUnauthorized, "Unauthorized")
	}

	r.Body = http.MaxBytesReader(w, r.Body, webhookMaxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var mbErr *http.MaxBytesError
		if errors.As(err, &mbErr) {
			log.Printf("jellyfin webhook: payload exceeds %d bytes", mbErr.Limit)
			return httputil.NewAPIError(http.StatusRequestEntityTooLarge, "Payload too large")
		}
		log.Printf("jellyfin webhook: %v", err)
		return httputil.BadRequest("Invalid request")
	}

	var payload model.JellyfinPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("jellyfin webhook: %v", err)
		return httputil.BadRequest("Invalid request")
	}

	ctx, cancel := context.WithTimeout(r.Context(), webhookProcessingTimeout)
	defer cancel()

	if err := h.jellyfin.ProcessJellyfinWebhook(ctx, &payload, string(body)); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
