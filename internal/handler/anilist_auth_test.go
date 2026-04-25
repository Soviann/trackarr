package handler_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAniListHandler(t *testing.T) (*handler.AniListAuthHandler, *sql.DB, *repository.SettingRepository) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	h := handler.NewAniListAuthHandler(db, settings, "test-client-id")
	return h, db, settings
}

func TestAniListAuth_Authorize(t *testing.T) {
	h, _, _ := setupAniListHandler(t)

	req := httptest.NewRequest("GET", "/api/anilist/auth", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Authorize(rr, req))

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "anilist.co/api/v2/oauth/authorize")
	assert.Contains(t, rr.Header().Get("Location"), "client_id=test-client-id")
}

func TestAniListAuth_SaveToken(t *testing.T) {
	h, _, settings := setupAniListHandler(t)

	req := httptest.NewRequest("POST", "/api/anilist/token", strings.NewReader(`{"token":"abc123"}`))
	rr := httptest.NewRecorder()
	require.NoError(t, h.SaveToken(rr, req))

	assert.Equal(t, http.StatusNoContent, rr.Code)

	val, err := settings.Get("anilist_token")
	require.NoError(t, err)
	assert.Equal(t, "abc123", val)
}

func TestAniListAuth_SaveToken_ClearsInvalidFlag(t *testing.T) {
	h, db, settings := setupAniListHandler(t)
	testutil.SetSetting(t, db, "anilist_token_invalid", "true")

	req := httptest.NewRequest("POST", "/api/anilist/token", strings.NewReader(`{"token":"xyz"}`))
	rr := httptest.NewRecorder()
	require.NoError(t, h.SaveToken(rr, req))

	got, _ := settings.Get("anilist_token_invalid")
	assert.NotEqual(t, "true", got)
}

func TestAniListAuth_SaveToken_EmptyRejected(t *testing.T) {
	h, _, _ := setupAniListHandler(t)

	req := httptest.NewRequest("POST", "/api/anilist/token", strings.NewReader(`{"token":""}`))
	rr := httptest.NewRecorder()
	err := h.SaveToken(rr, req)

	require.Error(t, err)
	apiErr, ok := err.(*httputil.APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestAniListAuth_Disconnect(t *testing.T) {
	h, db, settings := setupAniListHandler(t)
	testutil.SetSetting(t, db, "anilist_token", "abc123")

	req := httptest.NewRequest("DELETE", "/api/anilist/token", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Disconnect(rr, req))

	assert.Equal(t, http.StatusNoContent, rr.Code)

	_, err := settings.Get("anilist_token")
	assert.Error(t, err)
}

func TestAniListAuth_AuthorizeNotConfigured(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	h := handler.NewAniListAuthHandler(db, settings, "") // No client ID

	req := httptest.NewRequest("GET", "/api/anilist/auth", nil)
	rr := httptest.NewRecorder()
	err = h.Authorize(rr, req)

	require.Error(t, err)
	apiErr, ok := err.(*httputil.APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, apiErr.Status)
}
