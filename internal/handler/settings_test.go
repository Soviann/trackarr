package handler_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
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
	h := handler.NewSettingsHandler(settings, false)
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
}
