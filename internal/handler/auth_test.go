package handler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const (
	testJWTSecret    = "test-secret-with-enough-bytes-32"
	testAllowedEmail = "owner@example.com"
	testClientID     = "client-id.apps.googleusercontent.com"
)

func setupAuthTestEnv(t *testing.T) (*sql.DB, *repository.SettingRepository) {
	t.Helper()
	writeDB, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(writeDB))

	t.Cleanup(func() {
		writeDB.Close()
	})

	settingRepo := repository.NewSettingRepository(writeDB)
	return writeDB, settingRepo
}

func setSetting(t *testing.T, db *sql.DB, key, value string) {
	t.Helper()
	err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		return repository.NewSettingWriter(tx).Set(context.Background(), key, value)
	})
	require.NoError(t, err)
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

func assertJWTSignedFor(t *testing.T, raw, secret, expectedSub string) {
	t.Helper()
	tok, err := jwt.Parse(raw, func(_ *jwt.Token) (any, error) { return []byte(secret), nil })
	require.NoError(t, err)
	require.True(t, tok.Valid)
	claims, ok := tok.Claims.(jwt.MapClaims)
	require.True(t, ok)
	assert.Equal(t, expectedSub, claims["sub"])
}

func TestLogout_ClearsCookie(t *testing.T) {
	db, settings := setupAuthTestEnv(t)
	h := handler.NewAuthHandler(db, settings, testJWTSecret, testAllowedEmail, testClientID, false)
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

func TestPublicConfig(t *testing.T) {
	db, settings := setupAuthTestEnv(t)

	// Case 1: Fresh install without Google or Password
	h1 := handler.NewAuthHandler(db, settings, testJWTSecret, "", "", false)
	rr1 := httptest.NewRecorder()
	require.NoError(t, h1.PublicConfig(rr1, httptest.NewRequest("GET", "/api/config", nil)))
	var cfg1 handler.PublicConfigResponse
	require.NoError(t, json.NewDecoder(rr1.Body).Decode(&cfg1))
	assert.True(t, cfg1.SetupRequired)
	assert.False(t, cfg1.GoogleAuthEnabled)
	assert.False(t, cfg1.PasswordAuthEnabled)

	// Case 2: Google configured only
	h2 := handler.NewAuthHandler(db, settings, testJWTSecret, testAllowedEmail, testClientID, false)
	rr2 := httptest.NewRecorder()
	require.NoError(t, h2.PublicConfig(rr2, httptest.NewRequest("GET", "/api/config", nil)))
	var cfg2 handler.PublicConfigResponse
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&cfg2))
	assert.False(t, cfg2.SetupRequired)
	assert.True(t, cfg2.GoogleAuthEnabled)
	assert.False(t, cfg2.PasswordAuthEnabled)
	assert.Equal(t, "google", cfg2.AuthMode)

	// Case 3: Password configured
	passHash, _ := bcrypt.GenerateFromPassword([]byte("mypassword123"), 12)
	setSetting(t, db, "admin_password_hash", string(passHash))
	setSetting(t, db, "auth_mode", "password")

	rr3 := httptest.NewRecorder()
	require.NoError(t, h2.PublicConfig(rr3, httptest.NewRequest("GET", "/api/config", nil)))
	var cfg3 handler.PublicConfigResponse
	require.NoError(t, json.NewDecoder(rr3.Body).Decode(&cfg3))
	assert.False(t, cfg3.SetupRequired)
	assert.False(t, cfg3.GoogleAuthEnabled) // disabled because auth_mode = password
	assert.True(t, cfg3.PasswordAuthEnabled)
	assert.Equal(t, "password", cfg3.AuthMode)
}

