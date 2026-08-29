package handler_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/service/matching"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdminSettingsTestEnv(t *testing.T) (*sql.DB, *repository.SettingRepository, *service.DynamicConfigReloader, *handler.AdminSettingsHandler) {
	t.Helper()
	writeDB, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(writeDB))

	t.Cleanup(func() {
		writeDB.Close()
	})

	settingRepo := repository.NewSettingRepository(writeDB)
	cfg := &config.Config{
		TMDBAPIKey:            "env-tmdb-key-12345",
		JellyfinWebhookSecret: "env-jellyfin-secret",
		PlexWebhookSecret:     "env-plex-secret",
	}

	pipeline := matching.NewPipeline(nil, nil, nil, nil, "")
	reloader := service.NewDynamicConfigReloader(cfg, writeDB, settingRepo, pipeline, nil, nil, nil, nil, nil)
	h := handler.NewAdminSettingsHandler(writeDB, settingRepo, reloader)

	return writeDB, settingRepo, reloader, h
}

func TestGetSystemSettings(t *testing.T) {
	_, _, _, h := setupAdminSettingsTestEnv(t)

	req := httptest.NewRequest("GET", "/api/admin/system-settings", nil)
	req.Host = "tracker.example.com"
	rr := httptest.NewRecorder()

	require.NoError(t, h.GetSystemSettings(rr, req))
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp handler.SystemSettingsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	// TMDB should fall back to env key (masked)
	assert.True(t, resp.TMDBConfigured)
	assert.True(t, strings.HasPrefix(resp.TMDBAPIKey, "••••••••"))

	// Webhook URLs
	assert.Equal(t, "env-jellyfin-secret", resp.JellyfinWebhookSecret)
	assert.Equal(t, "http://tracker.example.com/api/webhook/jellyfin/env-jellyfin-secret", resp.JellyfinWebhookURL)
	assert.Equal(t, "env-plex-secret", resp.PlexWebhookSecret)
	assert.Equal(t, "http://tracker.example.com/api/webhook/plex/env-plex-secret", resp.PlexWebhookURL)

	// Default metadata language is fr
	assert.Equal(t, "fr", resp.MetadataLanguage)
}

func TestUpdateSystemSettings_AndReload(t *testing.T) {
	db, settings, reloader, h := setupAdminSettingsTestEnv(t)

	updateBody := `{
		"tmdb_api_key": "new-sqlite-tmdb-key",
		"tvdb_api_key": "new-sqlite-tvdb-key",
		"radarr_url": "http://192.168.1.50:7878",
		"radarr_api_key": "radarr-key-999",
		"metadata_language": "en"
	}`

	req := httptest.NewRequest("PUT", "/api/admin/system-settings", strings.NewReader(updateBody))
	rr := httptest.NewRecorder()

	require.NoError(t, h.UpdateSystemSettings(rr, req))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Verify SQLite persistence
	tmdbKey, err := settings.Get("tmdb_api_key")
	require.NoError(t, err)
	assert.Equal(t, "new-sqlite-tmdb-key", tmdbKey)

	metaLang, err := settings.Get("metadata_language")
	require.NoError(t, err)
	assert.Equal(t, "en", metaLang)

	radarrURL, err := settings.Get("radarr_url")
	require.NoError(t, err)
	assert.Equal(t, "http://192.168.1.50:7878", radarrURL)

	// Verify dynamic reloader returns the new value
	assert.Equal(t, "new-sqlite-tmdb-key", reloader.Get("tmdb_api_key"))
	assert.Equal(t, "en", reloader.Get("metadata_language"))
	assert.Equal(t, "http://192.168.1.50:7878", reloader.Get("radarr_url"))

	_ = db
}

func TestTestArr_Validation(t *testing.T) {
	_, _, _, h := setupAdminSettingsTestEnv(t)

	r := chi.NewRouter()
	r.Post("/api/admin/system-settings/test/{app}", func(w http.ResponseWriter, req *http.Request) {
		_ = h.TestArr(w, req)
	})

	// Missing URL/Key
	req := httptest.NewRequest("POST", "/api/admin/system-settings/test/radarr", strings.NewReader(`{"url":"","api_key":""}`))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, false, resp["ok"])
	assert.Contains(t, resp["error"], "URL et clé d'API requises")
}

func TestGenerateVAPIDKeys(t *testing.T) {
	_, settings, _, h := setupAdminSettingsTestEnv(t)

	req := httptest.NewRequest("POST", "/api/admin/system-settings/vapid/generate", strings.NewReader(`{"subject":"mailto:admin@domain.com"}`))
	rr := httptest.NewRecorder()

	require.NoError(t, h.GenerateVAPIDKeys(rr, req))
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, true, resp["ok"])
	assert.NotEmpty(t, resp["vapid_public_key"])
	assert.Equal(t, "mailto:admin@domain.com", resp["vapid_subject"])

	// Check DB persistence
	pubKey, err := settings.Get("vapid_public_key")
	require.NoError(t, err)
	assert.NotEmpty(t, pubKey)

	privKey, err := settings.Get("vapid_private_key")
	require.NoError(t, err)
	assert.NotEmpty(t, privKey)
}
