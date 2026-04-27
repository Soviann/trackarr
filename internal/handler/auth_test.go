package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testJWTSecret    = "test-secret-with-enough-bytes-32"
	testAllowedEmail = "owner@example.com"
	testClientID     = "client-id.apps.googleusercontent.com"
)

func fakeTokenInfoServer(t *testing.T, status int, body string, delay time.Duration) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func extractAuthCookie(t *testing.T, rr *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rr.Result().Cookies() {
		if c.Name == "token" {
			return c
		}
	}
	t.Fatalf("no token cookie set; cookies=%v", rr.Result().Cookies())
	return nil
}

func requireAPIError(t *testing.T, err error) *httputil.APIError {
	t.Helper()
	require.Error(t, err)
	var apiErr *httputil.APIError
	require.True(t, errors.As(err, &apiErr), "expected *httputil.APIError, got %T", err)
	return apiErr
}

func assertJWTSignedFor(t *testing.T, raw, secret, expectedEmail string) {
	t.Helper()
	tok, err := jwt.Parse(raw, func(_ *jwt.Token) (any, error) { return []byte(secret), nil })
	require.NoError(t, err)
	require.True(t, tok.Valid)
	claims, ok := tok.Claims.(jwt.MapClaims)
	require.True(t, ok)
	assert.Equal(t, expectedEmail, claims["email"])
}

func TestLogout_ClearsCookie(t *testing.T) {
	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false)
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	rr := httptest.NewRecorder()

	require.NoError(t, h.Logout(rr, req))

	assert.Equal(t, http.StatusNoContent, rr.Code)
	cookies := rr.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "token", cookies[0].Name)
	assert.Equal(t, "", cookies[0].Value)
	assert.True(t, cookies[0].MaxAge < 0)
}

func TestGoogleCallback_InvalidJSON(t *testing.T) {
	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false)
	req := httptest.NewRequest("POST", "/api/auth/google", strings.NewReader("not json"))
	rr := httptest.NewRecorder()

	apiErr := requireAPIError(t, h.GoogleCallback(rr, req))
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestGoogleCallback_BodyOverLimitTruncatedToInvalidJSON(t *testing.T) {
	// 4 KiB cap on the request body: a body that's huge but starts valid is
	// truncated by io.LimitReader, JSON decode fails on the cut → 400. Guards
	// against future drift that drops the LimitReader.
	body := `{"credential":"` + strings.Repeat("a", 5000) + `"}`
	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false)
	req := httptest.NewRequest("POST", "/api/auth/google", strings.NewReader(body))
	rr := httptest.NewRecorder()

	apiErr := requireAPIError(t, h.GoogleCallback(rr, req))
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestGoogleCallback_TokenInfoNon200(t *testing.T) {
	srv := fakeTokenInfoServer(t, http.StatusUnauthorized, `{}`, 0)
	defer srv.Close()
	defer handler.SetGoogleTokenInfoURLForTest(srv.URL)()

	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false)
	req := httptest.NewRequest("POST", "/api/auth/google", strings.NewReader(`{"credential":"forged"}`))
	rr := httptest.NewRecorder()

	apiErr := requireAPIError(t, h.GoogleCallback(rr, req))
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

func TestGoogleCallback_EmailMismatch(t *testing.T) {
	srv := fakeTokenInfoServer(t, http.StatusOK,
		`{"email":"intruder@example.com","aud":"`+testClientID+`"}`, 0)
	defer srv.Close()
	defer handler.SetGoogleTokenInfoURLForTest(srv.URL)()

	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false)
	req := httptest.NewRequest("POST", "/api/auth/google", strings.NewReader(`{"credential":"valid-token"}`))
	rr := httptest.NewRecorder()

	apiErr := requireAPIError(t, h.GoogleCallback(rr, req))
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
}

func TestGoogleCallback_AudMismatch(t *testing.T) {
	// A token valid for another OAuth client must be rejected — `aud` is the
	// cross-app forgery defense.
	srv := fakeTokenInfoServer(t, http.StatusOK,
		`{"email":"`+testAllowedEmail+`","aud":"some-other-client.apps.googleusercontent.com"}`, 0)
	defer srv.Close()
	defer handler.SetGoogleTokenInfoURLForTest(srv.URL)()

	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false)
	req := httptest.NewRequest("POST", "/api/auth/google", strings.NewReader(`{"credential":"valid-token"}`))
	rr := httptest.NewRecorder()

	apiErr := requireAPIError(t, h.GoogleCallback(rr, req))
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
}