func TestSetup_SuccessAndRecoveryKey(t *testing.T) {
	db, settings := setupAuthTestEnv(t)
	h := handler.NewAuthHandler(db, settings, testJWTSecret, "", "", false)

	reqBody := `{"username":"myadmin","password":"strongpassword123","auth_mode":"hybrid"}`
	req := httptest.NewRequest("POST", "/api/auth/setup", strings.NewReader(reqBody))
	rr := httptest.NewRecorder()

	require.NoError(t, h.Setup(rr, req))
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	recoveryKey := resp["recovery_key"]
	assert.True(t, strings.HasPrefix(recoveryKey, "TRCK-"), "expected recovery key format TRCK-XXXX-XXXX-XXXX")

	// Cookie issued
	cookie := extractAuthCookie(t, rr)
	assertJWTSignedFor(t, cookie.Value, testJWTSecret, "myadmin")

	// Settings saved
	u, err := settings.Get("admin_username")
	require.NoError(t, err)
	assert.Equal(t, "myadmin", u)

	mode, err := settings.Get("auth_mode")
	require.NoError(t, err)
	assert.Equal(t, "hybrid", mode)

	// Second setup rejected
	req2 := httptest.NewRequest("POST", "/api/auth/setup", strings.NewReader(reqBody))
	rr2 := httptest.NewRecorder()
	apiErr := requireAPIError(t, h.Setup(rr2, req2))
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
}

func TestLogin_SuccessAndErrors(t *testing.T) {
	db, settings := setupAuthTestEnv(t)
	h := handler.NewAuthHandler(db, settings, testJWTSecret, testAllowedEmail, testClientID, false)

	passHash, _ := bcrypt.GenerateFromPassword([]byte("secret12345"), 12)
	setSetting(t, db, "admin_username", "admin")
	setSetting(t, db, "admin_password_hash", string(passHash))
	setSetting(t, db, "auth_mode", "hybrid")

	// Valid login
	reqValid := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"secret12345"}`))
	rrValid := httptest.NewRecorder()
	require.NoError(t, h.Login(rrValid, reqValid))
	assert.Equal(t, http.StatusNoContent, rrValid.Code)
	cookie := extractAuthCookie(t, rrValid)
	assertJWTSignedFor(t, cookie.Value, testJWTSecret, "admin")

	// Bad password
	reqBadPass := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"wrongpassword"}`))
	rrBadPass := httptest.NewRecorder()
	apiErr := requireAPIError(t, h.Login(rrBadPass, reqBadPass))
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)

	// Mode google only
	setSetting(t, db, "auth_mode", "google")
	reqGoogleOnly := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"secret12345"}`))
	rrGoogleOnly := httptest.NewRecorder()
	apiErr2 := requireAPIError(t, h.Login(rrGoogleOnly, reqGoogleOnly))
	assert.Equal(t, http.StatusForbidden, apiErr2.Status)
}

func TestRecover_StrictlySingleUse(t *testing.T) {
	db, settings := setupAuthTestEnv(t)
	h := handler.NewAuthHandler(db, settings, testJWTSecret, testAllowedEmail, testClientID, false)

	// Setup initial password and recovery key
	const initialRecoveryKey = "TRCK-A1B2-C3D4-E5F6"
	const normalizedKey = "TRCKA1B2C3D4E5F6"
	keyHash, _ := bcrypt.GenerateFromPassword([]byte(normalizedKey), 12)
	passHash, _ := bcrypt.GenerateFromPassword([]byte("oldpassword123"), 12)

	setSetting(t, db, "admin_username", "admin")
	setSetting(t, db, "admin_password_hash", string(passHash))
	setSetting(t, db, "admin_recovery_key_hash", string(keyHash))

	// First recovery using the key must succeed
	recReq := httptest.NewRequest("POST", "/api/auth/recover", strings.NewReader(`{"recovery_key":"trck-a1b2-c3d4-e5f6","new_password":"brandnewpassword123"}`))
	recRR := httptest.NewRecorder()
	require.NoError(t, h.Recover(recRR, recReq))
	assert.Equal(t, http.StatusOK, recRR.Code)

	var recResp map[string]string
	require.NoError(t, json.NewDecoder(recRR.Body).Decode(&recResp))
	newKey := recResp["new_recovery_key"]
	assert.NotEmpty(t, newKey)
	assert.NotEqual(t, initialRecoveryKey, newKey, "Recovery key must be newly regenerated after successful recovery")

	// Single-use invariant: Using the EXACT same key a second time MUST FAIL with 401
	recReq2 := httptest.NewRequest("POST", "/api/auth/recover", strings.NewReader(`{"recovery_key":"`+initialRecoveryKey+`","new_password":"anotherpassword123"}`))
	recRR2 := httptest.NewRecorder()
	apiErr := requireAPIError(t, h.Recover(recRR2, recReq2))
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status, "Used recovery key must be permanently invalidated")

	// Can login with the newly set password
	loginReq := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"brandnewpassword123"}`))
	loginRR := httptest.NewRecorder()
	require.NoError(t, h.Login(loginRR, loginReq))
	assert.Equal(t, http.StatusNoContent, loginRR.Code)
}

