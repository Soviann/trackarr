package handler

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
)

// googleAuthClient caps every outbound call to Google's tokeninfo endpoint at
// 5 seconds. The default `http.Client` has no timeout; without this cap a
// stalled upstream (Google outage, network blackhole) would pin each auth
// handler goroutine until the rate limiter itself fills up, which is
// indistinguishable from an attack against the login route.
var googleAuthClient = &http.Client{Timeout: 5 * time.Second}

// googleTokenInfoURL is overridden by tests to point at a httptest.Server.
var googleTokenInfoURL = "https://oauth2.googleapis.com/tokeninfo"

type AuthHandler struct {
	jwtSecret    string
	allowedEmail string
	clientID     string
	cookieSecure bool
	devUser      string
	devPassword  string
}

func NewAuthHandler(jwtSecret, allowedEmail, clientID string, cookieSecure bool) *AuthHandler {
	return &AuthHandler{
		jwtSecret:    jwtSecret,
		allowedEmail: allowedEmail,
		clientID:     clientID,
		cookieSecure: cookieSecure,
	}
}

func (h *AuthHandler) WithDevLogin(user, password string) *AuthHandler {
	h.devUser = user
	h.devPassword = password
	return h
}

// GoogleCallback verifies the Google ID token and issues a JWT cookie.
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Credential string `json:"credential"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	// Verify Google ID token via Google's tokeninfo endpoint. NewRequestWithContext
	// ties the upstream call to the HTTP request so a client disconnect aborts
	// the verify immediately, and googleAuthClient adds a hard 5s timeout on top.
	// url.Values escape any & or = in the credential — fmt.Sprintf would have
	// let a forged token splice extra query params into the request.
	q := url.Values{"id_token": []string{body.Credential}}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, googleTokenInfoURL+"?"+q.Encode(), nil)
	if err != nil {
		return httputil.NewAPIError(http.StatusUnauthorized, "Invalid token")
	}
	resp, err := googleAuthClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return httputil.NewAPIError(http.StatusUnauthorized, "Invalid token")
	}
	defer resp.Body.Close()

	var tokenInfo struct {
		Email string `json:"email"`
		Aud   string `json:"aud"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return httputil.NewAPIError(http.StatusUnauthorized, "Invalid token")
	}

	if tokenInfo.Email != h.allowedEmail || tokenInfo.Aud != h.clientID {
		return httputil.NewAPIError(http.StatusForbidden, "Unauthorized")
	}

	// Issue JWT
	claims := jwt.MapClaims{
		"email": tokenInfo.Email,
		"exp":   jwt.NewNumericDate(time.Now().UTC().Add(30 * 24 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	http.SetCookie(w, h.authCookie(signed))
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// DevLogin authenticates with username/password for local development.
// Only works when debug login is enabled and credentials are non-default.
func (h *AuthHandler) DevLogin(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&body); err != nil {
		return httputil.NotFound("Not found")
	}

	// Hash both sides before constant-time compare. Comparing raw strings of
	// different lengths leaks the expected length via the early-out: an
	// attacker who notices "I get 401 faster on a 12-byte guess than on a
	// 14-byte one" learns the password is 12 chars long.
	userHash := sha256.Sum256([]byte(body.Username))
	expectedUserHash := sha256.Sum256([]byte(h.devUser))
	passHash := sha256.Sum256([]byte(body.Password))
	expectedPassHash := sha256.Sum256([]byte(h.devPassword))
	userOK := subtle.ConstantTimeCompare(userHash[:], expectedUserHash[:])
	passOK := subtle.ConstantTimeCompare(passHash[:], expectedPassHash[:])
	if userOK&passOK != 1 {
		return httputil.NotFound("Not found")
	}

	// Issue JWT with the allowed email, same as Google OAuth
	claims := jwt.MapClaims{
		"email": h.allowedEmail,
		"exp":   jwt.NewNumericDate(time.Now().UTC().Add(30 * 24 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return httputil.NotFound("Not found")
	}

	http.SetCookie(w, h.authCookie(signed))
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *AuthHandler) authCookie(token string) *http.Cookie {
	return &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 3600,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}
