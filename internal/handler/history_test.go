package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryHandler_Get(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`INSERT INTO titles (id, type, year, status, match_status) VALUES (1, 'series', 2026, 'watching', 'confirmed')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO seasons (id, title_id, season_number, total_episodes) VALUES (1, 1, 1, 12)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO episodes (id, season_id, episode, name, watched) VALUES (10, 1, 1, 'Pilot', 1), (11, 1, 2, 'Second', 1)`)
	require.NoError(t, err)

	// Two watch events for episode 10 (rewatch), one for episode 11
	_, err = db.Exec(`INSERT INTO watch_events (title_id, episode_id, source, created_at) VALUES (1, 10, 'manual', '2026-04-01 21:00:00')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO watch_events (title_id, episode_id, source, created_at) VALUES (1, 10, 'manual', '2026-04-10 22:00:00')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO watch_events (title_id, episode_id, source, created_at) VALUES (1, 11, 'manual', '2026-04-11 20:00:00')`)
	require.NoError(t, err)

	h := handler.NewHistoryHandler(repository.NewHistoryRepository(db))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/titles/1/history", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	err = h.Get(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	// Episode 10 watched twice, episode 11 once → 2 distinct episode groups
	assert.Len(t, result, 2)
	// Most recent first: episode 11 (2026-04-11) before episode 10 (2026-04-10)
	assert.Equal(t, float64(11), result[0]["episode_id"])
}

func TestHistoryHandler_Get_InvalidID(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	h := handler.NewHistoryHandler(repository.NewHistoryRepository(db))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/titles/abc/history", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	err = h.Get(w, r)
	assert.Error(t, err)
}
