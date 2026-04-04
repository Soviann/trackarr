package handler

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
)

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

	// Verify Google ID token via Google's tokeninfo endpoint
	resp, err := http.Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", body.Credential))
	if err != nil || resp.StatusCode != 200 {
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
		"exp":   jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
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

	userOK := subtle.ConstantTimeCompare([]byte(body.Username), []byte(h.devUser))
	passOK := subtle.ConstantTimeCompare([]byte(body.Password), []byte(h.devPassword))
	if userOK&passOK != 1 {
		return httputil.NotFound("Not found")
	}

	// Issue JWT with the allowed email, same as Google OAuth
	claims := jwt.MapClaims{
		"email": h.allowedEmail,
		"exp":   jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
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
