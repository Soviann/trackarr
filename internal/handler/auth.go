package handler

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// googleAuthClient caps every outbound call to Google's tokeninfo endpoint at 5 seconds.
var googleAuthClient = &http.Client{Timeout: 5 * time.Second}

// googleTokenInfoURL is overridden by tests to point at a httptest.Server.
var googleTokenInfoURL = "https://oauth2.googleapis.com/tokeninfo"

type AuthHandler struct {
	writeDB      *sql.DB
	settings     *repository.SettingRepository
	jwtSecret    string
	allowedEmail string
	clientID     string
	cookieSecure bool
}

func NewAuthHandler(writeDB *sql.DB, settings *repository.SettingRepository, jwtSecret, allowedEmail, clientID string, cookieSecure bool) *AuthHandler {
	secret := jwtSecret
	if secret == "" && settings != nil && writeDB != nil {
		if s, err := settings.Get(repository.SettingKeyJWTSecret); err == nil && len(s) >= 32 {
			secret = s
		} else {
			// Generate and persist a cryptographic random 32-byte secret
			b := make([]byte, 32)
			if _, err := rand.Read(b); err == nil {
				secret = base64.StdEncoding.EncodeToString(b)
				_ = database.WithTxContext(context.Background(), writeDB, func(tx *sql.Tx) error {
					return repository.NewSettingWriter(tx).Set(context.Background(), repository.SettingKeyJWTSecret, secret)
				})
			}
		}
	}

	return &AuthHandler{
		writeDB:      writeDB,
		settings:     settings,
		jwtSecret:    secret,
		allowedEmail: allowedEmail,
		clientID:     clientID,
		cookieSecure: cookieSecure,
	}
}

func (h *AuthHandler) JWTSecret() string {
	return h.jwtSecret
}

type PublicConfigResponse struct {
	GoogleClientID      string `json:"google_client_id"`
	GoogleAuthEnabled   bool   `json:"google_auth_enabled"`
	PasswordAuthEnabled bool   `json:"password_auth_enabled"`
	AuthMode            string `json:"auth_mode"` // "google", "password", "hybrid"
	SetupRequired       bool   `json:"setup_required"`
	VAPIDPublicKey      string `json:"vapid_public_key"`
	MetadataLanguage    string `json:"metadata_language"`
}

// PublicConfig returns public unauthenticated configuration for login and setup screens.
func (h *AuthHandler) PublicConfig(w http.ResponseWriter, r *http.Request) error {
	hasGoogle := h.clientID != "" && h.allowedEmail != ""
	hasPassword := false
	authMode := "hybrid"

	if h.settings != nil {
		if _, err := h.settings.Get(repository.SettingKeyAdminPasswordHash); err == nil {
			hasPassword = true
		}
		if mode, err := h.settings.Get(repository.SettingKeyAuthMode); err == nil && mode != "" {
			authMode = mode
		} else if hasGoogle && !hasPassword {
			authMode = "google"
		} else if hasPassword && !hasGoogle {
			authMode = "password"
		}
	}

	setupRequired := !hasPassword && !hasGoogle

	var vapidKey string
	metadataLang := "fr"
	if h.settings != nil {
		vapidKey, _ = h.settings.Get(repository.SettingKeyVAPIDPublicKey)
		if lang, err := h.settings.Get(repository.SettingKeyMetadataLanguage); err == nil && lang != "" {
			metadataLang = lang
		}
	}

	cfg := PublicConfigResponse{
		GoogleClientID:      h.clientID,
		GoogleAuthEnabled:   hasGoogle && (authMode == "google" || authMode == "hybrid"),
		PasswordAuthEnabled: hasPassword && (authMode == "password" || authMode == "hybrid"),
		AuthMode:            authMode,
		SetupRequired:       setupRequired,
		VAPIDPublicKey:      vapidKey,
		MetadataLanguage:    metadataLang,
	}

	httputil.WriteJSON(w, http.StatusOK, cfg)
	return nil
}

