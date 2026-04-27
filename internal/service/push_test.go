package service_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubPushTransport returns a canned response for every request, regardless
// of URL — webpush-go signs and encrypts the request internally; the only
// thing under test is how PushService reacts to the response.
type stubPushTransport struct {
	status      int
	retryAfter  string
	calls       int
	captured    *http.Request
	cannedError error
}

func (s *stubPushTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.calls++
	s.captured = req
	if s.cannedError != nil {
		return nil, s.cannedError
	}
	h := http.Header{}
	if s.retryAfter != "" {
		h.Set("Retry-After", s.retryAfter)
	}
	return &http.Response{
		StatusCode: s.status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

// seedTestSubscription stores a webpush-go-compatible subscription in
// settings (P256dh / auth values lifted from the library's own tests so the
// encryption pre-flight succeeds before the canned RoundTripper kicks in).
func seedTestSubscription(t *testing.T, db *sql.DB) {
	t.Helper()
	const sub = `{"endpoint":"https://push.example.test/subscriber","keys":{"p256dh":"BNNL5ZaTfK81qhXOx23-wewhigUeFb632jN6LvRWCFH1ubQr77FE_9qV1FuojuRmHP42zmf34rXgW80OvUVDgTk","auth":"zqbxT6JKstKSY9JKibZLSQ"}}`
	testutil.SetSetting(t, db, "push_subscription", sub)
}

func setupPushService(t *testing.T) (*service.PushService, *sql.DB, *repository.SettingRepository) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	svc := service.NewPushService(db, settings, "test-public-key", "test-private-key", "mailto:test@example.com")
	return svc, db, settings
}

func TestPushService_Subscribe(t *testing.T) {
	svc, _, settings := setupPushService(t)

	sub := `{"endpoint":"https://push.example.com/sub1","keys":{"p256dh":"key1","auth":"auth1"}}`
	err := svc.Subscribe(context.Background(), sub)
	require.NoError(t, err)

	// Subscription stored in settings
	stored, err := settings.Get("push_subscription")
	require.NoError(t, err)
	assert.Equal(t, sub, stored)
}

func TestPushService_Unsubscribe(t *testing.T) {
	svc, db, settings := setupPushService(t)

	// Seed a subscription.
	testutil.SetSetting(t, db, "push_subscription", `{"endpoint":"https://push.example.com/sub1"}`)

	err := svc.Unsubscribe(context.Background())
	require.NoError(t, err)

	// Subscription removed
	_, err = settings.Get("push_subscription")
	assert.Error(t, err) // Not found
}

func TestPushService_HasSubscription(t *testing.T) {
	svc, db, _ := setupPushService(t)

	assert.False(t, svc.HasSubscription())

	testutil.SetSetting(t, db, "push_subscription", `{"endpoint":"https://push.example.com/sub1"}`)
	assert.True(t, svc.HasSubscription())
}

func TestPushService_Subscribe_ValidatesJSON(t *testing.T) {
	svc, _, _ := setupPushService(t)

	err := svc.Subscribe(context.Background(), "not json")
	assert.Error(t, err)
}

func TestPushService_Subscribe_RequiresEndpoint(t *testing.T) {
	svc, _, _ := setupPushService(t)

	err := svc.Subscribe(context.Background(), `{"keys":{"p256dh":"key","auth":"auth"}}`)
	assert.Error(t, err)
}

func TestNoopNotifier(t *testing.T) {
	noop := service.NewNoopNotifier()
	assert.False(t, noop.HasSubscription())
	assert.NoError(t, noop.SendNotification(context.Background(), "title", "body", ""))
	assert.Error(t, noop.Subscribe(context.Background(), `{"endpoint":"https://example.com"}`))
	assert.Error(t, noop.Unsubscribe(context.Background()))
}

// TestSendNotification_NoSubscription guards the silent skip path: with no
// subscription stored, SendNotification must return nil without ever
// touching the HTTP client.
func TestSendNotification_NoSubscription(t *testing.T) {
	svc, _, _ := setupPushService(t)
	stub := &stubPushTransport{status: 201}
	defer service.SetPushHTTPClientForTest(&http.Client{Transport: stub})()

	require.NoError(t, svc.SendNotification(context.Background(), "t", "b", "/u"))
	assert.Zero(t, stub.calls, "no subscription → no HTTP call")
}

// TestSendNotification_2xxSuccess covers the happy path — a 201 from the
// push service must surface as nil error and no subscription cleanup.
func TestSendNotification_2xxSuccess(t *testing.T) {
	svc, db, settings := setupPushService(t)
	seedTestSubscription(t, db)
	stub := &stubPushTransport{status: 201}
	defer service.SetPushHTTPClientForTest(&http.Client{Transport: stub})()

	require.NoError(t, svc.SendNotification(context.Background(), "t", "b", "/u"))
	assert.Equal(t, 1, stub.calls)
	_, err := settings.Get("push_subscription")
	assert.NoError(t, err, "successful push must not unsubscribe")
}

// TestSendNotification_410GoneUnsubscribes is the canonical SESSION-14 fix:
// 410 means the browser revoked the subscription; we must silently drop it
// so the next refresh / dead-task / series-ended doesn't burn another call
// against the broken endpoint. A regression that re-introduces a kept-stale
// subscription is invisible — pushes just stop arriving.
func TestSendNotification_410GoneUnsubscribes(t *testing.T) {
	svc, db, settings := setupPushService(t)
	seedTestSubscription(t, db)
	stub := &stubPushTransport{status: http.StatusGone}
	defer service.SetPushHTTPClientForTest(&http.Client{Transport: stub})()

	require.NoError(t, svc.SendNotification(context.Background(), "t", "b", "/u"),
		"410 must NOT propagate as an error — the subscription is dead, not the system")
	_, err := settings.Get("push_subscription")
	assert.Error(t, err, "subscription must be removed after 410")
}

func TestSendNotification_404NotFoundUnsubscribes(t *testing.T) {
	svc, db, settings := setupPushService(t)
	seedTestSubscription(t, db)
	stub := &stubPushTransport{status: http.StatusNotFound}
	defer service.SetPushHTTPClientForTest(&http.Client{Transport: stub})()

	require.NoError(t, svc.SendNotification(context.Background(), "t", "b", "/u"))
	_, err := settings.Get("push_subscription")
	assert.Error(t, err, "subscription must be removed after 404")
}

// TestSendNotification_429ParsesRetryAfter proves the rate-limit signal
// reaches the task scheduler: an APIError with a parsed RetryAfter is what
// IsRateLimitError + ExtractRetryAfter rely on to back the queue off.
func TestSendNotification_429ParsesRetryAfter(t *testing.T) {
	svc, db, settings := setupPushService(t)
	seedTestSubscription(t, db)
	stub := &stubPushTransport{status: http.StatusTooManyRequests, retryAfter: "30"}
	defer service.SetPushHTTPClientForTest(&http.Client{Transport: stub})()

	err := svc.SendNotification(context.Background(), "t", "b", "/u")
	require.Error(t, err)

	var apiErr *matching.APIError
	require.True(t, errors.As(err, &apiErr), "must be a *matching.APIError so the queue can branch on it")
	assert.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	assert.Equal(t, 30*time.Second, apiErr.RetryAfter, "Retry-After must be parsed into the structured field")

	// Subscription must STAY — 429 is transient, not permanent.
	_, err = settings.Get("push_subscription")
	assert.NoError(t, err)
}

func TestSendNotification_5xxReturnsAPIError(t *testing.T) {
	svc, db, settings := setupPushService(t)
	seedTestSubscription(t, db)
	stub := &stubPushTransport{status: http.StatusInternalServerError}
	defer service.SetPushHTTPClientForTest(&http.Client{Transport: stub})()

	err := svc.SendNotification(context.Background(), "t", "b", "/u")
	require.Error(t, err)

	var apiErr *matching.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Zero(t, apiErr.RetryAfter, "no Retry-After header → zero duration")

	_, err = settings.Get("push_subscription")
	assert.NoError(t, err, "5xx is transient, must not unsubscribe")
}