func TestChangePassword_AndRegenerateKey(t *testing.T) {
	db, settings := setupAuthTestEnv(t)
	h := handler.NewAuthHandler(db, settings, testJWTSecret, testAllowedEmail, testClientID, false)

	passHash, _ := bcrypt.GenerateFromPassword([]byte("currentpass123"), 12)
	setSetting(t, db, "admin_password_hash", string(passHash))

	// Wrong current password
	badReq := httptest.NewRequest("POST", "/api/auth/change-password", strings.NewReader(`{"current_password":"wrong","new_password":"newstrongpass123"}`))
	badRR := httptest.NewRecorder()
	apiErr := requireAPIError(t, h.ChangePassword(badRR, badReq))
	assert.Equal(t, http.StatusUnauthorized, apiErr.Status)

	// Valid change
	goodReq := httptest.NewRequest("POST", "/api/auth/change-password", strings.NewReader(`{"current_password":"currentpass123","new_password":"newstrongpass123"}`))
	goodRR := httptest.NewRecorder()
	require.NoError(t, h.ChangePassword(goodRR, goodReq))
	assert.Equal(t, http.StatusOK, goodRR.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(goodRR.Body).Decode(&resp))
	assert.True(t, strings.HasPrefix(resp["new_recovery_key"], "TRCK-"))
}

func TestAuthSettings_GetAndUpdate(t *testing.T) {
	db, settings := setupAuthTestEnv(t)
	h := handler.NewAuthHandler(db, settings, testJWTSecret, testAllowedEmail, testClientID, false)

	passHash, _ := bcrypt.GenerateFromPassword([]byte("mypassword123"), 12)
	setSetting(t, db, "admin_password_hash", string(passHash))
	setSetting(t, db, "admin_username", "admin")
	setSetting(t, db, "auth_mode", "hybrid")

	// Get settings
	getReq := httptest.NewRequest("GET", "/api/admin/auth-settings", nil)
	getRR := httptest.NewRecorder()
	require.NoError(t, h.GetAuthSettings(getRR, getReq))
	assert.Equal(t, http.StatusOK, getRR.Code)

	var getResp map[string]any
	require.NoError(t, json.NewDecoder(getRR.Body).Decode(&getResp))
	assert.Equal(t, "hybrid", getResp["auth_mode"])
	assert.Equal(t, true, getResp["has_password"])
	assert.Equal(t, true, getResp["has_google"])

	// Update settings
	putReq := httptest.NewRequest("PUT", "/api/admin/auth-settings", strings.NewReader(`{"auth_mode":"password","username":"admin_user"}`))
	putRR := httptest.NewRecorder()
	require.NoError(t, h.UpdateAuthSettings(putRR, putReq))
	assert.Equal(t, http.StatusNoContent, putRR.Code)

	mode, _ := settings.Get("auth_mode")
	assert.Equal(t, "password", mode)
	user, _ := settings.Get("admin_username")
	assert.Equal(t, "admin_user", user)
}