// Setup handles initial admin account setup.
func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) error {
	if h.settings == nil || h.writeDB == nil {
		return httputil.InternalError("database not configured", errors.New("nil db or settings"))
	}

	hasGoogle := h.clientID != "" && h.allowedEmail != ""
	if _, err := h.settings.Get(repository.SettingKeyAdminPasswordHash); err == nil {
		return httputil.NewAPIError(http.StatusForbidden, "Setup already completed")
	}
	if hasGoogle {
		return httputil.NewAPIError(http.StatusForbidden, "Setup already completed (Google OAuth configured)")
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		AuthMode string `json:"auth_mode"`
	}
	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	username := strings.TrimSpace(body.Username)
	if username == "" {
		username = "admin"
	}
	if len(body.Password) < 8 {
		return httputil.BadRequest("Password must be at least 8 characters long")
	}

	passHash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err != nil {
		return httputil.InternalError("hash password", err)
	}

	recKeyFormatted, recKeyNorm := generateRecoveryKey()
	recKeyHash, err := bcrypt.GenerateFromPassword([]byte(recKeyNorm), 12)
	if err != nil {
		return httputil.InternalError("hash recovery key", err)
	}

	authMode := body.AuthMode
	if authMode != "google" && authMode != "password" && authMode != "hybrid" {
		authMode = "hybrid"
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		w := repository.NewSettingWriter(tx)
		if err := w.Set(r.Context(), repository.SettingKeyAdminUsername, username); err != nil {
			return err
		}
		if err := w.Set(r.Context(), repository.SettingKeyAdminPasswordHash, string(passHash)); err != nil {
			return err
		}
		if err := w.Set(r.Context(), repository.SettingKeyAdminRecoveryKeyHash, string(recKeyHash)); err != nil {
			return err
		}
		return w.Set(r.Context(), repository.SettingKeyAuthMode, authMode)
	}); err != nil {
		return httputil.InternalError("save setup settings", err)
	}

	// Issue JWT
	if err := h.issueAuthCookie(w, username); err != nil {
		return err
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"recovery_key": recKeyFormatted,
	})
	return nil
}

// Login authenticates with local username/password.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) error {
	if h.settings == nil {
		return httputil.InternalError("settings repository not configured", errors.New("nil settings"))
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	mode, _ := h.settings.Get(repository.SettingKeyAuthMode)
	if mode == "google" {
		return httputil.NewAPIError(http.StatusForbidden, "Local password login is disabled")
	}

	expectedUser, err := h.settings.Get(repository.SettingKeyAdminUsername)
	if err != nil || expectedUser == "" {
		expectedUser = "admin"
	}

	passHash, err := h.settings.Get(repository.SettingKeyAdminPasswordHash)
	if err != nil || passHash == "" {
		return httputil.NewAPIError(http.StatusUnauthorized, "Invalid credentials")
	}

	if !strings.EqualFold(strings.TrimSpace(body.Username), expectedUser) {
		_ = bcrypt.CompareHashAndPassword([]byte(passHash), []byte(body.Password))
		return httputil.NewAPIError(http.StatusUnauthorized, "Invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passHash), []byte(body.Password)); err != nil {
		return httputil.NewAPIError(http.StatusUnauthorized, "Invalid credentials")
	}

	if err := h.issueAuthCookie(w, expectedUser); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GoogleCallback verifies Google ID token and issues a JWT cookie.
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) error {
	if h.settings != nil {
		mode, _ := h.settings.Get(repository.SettingKeyAuthMode)
		if mode == "password" {
			return httputil.NewAPIError(http.StatusForbidden, "Google authentication is disabled")
		}
	}

	var body struct {
		Credential string `json:"credential"`
	}
	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

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

	if err := h.issueAuthCookie(w, "admin"); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Recover handles password recovery using the emergency recovery key.
// The recovery key is strictly SINGLE-USE: upon successful recovery, it is
// immediately consumed and atomically replaced by a freshly generated key.
func (h *AuthHandler) Recover(w http.ResponseWriter, r *http.Request) error {
	if h.settings == nil || h.writeDB == nil {
		return httputil.InternalError("database not configured", errors.New("nil db or settings"))
	}

	var body struct {
		RecoveryKey string `json:"recovery_key"`
		NewPassword string `json:"new_password"`
	}
	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	if len(body.NewPassword) < 8 {
		return httputil.BadRequest("New password must be at least 8 characters long")
	}

	recKeyHash, err := h.settings.Get(repository.SettingKeyAdminRecoveryKeyHash)
	if err != nil || recKeyHash == "" {
		return httputil.NewAPIError(http.StatusUnauthorized, "Invalid recovery key")
	}

	normInputKey := normalizeRecoveryKey(body.RecoveryKey)
	if err := bcrypt.CompareHashAndPassword([]byte(recKeyHash), []byte(normInputKey)); err != nil {
		return httputil.NewAPIError(http.StatusUnauthorized, "Invalid recovery key")
	}

	// Update password
	newPassHash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), 12)
	if err != nil {
		return httputil.InternalError("hash new password", err)
	}

	// Strictly single-use: immediately generate and replace with a fresh recovery key
	newRecKeyFormatted, newRecKeyNorm := generateRecoveryKey()
	newRecKeyHash, err := bcrypt.GenerateFromPassword([]byte(newRecKeyNorm), 12)
	if err != nil {
		return httputil.InternalError("hash new recovery key", err)
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		w := repository.NewSettingWriter(tx)
		if err := w.Set(r.Context(), repository.SettingKeyAdminPasswordHash, string(newPassHash)); err != nil {
			return err
		}
		return w.Set(r.Context(), repository.SettingKeyAdminRecoveryKeyHash, string(newRecKeyHash))
	}); err != nil {
		return httputil.InternalError("save recovery settings", err)
	}

	username, err := h.settings.Get(repository.SettingKeyAdminUsername)
	if err != nil || username == "" {
		username = "admin"
	}

	// Issue JWT
	if err := h.issueAuthCookie(w, username); err != nil {
		return err
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"new_recovery_key": newRecKeyFormatted,
	})
	return nil
}

