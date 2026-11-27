package handler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	for _, key := range []string{"top_actors", "top_directors", "top_writers", "top_composers"} {
		assert.Contains(t, result, key)
		assert.Equal(t, []any{}, result[key], "%s must serialize as [] rather than null", key)
	}
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

	// 3. Mutate: add a watched title with credits in WithTxContext
	credits := `[{"name":"Denis Villeneuve","role":"Director"},{"name":"Jon Spaihts","role":"Writer"},{"name":"Hans Zimmer","role":"Composer"},{"name":"Timothée Chalamet","role":"Paul"}]`
	titleID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, Credits: &credits},
		[]model.TitleName{{Name: "Stats Invalidation Movie", Language: "en", IsPrimary: true}},
	)

	// Verify cache key was purged post-commit
	_, okAfter := c.Get("stats:all:0:all")
	assert.False(t, okAfter, "cache must be invalidated when titles/stats change")

	// 4. Third request: refreshed from DB, now TotalTitles == 1 and crew rankings populated
	rr3 := httptest.NewRecorder()
	require.NoError(t, h.Get(rr3, req1))
	assert.Equal(t, http.StatusOK, rr3.Code)

	var res3 model.StatsResponse
	require.NoError(t, json.NewDecoder(rr3.Body).Decode(&res3))
	assert.Equal(t, 1, res3.Overview.TotalTitles)
	assert.Equal(t, []model.PersonStat{{Name: "Jon Spaihts", Count: 1}}, res3.TopWriters)
	assert.Equal(t, []model.PersonStat{{Name: "Hans Zimmer", Count: 1}}, res3.TopComposers)

	// 5. Mutate via UpdateCreditsCatchUp inside WithTxContext -> cache invalidated again
	_, cachedBeforeCatchUp := c.Get("stats:all:0:all")
	assert.True(t, cachedBeforeCatchUp)

	updatedCredits := `[{"name":"Denis Villeneuve","role":"Director"},{"name":"Eric Roth","role":"Writer"},{"name":"Hans Zimmer","role":"Composer"},{"name":"Timothée Chalamet","role":"Paul"}]`
	require.NoError(t, database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).UpdateCreditsCatchUp(context.Background(), titleID, updatedCredits, time.Now().UTC())
	}))

	_, okAfterCatchUp := c.Get("stats:all:0:all")
	assert.False(t, okAfterCatchUp, "cache must be invalidated after UpdateCreditsCatchUp commits")

	rr4 := httptest.NewRecorder()
	require.NoError(t, h.Get(rr4, req1))
	assert.Equal(t, http.StatusOK, rr4.Code)
	var res4 model.StatsResponse
	require.NoError(t, json.NewDecoder(rr4.Body).Decode(&res4))
	assert.Equal(t, []model.PersonStat{{Name: "Eric Roth", Count: 1}}, res4.TopWriters)
}

func TestStatsHandler_Get_ExtendedCreditsAndFilters(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	statsRepo := repository.NewStatsRepository(db)
	wrappedRepo := repository.NewWrappedRepository(db)
	h := handler.NewStatsHandler(db, statsRepo, wrappedRepo, nil)

	movieCredits := `[{"name":"Denis Villeneuve","role":"Director"},{"name":"Eric Roth","role":"Writer"},{"name":"Hans Zimmer","role":"Composer"},{"name":"Rebecca Ferguson","role":"Lady Jessica"}]`
	movieID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2021, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, Credits: &movieCredits},
		[]model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}},
	)
	animeCredits := `[{"name":"Shinichiro Watanabe","role":"Director"},{"name":"Keiko Nobumoto","role":"Writer"},{"name":"Yoko Kanno","role":"Composer"},{"name":"Koichi Yamadera","role":"Spike"}]`
	animeID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, IsAnime: true, Year: 1998, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, Credits: &animeCredits},
		[]model.TitleName{{Name: "Cowboy Bebop", Language: "en", IsPrimary: true}},
	)

	testutil.CreateWatchEvent(t, db, &model.WatchEvent{
		TitleID:   movieID,
		Source:    model.WatchEventSourcePlex,
		CreatedAt: time.Date(2024, 3, 1, 20, 0, 0, 0, time.UTC),
	})
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{
		TitleID:   animeID,
		Source:    model.WatchEventSourcePlex,
		CreatedAt: time.Now().UTC().AddDate(0, 0, -3),
	})

	t.Run("all titles", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
		rr := httptest.NewRecorder()
		require.NoError(t, h.Get(rr, req))
		assert.Equal(t, http.StatusOK, rr.Code)

		var resp model.StatsResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, []model.PersonStat{{Name: "Eric Roth", Count: 1}, {Name: "Keiko Nobumoto", Count: 1}}, resp.TopWriters)
		assert.Equal(t, []model.PersonStat{{Name: "Hans Zimmer", Count: 1}, {Name: "Yoko Kanno", Count: 1}}, resp.TopComposers)
		assert.Equal(t, []model.PersonStat{{Name: "Koichi Yamadera", Count: 1}, {Name: "Rebecca Ferguson", Count: 1}}, resp.TopActors)
	})

	t.Run("media_type=movie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/stats?media_type=movie", nil)
		rr := httptest.NewRecorder()
		require.NoError(t, h.Get(rr, req))

		var resp model.StatsResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, []model.PersonStat{{Name: "Eric Roth", Count: 1}}, resp.TopWriters)
		assert.Equal(t, []model.PersonStat{{Name: "Hans Zimmer", Count: 1}}, resp.TopComposers)
		assert.Equal(t, []model.PersonStat{{Name: "Denis Villeneuve", Count: 1}}, resp.TopDirectors)
		assert.Equal(t, []model.PersonStat{{Name: "Rebecca Ferguson", Count: 1}}, resp.TopActors)
	})

	t.Run("timeframe=year and year=2024", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/stats?timeframe=year&year=2024", nil)
		rr := httptest.NewRecorder()
		require.NoError(t, h.Get(rr, req))

		var resp model.StatsResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, []model.PersonStat{{Name: "Eric Roth", Count: 1}}, resp.TopWriters)
		assert.Equal(t, []model.PersonStat{{Name: "Hans Zimmer", Count: 1}}, resp.TopComposers)
	})

	t.Run("timeframe=30d and media_type=anime", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/stats?timeframe=30d&media_type=anime", nil)
		rr := httptest.NewRecorder()
		require.NoError(t, h.Get(rr, req))

		var resp model.StatsResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, []model.PersonStat{{Name: "Keiko Nobumoto", Count: 1}}, resp.TopWriters)
		assert.Equal(t, []model.PersonStat{{Name: "Yoko Kanno", Count: 1}}, resp.TopComposers)
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		req := httptest.NewRequest(http.MethodGet, "/api/stats", nil).WithContext(ctx)
		rr := httptest.NewRecorder()
		err := h.Get(rr, req)
		assert.Error(t, err)
	})
}
