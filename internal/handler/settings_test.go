package handler_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSettingsHandler(t *testing.T) (*handler.SettingsHandler, *sql.DB) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	events := repository.NewWatchEventRepository(db)
	h := handler.NewSettingsHandler(settings, events, false, true)
	return h, db
}

func TestSettingsHandler_Get_Empty(t *testing.T) {
	h, _ := setupSettingsHandler(t)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)

	var result map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&result)
	assert.Equal(t, false, result["anilist_connected"])
	assert.Equal(t, false, result["push_subscribed"])
}

func TestSettingsHandler_Get_AniListConnected(t *testing.T) {
	h, db := setupSettingsHandler(t)
	testutil.SetSetting(t, db, "anilist_token", "some-token")

	req := httptest.NewRequest("GET", "/api/settings", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	var result map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&result)
	assert.Equal(t, true, result["anilist_connected"])
	assert.Equal(t, false, result["push_subscribed"])
}

func TestSettingsHandler_Get_PushSubscribed(t *testing.T) {
	h, db := setupSettingsHandler(t)
	testutil.SetSetting(t, db, "push_subscription", `{"endpoint":"https://push.example.com"}`)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	var result map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&result)
	assert.Equal(t, false, result["anilist_connected"])
	assert.Equal(t, true, result["push_subscribed"])
}

func TestSettingsHandler_Get_ReportsTokenInvalid(t *testing.T) {
	h, db := setupSettingsHandler(t)
	testutil.SetSetting(t, db, "anilist_token", "abc")
	testutil.SetSetting(t, db, "anilist_token_invalid", "true")

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	var result map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.Equal(t, true, result["anilist_connected"])
	assert.Equal(t, true, result["anilist_token_invalid"])
}

func TestSettingsHandler_Get_TokenValidFlagAbsent(t *testing.T) {
	h, db := setupSettingsHandler(t)
	testutil.SetSetting(t, db, "anilist_token", "abc")

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	var result map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.Equal(t, false, result["anilist_token_invalid"])
	assert.Equal(t, true, result["jellyfin_configured"])
}

func TestSettingsHandler_Get_JellyfinLastScrobble(t *testing.T) {
	h, db := setupSettingsHandler(t)

	// Create a title and a jellyfin watch event
	_, err := db.Exec(`INSERT INTO titles (id, type, year, status, match_status) VALUES (1, 'movie', 2024, 'completed', 'confirmed')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO watch_events (title_id, source, created_at) VALUES (1, 'jellyfin', '2026-08-18 14:00:00')`)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	var result map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.Equal(t, true, result["jellyfin_configured"])
	assert.NotNil(t, result["jellyfin_last_scrobble_at"])
}

type mockProwlarrChecker struct {
	configured bool
}

func (m mockProwlarrChecker) IsConfigured() bool {
	return m.configured
}

func TestSettingsHandler_Get_ProwlarrConfigured(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	events := repository.NewWatchEventRepository(db)
	h := handler.NewSettingsHandler(settings, events, false, false, mockProwlarrChecker{configured: true})

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	var result map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.Equal(t, true, result["prowlarr_configured"])
}
