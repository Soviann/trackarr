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

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.True(t, cfg.DisableBackgroundTasks)
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "")
	t.Setenv("JWT_SECRET", "")

	_, err := config.Load()
	assert.Error(t, err)
}

// Even with DEBUG_LOGIN=true, the default sentinel JWT secret must be rejected —
// otherwise a deploy that ships DEBUG_LOGIN=true (env mis-copied) silently
// accepts the dev placeholder and lets anyone forge tokens.
func TestLoad_DefaultJWTSecretRejectedRegardlessOfDebugLogin(t *testing.T) {
	for _, debug := range []string{"true", "false"} {
		t.Run("DEBUG_LOGIN="+debug, func(t *testing.T) {
			t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
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

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, secret, cfg.JWTSecret)
}
