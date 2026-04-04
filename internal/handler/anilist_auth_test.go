package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAniListHandler(t *testing.T) (*handler.AniListAuthHandler, *repository.SettingRepository) {
	t.Helper()
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	h := handler.NewAniListAuthHandler(settings, "test-client-id")
	return h, settings
}

func TestAniListAuth_Authorize(t *testing.T) {
	h, _ := setupAniListHandler(t)

	req := httptest.NewRequest("GET", "/api/anilist/auth", nil)
	rr := httptest.NewRecorder()
	h.Authorize(rr, req)

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "anilist.co/api/v2/oauth/authorize")
	assert.Contains(t, rr.Header().Get("Location"), "client_id=test-client-id")
}

func TestAniListAuth_SaveToken(t *testing.T) {
	h, settings := setupAniListHandler(t)

	req := httptest.NewRequest("POST", "/api/anilist/token", strings.NewReader(`{"token":"abc123"}`))
	rr := httptest.NewRecorder()
	h.SaveToken(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)

	val, err := settings.Get("anilist_token")
	require.NoError(t, err)
	assert.Equal(t, "abc123", val)
}

func TestAniListAuth_SaveToken_EmptyRejected(t *testing.T) {
	h, _ := setupAniListHandler(t)

	req := httptest.NewRequest("POST", "/api/anilist/token", strings.NewReader(`{"token":""}`))
	rr := httptest.NewRecorder()
	h.SaveToken(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAniListAuth_Disconnect(t *testing.T) {
	h, settings := setupAniListHandler(t)
	_ = settings.Set("anilist_token", "abc123")

	req := httptest.NewRequest("DELETE", "/api/anilist/token", nil)
	rr := httptest.NewRecorder()
	h.Disconnect(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)

	_, err := settings.Get("anilist_token")
	assert.Error(t, err)
}

func TestAniListAuth_AuthorizeNotConfigured(t *testing.T) {
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	h := handler.NewAniListAuthHandler(settings, "") // No client ID

	req := httptest.NewRequest("GET", "/api/anilist/auth", nil)
	rr := httptest.NewRecorder()
	h.Authorize(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
