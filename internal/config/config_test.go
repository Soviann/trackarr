package config_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("DATA_DIR", "")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "test-client-id", cfg.GoogleClientID)
	assert.Equal(t, "test@example.com", cfg.GoogleAllowedEmail)
	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Equal(t, "/data", cfg.DataDir)
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "")
	t.Setenv("JWT_SECRET", "")

	_, err := config.Load()
	assert.Error(t, err)
}
