package config_test

import (
	"strings"
	"testing"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validSecret = "a-secret-with-at-least-32-characters"

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("JELLYFIN_WEBHOOK_SECRET", "test-jellyfin-secret")
	t.Setenv("DATA_DIR", "")
	t.Setenv("DISABLE_BACKGROUND_TASKS", "false")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "test-client-id", cfg.GoogleClientID)
	assert.Equal(t, "test@example.com", cfg.GoogleAllowedEmail)
	assert.Equal(t, "test-jellyfin-secret", cfg.JellyfinWebhookSecret)
	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Equal(t, "/data", cfg.DataDir)
	assert.False(t, cfg.DisableBackgroundTasks)
}

func TestLoad_DisableBackgroundTasks(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("JELLYFIN_WEBHOOK_SECRET", "test-jellyfin-secret")
	t.Setenv("DISABLE_BACKGROUND_TASKS", "true")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.True(t, cfg.DisableBackgroundTasks)
}

func TestLoad_MissingRequired(t *testing.T) {
	// If one of Google Client ID or Google Allowed Email is set without the other, it must error
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "")
	t.Setenv("JWT_SECRET", "")

	_, err := config.Load()
	assert.Error(t, err)
}

func TestLoad_DefaultJWTSecretRejected(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", "dev-secret-change-me")
	t.Setenv("JELLYFIN_WEBHOOK_SECRET", "test-jellyfin-secret")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_JWTSecretTooShortRejected(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("JELLYFIN_WEBHOOK_SECRET", "test-jellyfin-secret")

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
	t.Setenv("JELLYFIN_WEBHOOK_SECRET", "test-jellyfin-secret")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, secret, cfg.JWTSecret)
}

// JELLYFIN_WEBHOOK_SECRET is mandatory in production (COOKIE_SECURE=true).
func TestLoad_JellyfinWebhookSecretRequiredInProd(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("JELLYFIN_WEBHOOK_SECRET", "")
	t.Setenv("WEBHOOK_SECRET", "")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook secret")
}

// In local dev (COOKIE_SECURE=false), JELLYFIN_WEBHOOK_SECRET is optional.
func TestLoad_JellyfinWebhookSecretOptionalInDev(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "dev")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("JELLYFIN_WEBHOOK_SECRET", "")
	t.Setenv("WEBHOOK_SECRET", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.JellyfinWebhookSecret)
}
