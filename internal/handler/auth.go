package handler

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Credential string `json:"credential"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verify Google ID token via Google's tokeninfo endpoint
	resp, err := http.Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", body.Credential))
	if err != nil || resp.StatusCode != 200 {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	defer resp.Body.Close()

	var tokenInfo struct {
		Email string `json:"email"`
		Aud   string `json:"aud"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	if tokenInfo.Email != h.allowedEmail || tokenInfo.Aud != h.clientID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	// Issue JWT
	claims := jwt.MapClaims{
		"email": tokenInfo.Email,
		"exp":   jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, h.authCookie(signed))
	w.WriteHeader(http.StatusNoContent)
}

// DevLogin authenticates with username/password for local development.
// Only works when debug login is enabled and credentials are non-default.
func (h *AuthHandler) DevLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&body); err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	userOK := subtle.ConstantTimeCompare([]byte(body.Username), []byte(h.devUser))
	passOK := subtle.ConstantTimeCompare([]byte(body.Password), []byte(h.devPassword))
	if userOK&passOK != 1 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Issue JWT with the allowed email, same as Google OAuth
	claims := jwt.MapClaims{
		"email": h.allowedEmail,
		"exp":   jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	http.SetCookie(w, h.authCookie(signed))
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
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