// ChangePassword changes the admin password when authenticated.
// It also automatically regenerates the recovery key.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	if h.settings == nil || h.writeDB == nil {
		return httputil.InternalError("database not configured", errors.New("nil db or settings"))
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	if len(body.NewPassword) < 8 {
		return httputil.BadRequest("New password must be at least 8 characters long")
	}

	passHash, err := h.settings.Get(repository.SettingKeyAdminPasswordHash)
	if err == nil && passHash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(passHash), []byte(body.CurrentPassword)); err != nil {
			return httputil.NewAPIError(http.StatusUnauthorized, "Current password is incorrect")
		}
	}

	newPassHash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), 12)
	if err != nil {
		return httputil.InternalError("hash new password", err)
	}

	// Regenerate recovery key
	newRecKeyFormatted, newRecKeyNorm := generateRecoveryKey()
	newRecKeyHash, err := bcrypt.GenerateFromPassword([]byte(newRecKeyNorm), 12)
	if err != nil {
		return httputil.InternalError("hash new recovery key", err)
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		w := repository.NewSettingWriter(tx)
		if err := w.Set(r.Context(), repository.SettingKeyAdminPasswordHash, string(newPassHash)); err != nil {
			return err
		}
		return w.Set(r.Context(), repository.SettingKeyAdminRecoveryKeyHash, string(newRecKeyHash))
	}); err != nil {
		return httputil.InternalError("save password settings", err)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"new_recovery_key": newRecKeyFormatted,
	})
	return nil
}

// RegenerateRecoveryKey generates a fresh emergency recovery key.
func (h *AuthHandler) RegenerateRecoveryKey(w http.ResponseWriter, r *http.Request) error {
	if h.settings == nil || h.writeDB == nil {
		return httputil.InternalError("database not configured", errors.New("nil db or settings"))
	}

	newRecKeyFormatted, newRecKeyNorm := generateRecoveryKey()
	newRecKeyHash, err := bcrypt.GenerateFromPassword([]byte(newRecKeyNorm), 12)
	if err != nil {
		return httputil.InternalError("hash new recovery key", err)
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		return repository.NewSettingWriter(tx).Set(r.Context(), repository.SettingKeyAdminRecoveryKeyHash, string(newRecKeyHash))
	}); err != nil {
		return httputil.InternalError("save recovery key", err)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"new_recovery_key": newRecKeyFormatted,
	})
	return nil
}