func TestGoogleCallback_Success(t *testing.T) {
	srv := fakeTokenInfoServer(t, http.StatusOK,
		`{"email":"`+testAllowedEmail+`","aud":"`+testClientID+`"}`, 0)
	defer srv.Close()
	defer handler.SetGoogleTokenInfoURLForTest(srv.URL)()

	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, true)
	req := httptest.NewRequest("POST", "/api/auth/google", strings.NewReader(`{"credential":"valid-token"}`))
	rr := httptest.NewRecorder()

	require.NoError(t, h.GoogleCallback(rr, req))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	cookie := extractAuthCookie(t, rr)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure, "Secure flag must follow cookieSecure=true")
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assertJWTSignedFor(t, cookie.Value, testJWTSecret, testAllowedEmail)
}

func TestGoogleCallback_UpstreamTimeout(t *testing.T) {
	// Server sleeps longer than the test client's 100ms timeout — the handler
	// must NOT pin its goroutine waiting for Google.
	srv := fakeTokenInfoServer(t, http.StatusOK, `{}`, 200*time.Millisecond)
	defer srv.Close()
	defer handler.SetGoogleTokenInfoURLForTest(srv.URL)()
	defer handler.SetGoogleAuthClientForTest(&http.Client{Timeout: 100 * time.Millisecond})()

	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false)
	req := httptest.NewRequest("POST", "/api/auth/google", strings.NewReader(`{"credential":"valid-token"}`))
	rr := httptest.NewRecorder()

	start := time.Now()
	apiErr := requireAPIError(t, h.GoogleCallback(rr, req))
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)
	assert.Less(t, elapsed, time.Second, "handler must surface client timeout, not block on default")
}

func TestGoogleCallback_TokenInfoBadJSON(t *testing.T) {
	srv := fakeTokenInfoServer(t, http.StatusOK, `<<not json>>`, 0)
	defer srv.Close()
	defer handler.SetGoogleTokenInfoURLForTest(srv.URL)()

	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false)
	req := httptest.NewRequest("POST", "/api/auth/google", strings.NewReader(`{"credential":"valid-token"}`))
	rr := httptest.NewRecorder()

	apiErr := requireAPIError(t, h.GoogleCallback(rr, req))
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

func TestDevLogin_WrongPassword(t *testing.T) {
	// 404, not 401: DevLogin must not confirm the route exists when creds fail.
	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false).
		WithDevLogin("dev", "correct-password")
	req := httptest.NewRequest("POST", "/api/auth/dev", strings.NewReader(`{"username":"dev","password":"WRONG"}`))
	rr := httptest.NewRecorder()

	apiErr := requireAPIError(t, h.DevLogin(rr, req))
	assert.Equal(t, http.StatusNotFound, apiErr.Status)
}

func TestDevLogin_WrongUser(t *testing.T) {
	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false).
		WithDevLogin("dev", "correct-password")
	req := httptest.NewRequest("POST", "/api/auth/dev", strings.NewReader(`{"username":"intruder","password":"correct-password"}`))
	rr := httptest.NewRecorder()

	apiErr := requireAPIError(t, h.DevLogin(rr, req))
	assert.Equal(t, http.StatusNotFound, apiErr.Status)
}

func TestDevLogin_Success(t *testing.T) {
	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false).
		WithDevLogin("dev", "correct-password")
	req := httptest.NewRequest("POST", "/api/auth/dev", strings.NewReader(`{"username":"dev","password":"correct-password"}`))
	rr := httptest.NewRecorder()

	require.NoError(t, h.DevLogin(rr, req))
	assert.Equal(t, http.StatusNoContent, rr.Code)
	cookie := extractAuthCookie(t, rr)
	assertJWTSignedFor(t, cookie.Value, testJWTSecret, testAllowedEmail)
}

func TestDevLogin_InvalidJSON(t *testing.T) {
	h := handler.NewAuthHandler(testJWTSecret, testAllowedEmail, testClientID, false).
		WithDevLogin("dev", "correct-password")
	req := httptest.NewRequest("POST", "/api/auth/dev", strings.NewReader("not json"))
	rr := httptest.NewRecorder()

	apiErr := requireAPIError(t, h.DevLogin(rr, req))
	assert.Equal(t, http.StatusNotFound, apiErr.Status)
}
