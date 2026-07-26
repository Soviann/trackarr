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
}

func NewWebhookHandler(jellyfin jellyfinProcessor, secret string) *WebhookHandler {
	return &WebhookHandler{
		jellyfin:       jellyfin,
		jellyfinSecret: secret,
	}
}

// HandleJellyfin ingests a notification from the Jellyfin Webhook plugin's
// Generic Destination. The body is always plain JSON.
func (h *WebhookHandler) HandleJellyfin(w http.ResponseWriter, r *http.Request) error {
	token := chi.URLParam(r, "secret")
	if h.jellyfin == nil || h.jellyfinSecret == "" || subtle.ConstantTimeCompare([]byte(token), []byte(h.jellyfinSecret)) != 1 {
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
