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

func setupStatsHandler(t *testing.T) *handler.StatsHandler {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })
	statsRepo := repository.NewStatsRepository(db)
	return handler.NewStatsHandler(statsRepo)
}

func TestStatsHandler_Get_EmptyDB(t *testing.T) {
	h := setupStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
}

func TestStatsHandler_Get_CacheControl(t *testing.T) {
	h := setupStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	assert.Contains(t, rr.Header().Get("Cache-Control"), "private, max-age=300")
}