// GetAuthSettings returns admin auth configuration.
func (h *AuthHandler) GetAuthSettings(w http.ResponseWriter, r *http.Request) error {
	hasGoogle := h.clientID != "" && h.allowedEmail != ""
	hasPassword := false
	authMode := "hybrid"
	username := "admin"

	if h.settings != nil {
		if _, err := h.settings.Get(repository.SettingKeyAdminPasswordHash); err == nil {
			hasPassword = true
		}
		if u, err := h.settings.Get(repository.SettingKeyAdminUsername); err == nil && u != "" {
			username = u
		}
		if mode, err := h.settings.Get(repository.SettingKeyAuthMode); err == nil && mode != "" {
			authMode = mode
		} else if hasGoogle && !hasPassword {
			authMode = "google"
		} else if hasPassword && !hasGoogle {
			authMode = "password"
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"auth_mode":    authMode,
		"has_password": hasPassword,
		"has_google":   hasGoogle,
		"google_email": h.allowedEmail,
		"username":     username,
	})
	return nil
}

// UpdateAuthSettings updates the authentication mode.
func (h *AuthHandler) UpdateAuthSettings(w http.ResponseWriter, r *http.Request) error {
	if h.settings == nil || h.writeDB == nil {
		return httputil.InternalError("database not configured", errors.New("nil db or settings"))
	}

	var body struct {
		AuthMode string `json:"auth_mode"`
		Username string `json:"username"`
	}
	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	hasGoogle := h.clientID != "" && h.allowedEmail != ""
	hasPassword := false
	if _, err := h.settings.Get(repository.SettingKeyAdminPasswordHash); err == nil {
		hasPassword = true
	}

	switch body.AuthMode {
	case "google":
		if !hasGoogle {
			return httputil.BadRequest("Google OAuth is not configured")
		}
	case "password":
		if !hasPassword {
			return httputil.BadRequest("No admin password configured yet")
		}
	case "hybrid":
		if !hasGoogle && !hasPassword {
			return httputil.BadRequest("At least one authentication method must be configured")
		}
	default:
		return httputil.BadRequest("Invalid auth mode (expected google, password, or hybrid)")
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		w := repository.NewSettingWriter(tx)
		if err := w.Set(r.Context(), repository.SettingKeyAuthMode, body.AuthMode); err != nil {
			return err
		}
		if u := strings.TrimSpace(body.Username); u != "" {
			return w.Set(r.Context(), repository.SettingKeyAdminUsername, u)
		}
		return nil
	}); err != nil {
		return httputil.InternalError("save auth settings", err)
	}

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

func (h *AuthHandler) issueAuthCookie(w http.ResponseWriter, sub string) error {
	claims := jwt.MapClaims{
		"sub":   sub,
		"email": h.allowedEmail,
		"exp":   jwt.NewNumericDate(time.Now().UTC().Add(30 * 24 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return httputil.InternalError("sign token", err)
	}

	http.SetCookie(w, h.authCookie(signed))
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

func generateRecoveryKey() (string, string) {
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	chars := make([]byte, 12)
	for i := 0; i < 12; i++ {
		chars[i] = alphabet[int(b[i])%len(alphabet)]
	}
	formatted := fmt.Sprintf("TRCK-%s-%s-%s", string(chars[0:4]), string(chars[4:8]), string(chars[8:12]))
	normalized := "TRCK" + string(chars)
	return formatted, normalized
}

func normalizeRecoveryKey(k string) string {
	k = strings.ToUpper(strings.TrimSpace(k))
	k = strings.TrimPrefix(k, "TRCK-")
	k = strings.TrimPrefix(k, "TRCK")
	k = strings.ReplaceAll(k, "-", "")
	k = strings.ReplaceAll(k, " ", "")
	return "TRCK" + k
}
