package handler

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/go-chi/chi/v5"
	plexwebhooks "github.com/hekmon/plexwebhooks"
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

	payload, rawPayload, err := extractPlexPayload(r)
	if err != nil {
		log.Printf("plex webhook: %v", err)
		return httputil.BadRequest("Invalid request")
	}

	if err := h.plex.ProcessWebhook(payload, rawPayload); err != nil {
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

	// Fallback: proxy may have altered Content-Type
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
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
