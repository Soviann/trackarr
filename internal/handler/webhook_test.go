package handler_test

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
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

// fakePlexProcessor stubs the Plex processor seam for handler tests.
type fakePlexProcessor struct {
	err        error
	gotPayload *model.PlexPayload
	gotRaw     string
	calls      int
}

func (f *fakePlexProcessor) ProcessPlexWebhook(_ context.Context, p *model.PlexPayload, raw string) error {
	f.calls++
	f.gotPayload = p
	f.gotRaw = raw
	return f.err
}

func newWebhookRouter(h *handler.WebhookHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/webhook/jellyfin/{secret}", httputil.WrapHandler(h.HandleJellyfin))
	r.Post("/webhook/plex/{secret}", httputil.WrapHandler(h.HandlePlex))
	return r
}

func TestJellyfinHandler_WrongSecret(t *testing.T) {
	h := handler.NewWebhookHandler(&fakeJellyfinProcessor{}, &fakePlexProcessor{}, "jfsecret", "plexsecret")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/wrong", nil)
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJellyfinHandler_EmptyConfigSecret(t *testing.T) {
	h := handler.NewWebhookHandler(&fakeJellyfinProcessor{}, &fakePlexProcessor{}, "", "")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/anything", nil)
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJellyfinHandler_FallbackSecret(t *testing.T) {
	fake := &fakeJellyfinProcessor{}
	h := handler.NewWebhookHandler(fake, &fakePlexProcessor{}, "primary_secret", "plex_secret", "fallback_secret")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/fallback_secret", strings.NewReader(`{"notification_type":"PlaybackStop","item_type":"Movie","name":"Dune","played_to_completion":"True"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 1, fake.calls)
}

func TestJellyfinHandler_Valid(t *testing.T) {
	const raw = `{"notification_type":"PlaybackStop","item_type":"Movie","name":"Dune","played_to_completion":"True","provider_tmdb":"438631"}`
	fake := &fakeJellyfinProcessor{}
	h := handler.NewWebhookHandler(fake, &fakePlexProcessor{}, "jfsecret", "plexsecret")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/jfsecret", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 1, fake.calls)
	require.NotNil(t, fake.gotPayload)
	assert.Equal(t, "Dune", fake.gotPayload.Name)
	assert.Equal(t, "438631", fake.gotPayload.ProviderTMDB)
	assert.Equal(t, raw, fake.gotRaw)
}

func TestJellyfinHandler_InvalidJSON(t *testing.T) {
	fake := &fakeJellyfinProcessor{}
	h := handler.NewWebhookHandler(fake, &fakePlexProcessor{}, "jfsecret", "plexsecret")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/jfsecret", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, 0, fake.calls)
}

func TestJellyfinHandler_ProcessorError(t *testing.T) {
	const raw = `{"notification_type":"PlaybackStop","item_type":"Movie","name":"Dune","played_to_completion":"True"}`
	fake := &fakeJellyfinProcessor{err: errors.New("downstream error")}
	h := handler.NewWebhookHandler(fake, &fakePlexProcessor{}, "jfsecret", "plexsecret")
	req := httptest.NewRequest("POST", "/webhook/jellyfin/jfsecret", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, 1, fake.calls)
}

// Plex webhook tests

func TestPlexHandler_WrongSecret(t *testing.T) {
	h := handler.NewWebhookHandler(&fakeJellyfinProcessor{}, &fakePlexProcessor{}, "jfsecret", "plexsecret")
	req := httptest.NewRequest("POST", "/webhook/plex/wrong", nil)
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestPlexHandler_MultipartValid(t *testing.T) {
	fake := &fakePlexProcessor{}
	h := handler.NewWebhookHandler(&fakeJellyfinProcessor{}, fake, "jfsecret", "plexsecret")

	const payloadJSON = `{"event":"media.scrobble","Metadata":{"type":"movie","title":"Inception","year":2010,"ratingKey":"1234","Guid":[{"id":"imdb://tt1375666"},{"id":"tmdb://27205"}]}}`

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("payload", payloadJSON))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/webhook/plex/plexsecret", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 1, fake.calls)
	require.NotNil(t, fake.gotPayload)
	assert.Equal(t, "media.scrobble", fake.gotPayload.Event)
	assert.Equal(t, "Inception", fake.gotPayload.Metadata.Title)
	imdb, tmdb, tvdb := fake.gotPayload.Metadata.ExtractExternalIDs()
	assert.Equal(t, "tt1375666", imdb)
	assert.Equal(t, int64(27205), tmdb)
	assert.Equal(t, int64(0), tvdb)
	assert.Equal(t, payloadJSON, fake.gotRaw)
}

func TestPlexHandler_DirectJSON(t *testing.T) {
	fake := &fakePlexProcessor{}
	h := handler.NewWebhookHandler(&fakeJellyfinProcessor{}, fake, "jfsecret", "plexsecret")

	const payloadJSON = `{"event":"media.scrobble","Metadata":{"type":"episode","title":"Episode 1","grandparentTitle":"Show","year":2022,"parentIndex":1,"index":1,"ratingKey":"555","grandparentRatingKey":"500"}}`

	req := httptest.NewRequest("POST", "/webhook/plex/plexsecret", strings.NewReader(payloadJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 1, fake.calls)
	require.NotNil(t, fake.gotPayload)
	assert.Equal(t, "media.scrobble", fake.gotPayload.Event)
	assert.Equal(t, "Show", fake.gotPayload.Metadata.GrandparentTitle)
	assert.Equal(t, 1, fake.gotPayload.Metadata.ParentIndex)
	assert.Equal(t, 1, fake.gotPayload.Metadata.Index)
	assert.Equal(t, payloadJSON, fake.gotRaw)
}

func TestPlexHandler_RealProdPayload(t *testing.T) {
	fake := &fakePlexProcessor{}
	h := handler.NewWebhookHandler(&fakeJellyfinProcessor{}, fake, "jfsecret", "plexsecret")

	// Real raw payload sample from production SQLite watch_events
	const prodPayload = `{"Rating":0,"event":"media.scrobble","user":true,"owner":true,"Account":{"id":2765005,"title":"Sovian"},"Server":{"title":"GITS","uuid":"214b579771792f56ebd4af54b9c3eb4d8000b68e"},"Player":{"local":true,"PublicAddress":"109.9.244.233","title":"LG OLED55C25LB","uuid":"0u61ssozchvtm5z6btqevq11"},"Metadata":{"addedAt":"2026-03-03T18:56:50Z","grandparentRatingKey":"65538","grandparentTitle":"Dr. Globule","Guid":[{"Scheme":"tvdb","Host":"5228295"}],"index":23,"key":"/library/metadata/65562","parentIndex":1,"parentRatingKey":"65539","parentTitle":"Season 1","ratingKey":"65562","title":"Ronchons et dragons","type":"episode","year":1994}}`

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("payload", prodPayload))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/webhook/plex/plexsecret", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 1, fake.calls)
	require.NotNil(t, fake.gotPayload)
	assert.Equal(t, "media.scrobble", fake.gotPayload.Event)
	assert.Equal(t, "Dr. Globule", fake.gotPayload.Metadata.GrandparentTitle)
	assert.Equal(t, "65538", fake.gotPayload.Metadata.GrandparentRatingKey)
	assert.Equal(t, 1, fake.gotPayload.Metadata.ParentIndex)
	assert.Equal(t, 23, fake.gotPayload.Metadata.Index)
	_, _, tvdb := fake.gotPayload.Metadata.ExtractExternalIDs()
	assert.Equal(t, int64(5228295), tvdb)
}
