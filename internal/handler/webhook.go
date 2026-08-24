package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
)

type jellyfinProcessor interface {
	ProcessJellyfinWebhook(ctx context.Context, payload *model.JellyfinPayload, rawPayload string) error
}

type plexProcessor interface {
	ProcessPlexWebhook(ctx context.Context, payload *model.PlexPayload, rawPayload string) error
}

const webhookProcessingTimeout = 30 * time.Second
const webhookMaxBodyBytes int64 = 10 << 20 // 10 MiB to accommodate multipart Plex requests with poster thumbs

type WebhookHandler struct {
	mu             sync.RWMutex
	jellyfin       jellyfinProcessor
	jellyfinSecret string
	plex           plexProcessor
	plexSecret     string
	fallbackSecret string
}

func NewWebhookHandler(jellyfin jellyfinProcessor, plex plexProcessor, jellyfinSecret, plexSecret string, fallbackSecret ...string) *WebhookHandler {
	var fallback string
	if len(fallbackSecret) > 0 {
		fallback = fallbackSecret[0]
	}
	return &WebhookHandler{
		jellyfin:       jellyfin,
		jellyfinSecret: jellyfinSecret,
		plex:           plex,
		plexSecret:     plexSecret,
		fallbackSecret: fallback,
	}
}

// SetSecrets updates webhook secret tokens dynamically at runtime.
func (h *WebhookHandler) SetSecrets(jellyfinSecret, plexSecret string, fallbackSecret ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.jellyfinSecret = jellyfinSecret
	h.plexSecret = plexSecret
	if len(fallbackSecret) > 0 {
		h.fallbackSecret = fallbackSecret[0]
	}
}

func (h *WebhookHandler) isJellyfinAuthorized(token string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.jellyfinSecret != "" && subtle.ConstantTimeCompare([]byte(token), []byte(h.jellyfinSecret)) == 1 {
		return true
	}
	if h.fallbackSecret != "" && subtle.ConstantTimeCompare([]byte(token), []byte(h.fallbackSecret)) == 1 {
		return true
	}
	return false
}

func (h *WebhookHandler) isPlexAuthorized(token string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.plexSecret != "" && subtle.ConstantTimeCompare([]byte(token), []byte(h.plexSecret)) == 1 {
		return true
	}
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
	if h.jellyfin == nil || !h.isJellyfinAuthorized(token) {
		if h.jellyfinSecret == "" && h.fallbackSecret == "" {
			log.Printf("jellyfin webhook: rejected request from %s — JELLYFIN_WEBHOOK_SECRET is not configured", r.RemoteAddr)
		} else {
			log.Printf("jellyfin webhook: rejected unauthorized request from %s — secret token mismatch", r.RemoteAddr)
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

// HandlePlex ingests a webhook from Plex Media Server.
// Supports multipart/form-data (with form field 'payload') and raw application/json.
func (h *WebhookHandler) HandlePlex(w http.ResponseWriter, r *http.Request) error {
	token := chi.URLParam(r, "secret")
	if h.plex == nil || !h.isPlexAuthorized(token) {
		if h.plexSecret == "" && h.jellyfinSecret == "" && h.fallbackSecret == "" {
			log.Printf("plex webhook: rejected request from %s — webhook secret is not configured", r.RemoteAddr)
		} else {
			log.Printf("plex webhook: rejected unauthorized request from %s — secret token mismatch", r.RemoteAddr)
		}
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

	if err := h.plex.ProcessPlexWebhook(ctx, payload, rawPayload); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

func extractPlexPayload(r *http.Request) (*model.PlexPayload, string, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		if err := r.ParseMultipartForm(webhookMaxBodyBytes); err != nil {
			return nil, "", fmt.Errorf("parse multipart form: %w", err)
		}
		payloadStr := r.FormValue("payload")
		if payloadStr == "" {
			return nil, "", errors.New("missing payload form field in multipart request")
		}
		var payload model.PlexPayload
		if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
			return nil, "", fmt.Errorf("unmarshal plex payload: %w", err)
		}
		return &payload, payloadStr, nil
	}

	// Fallback: direct JSON body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}
	var payload model.PlexPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", fmt.Errorf("unmarshal plex payload: %w", err)
	}
	return &payload, string(body), nil
}
