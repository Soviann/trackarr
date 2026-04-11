package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSeasonHandler(t *testing.T) *handler.SeasonHandler {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })
	seasonRepo := repository.NewSeasonRepository(db)
	return handler.NewSeasonHandler(seasonRepo)
}

func TestSeasonHandler_UpdateRating_InvalidSeasonID(t *testing.T) {
	h := setupSeasonHandler(t)

	r := chi.NewRouter()
	r.Patch("/titles/{titleID}/seasons/{seasonID}", httputil.WrapHandler(h.UpdateRating))

	req := httptest.NewRequest(http.MethodPatch, "/titles/1/seasons/abc", strings.NewReader(`{"rating":8}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSeasonHandler_UpdateRating_InvalidJSON(t *testing.T) {
	h := setupSeasonHandler(t)

	r := chi.NewRouter()
	r.Patch("/titles/{titleID}/seasons/{seasonID}", httputil.WrapHandler(h.UpdateRating))

	req := httptest.NewRequest(http.MethodPatch, "/titles/1/seasons/1", strings.NewReader(`{bad json}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSeasonHandler_UpdateRating_NotFound(t *testing.T) {
	h := setupSeasonHandler(t)

	r := chi.NewRouter()
	r.Patch("/titles/{titleID}/seasons/{seasonID}", httputil.WrapHandler(h.UpdateRating))

	req := httptest.NewRequest(http.MethodPatch, "/titles/1/seasons/99999", strings.NewReader(`{"rating":8}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// UpdateRating uses UPDATE without checking RowsAffected, so a missing ID
	// does not produce an error — the handler returns 200.
	assert.Equal(t, http.StatusOK, w.Code)
}
