package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/cache"
	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
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
	return handler.NewStatsHandler(db, statsRepo, wrappedRepo, nil)
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

func TestStatsHandler_Get_CacheAndInvalidation(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	statsRepo := repository.NewStatsRepository(db)
	wrappedRepo := repository.NewWrappedRepository(db)
	h := handler.NewStatsHandler(db, statsRepo, wrappedRepo, nil)

	c := cache.New(10, 0)
	database.SetInvalidator(c)
	defer database.SetInvalidator(nil)
	h.SetCache(c)

	// 1. Initial request: empty library stats, populates cache
	req1 := httptest.NewRequest(http.MethodGet, "/api/stats?timeframe=all", nil)
	rr1 := httptest.NewRecorder()
	require.NoError(t, h.Get(rr1, req1))
	assert.Equal(t, http.StatusOK, rr1.Code)

	var res1 model.StatsResponse
	require.NoError(t, json.NewDecoder(rr1.Body).Decode(&res1))
	assert.Equal(t, 0, res1.Overview.TotalTitles)

	// Verify cache key exists
	cached, ok := c.Get("stats:all:0:all")
	assert.True(t, ok)
	assert.NotNil(t, cached)

	// 2. Second request hits cache
	rr2 := httptest.NewRecorder()
	require.NoError(t, h.Get(rr2, req1))
	assert.Equal(t, http.StatusOK, rr2.Code)

	// 3. Mutate: add a watched title in WithTxContext
	testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Stats Invalidation Movie", Language: "en", IsPrimary: true}},
	)

	// Verify cache key was purged post-commit
	_, okAfter := c.Get("stats:all:0:all")
	assert.False(t, okAfter, "cache must be invalidated when titles/stats change")

	// 4. Third request: refreshed from DB, now TotalTitles == 1
	rr3 := httptest.NewRecorder()
	require.NoError(t, h.Get(rr3, req1))
	assert.Equal(t, http.StatusOK, rr3.Code)

	var res3 model.StatsResponse
	require.NoError(t, json.NewDecoder(rr3.Body).Decode(&res3))
	assert.Equal(t, 1, res3.Overview.TotalTitles)
}

