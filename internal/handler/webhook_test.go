package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/stretchr/testify/assert"
)

func setupWebhookHandler(secret string) *handler.WebhookHandler {
	// PlexService = nil — secret checks fail before reaching it
	return handler.NewWebhookHandler(nil, secret)
}

func TestWebhookHandler_WrongSecret(t *testing.T) {
	h := setupWebhookHandler("correct")
	r := chi.NewRouter()
	r.Post("/webhook/plex/{secret}", httputil.WrapHandler(h.HandlePlex))

	req := httptest.NewRequest("POST", "/webhook/plex/wrong", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestWebhookHandler_EmptyConfigSecret(t *testing.T) {
	h := setupWebhookHandler("")
	r := chi.NewRouter()
	r.Post("/webhook/plex/{secret}", httputil.WrapHandler(h.HandlePlex))

	req := httptest.NewRequest("POST", "/webhook/plex/anything", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestWebhookHandler_MatchingSecret_InvalidPayload(t *testing.T) {
	h := setupWebhookHandler("mysecret")
	r := chi.NewRouter()
	r.Post("/webhook/plex/{secret}", httputil.WrapHandler(h.HandlePlex))

	// Empty body — falls into JSON fallback path; json.Unmarshal on empty []byte → error → 400
	req := httptest.NewRequest("POST", "/webhook/plex/mysecret", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
