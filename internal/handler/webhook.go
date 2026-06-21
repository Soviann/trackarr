package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	plexwebhooks "github.com/hekmon/plexwebhooks"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
)

// plexProcessor is the seam between the handler and PlexService — keeps the
// handler test-stubbable without dragging the full service surface into the
// test target. *service.PlexService satisfies it.
type plexProcessor interface {
	ProcessWebhook(ctx context.Context, payload *plexwebhooks.Payload, rawPayload string) error
}

// jellyfinProcessor is the equivalent seam for Jellyfin webhooks.
// *service.PlexService (built via NewJellyfinService) satisfies it.
type jellyfinProcessor interface {
	ProcessJellyfinWebhook(ctx context.Context, payload *model.JellyfinPayload, rawPayload string) error
}

// webhookProcessingTimeout caps how long a single Plex webhook may run.
// Plex itself retries aggressively and our payloads only touch SQLite + optional
// TMDB/push I/O, so anything longer than this points to a stuck goroutine that
// would otherwise hold the sole writeDB connection indefinitely.
const webhookProcessingTimeout = 30 * time.Second

// webhookMaxBodyBytes caps the total payload size a Plex webhook can push. Real
// webhook bodies are a few KB (JSON metadata + optional thumbnail part); 1 MiB
// leaves headroom while preventing a malicious or misconfigured proxy from
// OOM-ing us through the multipart branch, which previously read r.Body
// unbounded. The fallback non-multipart branch already capped at 1 MiB.
const webhookMaxBodyBytes int64 = 1 << 20

type WebhookHandler struct {
	plex           plexProcessor
	secret         string
	jellyfin       jellyfinProcessor
	jellyfinSecret string
}

func NewWebhookHandler(plex plexProcessor, secret string) *WebhookHandler {
	return &WebhookHandler{plex: plex, secret: secret}
}

// WithJellyfin wires the Jellyfin webhook path onto the same handler. Kept
// separate from NewWebhookHandler so the Plex constructor (and its tests) stay
// untouched. Returns the handler for chaining.
func (h *WebhookHandler) WithJellyfin(jellyfin jellyfinProcessor, secret string) *WebhookHandler {
	h.jellyfin = jellyfin
	h.jellyfinSecret = secret
	return h
}

func (h *WebhookHandler) HandlePlex(w http.ResponseWriter, r *http.Request) error {
	token := chi.URLParam(r, "secret")
	if h.secret == "" || subtle.ConstantTimeCompare([]byte(token), []byte(h.secret)) != 1 {
		return httputil.NewAPIError(http.StatusUnauthorized, "Unauthorized")
	}

	r.Body = http.MaxBytesReader(w, r.Body, webhookMaxBodyBytes)
	payload, rawPayload, err := extractPlexPayload(r)
	if err != nil {
		var mbErr *http.MaxBytesError
		if errors.As(err, &mbErr) {
			log.Printf("plex webhook: payload exceeds %d bytes", mbErr.Limit)
			return httputil.NewAPIError(http.StatusRequestEntityTooLarge, "Payload too large")
		}
		log.Printf("plex webhook: %v", err)
		return httputil.BadRequest("Invalid request")
	}

	ctx, cancel := context.WithTimeout(r.Context(), webhookProcessingTimeout)
	defer cancel()

	if err := h.plex.ProcessWebhook(ctx, payload, rawPayload); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

// HandleJellyfin ingests a notification from the Jellyfin Webhook plugin's
// Generic Destination. Unlike Plex, the body is always plain JSON (the template
// we ship — see docs/user-guide.md), so there is no multipart branch.
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

// extractPlexPayload extracts the Plex webhook payload using the plexwebhooks library.
// Falls back to raw JSON body parsing if the request is not multipart.
func extractPlexPayload(r *http.Request) (*plexwebhooks.Payload, string, error) {
	contentType := r.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)

	if err == nil && mediaType == "multipart/form-data" && params["boundary"] != "" {
		mr := multipart.NewReader(r.Body, params["boundary"])
		payload, _, err := plexwebhooks.Extract(mr)
		if err != nil {
			return nil, "", err
		}
		// Re-serialize for raw storage
		raw, _ := json.Marshal(payload)
		return payload, string(raw), nil
	}

	// Fallback: proxy may have altered Content-Type. r.Body is already bounded
	// by MaxBytesReader in HandlePlex, so ReadAll surfaces *http.MaxBytesError
	// on overflow instead of silently truncating.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, "", err
	}
	log.Printf("plex webhook: non-multipart request (Content-Type: %s, body length: %d)", contentType, len(body))

	var payload plexwebhooks.Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", err
	}
	return &payload, string(body), nil
}
