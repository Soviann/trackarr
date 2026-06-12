package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchEventHandler_List_Empty(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	h := handler.NewMatchEventHandler(repository.NewMatchEventRepository(db))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/match-events", nil)

	err = h.List(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	events, ok := result["events"]
	require.True(t, ok)
	// Must be [] not null
	assert.IsType(t, []any{}, events)
	assert.Empty(t, events)
}

func TestMatchEventHandler_List_WithEvents(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	// Insert a title and two match events
	_, err = db.Exec(`INSERT INTO titles (id, type, year, status, match_status) VALUES (1, 'movie', 2024, 'watching', 'confirmed')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO match_events (title_id, kind, detail) VALUES (1, ?, 'detail-1')`, model.MatchEventAutoConfirmed)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO match_events (title_id, kind, detail) VALUES (1, ?, 'detail-2')`, model.MatchEventSeasonAttached)
	require.NoError(t, err)

	h := handler.NewMatchEventHandler(repository.NewMatchEventRepository(db))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/match-events", nil)

	err = h.List(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	events, ok := result["events"].([]any)
	require.True(t, ok)
	assert.Len(t, events, 2)
}

func TestMatchEventHandler_List_LimitParam(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	// Insert 5 events
	_, err = db.Exec(`INSERT INTO titles (id, type, year, status, match_status) VALUES (1, 'movie', 2024, 'watching', 'confirmed')`)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err = db.Exec(`INSERT INTO match_events (title_id, kind, detail) VALUES (1, ?, 'detail')`, model.MatchEventAutoConfirmed)
		require.NoError(t, err)
	}

	h := handler.NewMatchEventHandler(repository.NewMatchEventRepository(db))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/match-events?limit=2", nil)

	err = h.List(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	events, ok := result["events"].([]any)
	require.True(t, ok)
	assert.Len(t, events, 2)
}

func TestMatchEventHandler_List_LimitCapped(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	h := handler.NewMatchEventHandler(repository.NewMatchEventRepository(db))
	w := httptest.NewRecorder()
	// limit=500 should be capped at 100 and not error
	r := httptest.NewRequest(http.MethodGet, "/api/match-events?limit=500", nil)

	err = h.List(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
}
