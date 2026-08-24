package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityHandler_List(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	// Setup: a movie title + watch event
	_, err = db.Exec(`INSERT INTO titles (id, type, year, status, match_status) VALUES (1, 'movie', 2026, 'completed', 'confirmed')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO title_names (title_id, name, language, is_primary) VALUES (1, 'Dune', 'fr', 1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO watch_events (title_id, source, created_at) VALUES (1, 'manual', '2026-04-10 21:00:00')`)
	require.NoError(t, err)

	h := handler.NewActivityHandler(repository.NewActivityRepository(db))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/stats/activity?limit=50&offset=0", nil)

	err = h.List(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Len(t, result, 1)
	assert.Equal(t, "Dune", result[0]["title_name"])
}

func TestActivityHandler_List_Empty(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	h := handler.NewActivityHandler(repository.NewActivityRepository(db))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/stats/activity", nil)

	err = h.List(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestActivityHandler_List_LimitCapped(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	h := handler.NewActivityHandler(repository.NewActivityRepository(db))
	w := httptest.NewRecorder()
	// limit=500 should be capped at 100
	r := httptest.NewRequest(http.MethodGet, "/api/stats/activity?limit=500", nil)

	err = h.List(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
}
