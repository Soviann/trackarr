package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeJellyfinProcessor stubs the Jellyfin processor seam for handler tests.
type fakeJellyfinProcessor struct {
	err        error
	gotPayload *model.JellyfinPayload
	gotRaw     string
	calls      int
}

func (f *fakeJellyfinProcessor) ProcessJellyfinWebhook(_ context.Context, p *model.JellyfinPayload, raw string) error {
	f.calls++
	f.gotPayload = p
	f.gotRaw = raw
	return f.err
}

func newJellyfinRouter(h *handler.WebhookHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/webhook/jellyfin/{secret}", httputil.WrapHandler(h.HandleJellyfin))
	return r
}

func TestJellyfinHandler_WrongSecret(t *testing.T) {
	h := handler.NewWebhookHandler(&fakeJellyfinProcessor{}, "jfsecret")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/wrong", nil)
	rr := httptest.NewRecorder()
	newJellyfinRouter(h).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJellyfinHandler_EmptyConfigSecret(t *testing.T) {
	h := handler.NewWebhookHandler(&fakeJellyfinProcessor{}, "")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/anything", nil)
	rr := httptest.NewRecorder()
	newJellyfinRouter(h).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJellyfinHandler_FallbackSecret(t *testing.T) {
	fake := &fakeJellyfinProcessor{}
	h := handler.NewWebhookHandler(fake, "primary_secret", "fallback_secret")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/fallback_secret", strings.NewReader(`{"notification_type":"PlaybackStop","item_type":"Movie","name":"Dune","played_to_completion":"True"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newJellyfinRouter(h).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 1, fake.calls)
}

func TestJellyfinHandler_Valid(t *testing.T) {
	const raw = `{"notification_type":"PlaybackStop","item_type":"Movie","name":"Dune","played_to_completion":"True","provider_tmdb":"438631"}`
	fake := &fakeJellyfinProcessor{}
	h := handler.NewWebhookHandler(fake, "jfsecret")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/jfsecret", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newJellyfinRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 1, fake.calls)
	require.NotNil(t, fake.gotPayload)
	assert.Equal(t, "Dune", fake.gotPayload.Name)
	assert.Equal(t, "438631", fake.gotPayload.ProviderTMDB)
	assert.Equal(t, raw, fake.gotRaw)
}

func TestJellyfinHandler_InvalidJSON(t *testing.T) {
	fake := &fakeJellyfinProcessor{}
	h := handler.NewWebhookHandler(fake, "jfsecret")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/jfsecret", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newJellyfinRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, 0, fake.calls)
}

func TestJellyfinHandler_ProcessorError(t *testing.T) {
	const raw = `{"notification_type":"PlaybackStop","item_type":"Movie","name":"Dune","played_to_completion":"True"}`
	fake := &fakeJellyfinProcessor{err: errors.New("downstream error")}
	h := handler.NewWebhookHandler(fake, "jfsecret")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/jfsecret", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newJellyfinRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, 1, fake.calls)
}
