package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
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
	wrappedRepo := repository.NewWrappedRepository(db)
	return handler.NewStatsHandler(statsRepo, wrappedRepo, nil)
}

func TestStatsHandler_Get_EmptyDB(t *testing.T) {
	h := setupStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.Contains(t, result, "top_actors")
	assert.Contains(t, result, "top_directors")
}

func TestStatsHandler_Get_CacheControl(t *testing.T) {
	h := setupStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	assert.Contains(t, rr.Header().Get("Cache-Control"), "private, max-age=300")
}

func TestStatsHandler_Get_WithQueryParams(t *testing.T) {
	h := setupStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats?timeframe=year&year=2024&media_type=movie", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Get(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)

	var result model.StatsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.NotEmpty(t, result.AvailableYears)
}

func TestStatsHandler_GetWrapped_EmptyDB(t *testing.T) {
	h := setupStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats/wrapped?year=2026", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.GetWrapped(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp model.WrappedResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, 2026, resp.Year)
	assert.NotEmpty(t, resp.Persona.Title)
}

func TestStatsHandler_GetWrappedArchives(t *testing.T) {
	h := setupStatsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats/wrapped/archives", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.GetWrappedArchives(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)

	var archives []model.WrappedArchiveItem
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&archives))
	assert.Empty(t, archives)
}

func TestStatsHandler_RegenerateWrapped(t *testing.T) {
	h := setupStatsHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/stats/wrapped/generate?year=2025", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.RegenerateWrapped(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp model.WrappedResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, 2025, resp.Year)
}
