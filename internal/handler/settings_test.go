package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSettingsHandler(t *testing.T) (*handler.SettingsHandler, *repository.SettingRepository) {
	t.Helper()
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	h := handler.NewSettingsHandler(settings)
	return h, settings
}

func TestSettingsHandler_Get_Empty(t *testing.T) {
	h, _ := setupSettingsHandler(t)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)
	assert.Equal(t, false, result["anilist_connected"])
	assert.Equal(t, false, result["push_subscribed"])
}

func TestSettingsHandler_Get_AniListConnected(t *testing.T) {
	h, settings := setupSettingsHandler(t)
	settings.Set("anilist_token", "some-token")

	req := httptest.NewRequest("GET", "/api/settings", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)
	assert.Equal(t, true, result["anilist_connected"])
	assert.Equal(t, false, result["push_subscribed"])
}

func TestSettingsHandler_Get_PushSubscribed(t *testing.T) {
	h, settings := setupSettingsHandler(t)
	settings.Set("push_subscription", `{"endpoint":"https://push.example.com"}`)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)
	assert.Equal(t, false, result["anilist_connected"])
	assert.Equal(t, true, result["push_subscribed"])
}
