package handler_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	plexwebhooks "github.com/hekmon/plexwebhooks"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePlexProcessor stubs the PlexService dependency for handler tests.
type fakePlexProcessor struct {
	err        error
	gotPayload *plexwebhooks.Payload
	gotRaw     string
	calls      int
}

func (f *fakePlexProcessor) ProcessWebhook(_ context.Context, p *plexwebhooks.Payload, raw string) error {
	f.calls++
	f.gotPayload = p
	f.gotRaw = raw
	return f.err
}

func newWebhookRouter(h *handler.WebhookHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/webhook/plex/{secret}", httputil.WrapHandler(h.HandlePlex))
	return r
}

// buildMultipart wraps a JSON payload in a Plex-style multipart body
// (one form field named "payload"). Returns the body and Content-Type header.
func buildMultipart(t *testing.T, payloadJSON string) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormField("payload")
	require.NoError(t, err)
	_, err = fw.Write([]byte(payloadJSON))
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

func TestWebhookHandler_WrongSecret(t *testing.T) {
	h := handler.NewWebhookHandler(&fakePlexProcessor{}, "correct")
	req := httptest.NewRequest("POST", "/webhook/plex/wrong", nil)
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestWebhookHandler_EmptyConfigSecret(t *testing.T) {
	// An empty server-side secret must always reject — this is the deploy-misconfig guard.
	h := handler.NewWebhookHandler(&fakePlexProcessor{}, "")
	req := httptest.NewRequest("POST", "/webhook/plex/anything", nil)
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestWebhookHandler_MatchingSecret_EmptyBody(t *testing.T) {
	// Empty body falls into the JSON-fallback path; json.Unmarshal on []byte{} errors → 400.
	fake := &fakePlexProcessor{}
	h := handler.NewWebhookHandler(fake, "mysecret")
	req := httptest.NewRequest("POST", "/webhook/plex/mysecret", nil)
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, 0, fake.calls)
}

func TestWebhookHandler_Multipart_Valid(t *testing.T) {
	const raw = `{"event":"media.scrobble","Metadata":{"title":"The Matrix","ratingKey":"42"}}`
	body, contentType := buildMultipart(t, raw)

	fake := &fakePlexProcessor{}
	h := handler.NewWebhookHandler(fake, "mysecret")
	req := httptest.NewRequest("POST", "/webhook/plex/mysecret", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 1, fake.calls)
	require.NotNil(t, fake.gotPayload)
	assert.Equal(t, plexwebhooks.EventTypeScrobble, fake.gotPayload.Event)
	assert.Equal(t, "The Matrix", fake.gotPayload.Metadata.Title)
}

func TestWebhookHandler_JSONFallback_Valid(t *testing.T) {
	// Real-world: a reverse proxy that strips multipart and re-emits as application/json.
	const raw = `{"event":"media.scrobble","Metadata":{"title":"Inception"}}`
	fake := &fakePlexProcessor{}
	h := handler.NewWebhookHandler(fake, "mysecret")
	req := httptest.NewRequest("POST", "/webhook/plex/mysecret", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 1, fake.calls)
	assert.Equal(t, plexwebhooks.EventTypeScrobble, fake.gotPayload.Event)
	assert.Equal(t, "Inception", fake.gotPayload.Metadata.Title)
	// Raw payload is the literal request body in the fallback path.
	assert.Equal(t, raw, fake.gotRaw)
}

func TestWebhookHandler_PayloadTooLarge(t *testing.T) {
	// Body of 1 MiB + 1 byte trips MaxBytesReader before json.Unmarshal runs.
	big := strings.Repeat("a", (1<<20)+1)
	fake := &fakePlexProcessor{}
	h := handler.NewWebhookHandler(fake, "mysecret")
	req := httptest.NewRequest("POST", "/webhook/plex/mysecret", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.Equal(t, 0, fake.calls)
}

func TestWebhookHandler_MultipartMalformed(t *testing.T) {
	// Content-Type promises multipart with a specific boundary but body has no parts.
	fake := &fakePlexProcessor{}
	h := handler.NewWebhookHandler(fake, "mysecret")
	req := httptest.NewRequest("POST", "/webhook/plex/mysecret", strings.NewReader("not a multipart body"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=plex-fake-boundary-12345")
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, 0, fake.calls)
}

func TestWebhookHandler_EventTypePassthrough(t *testing.T) {
	// The handler is event-agnostic: it must forward whatever event arrives so
	// PlexService.ProcessWebhook can route on it. A regression that ate a field
	// during parse would only surface here.
	cases := []plexwebhooks.EventType{
		plexwebhooks.EventTypeScrobble,
		plexwebhooks.EventTypePlay,
		plexwebhooks.EventTypePause,
		plexwebhooks.EventTypeStop,
	}
	for _, ev := range cases {
		t.Run(string(ev), func(t *testing.T) {
			raw := `{"event":"` + string(ev) + `","Metadata":{"title":"X"}}`
			body, contentType := buildMultipart(t, raw)
			fake := &fakePlexProcessor{}
			h := handler.NewWebhookHandler(fake, "mysecret")
			req := httptest.NewRequest("POST", "/webhook/plex/mysecret", body)
			req.Header.Set("Content-Type", contentType)
			rr := httptest.NewRecorder()
			newWebhookRouter(h).ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			require.Equal(t, 1, fake.calls)
			assert.Equal(t, ev, fake.gotPayload.Event)
		})
	}
}

func TestWebhookHandler_ProcessorError(t *testing.T) {
	body, contentType := buildMultipart(t, `{"event":"media.scrobble","Metadata":{"title":"X"}}`)
	fake := &fakePlexProcessor{err: errors.New("downstream blew up")}
	h := handler.NewWebhookHandler(fake, "mysecret")
	req := httptest.NewRequest("POST", "/webhook/plex/mysecret", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()
	newWebhookRouter(h).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, 1, fake.calls)
}
