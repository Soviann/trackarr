package config_test

import (
	"strings"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validSecret = "a-secret-with-at-least-32-characters"

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("DATA_DIR", "")
	t.Setenv("DISABLE_BACKGROUND_TASKS", "false")
	t.Setenv("DEBUG_LOGIN", "false")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "test-client-id", cfg.GoogleClientID)
	assert.Equal(t, "test@example.com", cfg.GoogleAllowedEmail)
	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Equal(t, "/data", cfg.DataDir)
	assert.False(t, cfg.DisableBackgroundTasks)
}

func TestLoad_DisableBackgroundTasks(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("DISABLE_BACKGROUND_TASKS", "true")
	t.Setenv("DEBUG_LOGIN", "false")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.True(t, cfg.DisableBackgroundTasks)
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("DEBUG_LOGIN", "false")

	_, err := config.Load()
	assert.Error(t, err)
}

// Even with DEBUG_LOGIN=true, the default sentinel JWT secret must be rejected —
// otherwise a deploy that ships DEBUG_LOGIN=true (env mis-copied) silently
// accepts the dev placeholder and lets anyone forge tokens.
func TestLoad_DefaultJWTSecretRejectedRegardlessOfDebugLogin(t *testing.T) {
	for _, debug := range []string{"true", "false"} {
		t.Run("DEBUG_LOGIN="+debug, func(t *testing.T) {
			t.Setenv("GOOGLE_CLIENT_ID", "dev")
			t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
			t.Setenv("JWT_SECRET", "dev-secret-change-me")
			t.Setenv("DEBUG_LOGIN", debug)

			_, err := config.Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "JWT_SECRET")
		})
	}
}

func TestLoad_JWTSecretTooShortRejected(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("DEBUG_LOGIN", "false")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "32 characters")
}

func TestLoad_JWTSecretMinLengthAccepted(t *testing.T) {
	// Exactly 32 characters — boundary case.
	secret := strings.Repeat("x", 32)
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", secret)
	t.Setenv("DEBUG_LOGIN", "false")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, secret, cfg.JWTSecret)
}

// DEBUG_LOGIN=true with COOKIE_SECURE=true means a prod deploy accidentally
// inherited the dev .env — refuse to boot rather than expose /api/auth/dev.
func TestLoad_DebugLoginRejectedWhenCookieSecure(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "dev")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("DEBUG_LOGIN", "true")
	t.Setenv("COOKIE_SECURE", "true")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DEBUG_LOGIN")
	assert.Contains(t, err.Error(), "COOKIE_SECURE")
}

// DEBUG_LOGIN=true with COOKIE_SECURE unset is the standard dev path —
// CookieSecure defaults to !DebugLogin = false, so the guard must not trip when GOOGLE_CLIENT_ID=dev.
func TestLoad_DebugLoginAllowedInDev(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "dev")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("DEBUG_LOGIN", "true")
	t.Setenv("COOKIE_SECURE", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.DebugLogin)
	assert.False(t, cfg.CookieSecure)
}

func TestLoad_DebugLoginDisabledInProductionOAuth(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "real-prod-oauth-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("DEBUG_LOGIN", "true")
	t.Setenv("COOKIE_SECURE", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.False(t, cfg.DebugLogin)
	assert.True(t, cfg.CookieSecure)
}
