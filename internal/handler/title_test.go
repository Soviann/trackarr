package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHandler(t *testing.T) (*handler.TitleHandler, *repository.TitleRepository) {
	t.Helper()
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	eventRepo := repository.NewWatchEventRepository(db)
	h := handler.NewTitleHandler(titleRepo, seasonRepo, episodeRepo, eventRepo)
	return h, titleRepo
}

func TestTitleHandler_List(t *testing.T) {
	h, titleRepo := setupHandler(t)

	_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})
	_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Shogun", Language: "en", IsPrimary: true}})

	req := httptest.NewRequest("GET", "/api/titles?status=watching", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.List(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	var result repository.PaginatedResult
	_ = json.NewDecoder(rr.Body).Decode(&result)
	assert.Len(t, result.Titles, 1)
	assert.Equal(t, "Dune", result.Titles[0].PrimaryName())
	assert.Equal(t, 1, result.Total)
	assert.False(t, result.HasMore)
}

func TestTitleHandler_Create(t *testing.T) {
	h, _ := setupHandler(t)

	body, _ := json.Marshal(map[string]interface{}{
		"type":         "movie",
		"year":         2024,
		"status":       "watching",
		"match_status": "confirmed",
		"names":        []map[string]interface{}{{"name": "Dune: Part Two", "language": "en", "is_primary": true}},
	})

	req := httptest.NewRequest("POST", "/api/titles", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	require.NoError(t, h.Create(rr, req))

	assert.Equal(t, http.StatusCreated, rr.Code)
	var title model.Title
	_ = json.NewDecoder(rr.Body).Decode(&title)
	assert.Equal(t, "Dune: Part Two", title.PrimaryName())
}

func TestTitleHandler_GetByID(t *testing.T) {
	h, titleRepo := setupHandler(t)

	id, _ := titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	r := chi.NewRouter()
	r.Get("/api/titles/{id}", httputil.WrapHandler(h.GetByID))

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/titles/%d", id), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var title model.Title
	_ = json.NewDecoder(rr.Body).Decode(&title)
	assert.Equal(t, "Test", title.PrimaryName())
}

func TestTitleHandler_GetByID_InvalidID(t *testing.T) {
	h, _ := setupHandler(t)

	r := chi.NewRouter()
	r.Get("/api/titles/{id}", httputil.WrapHandler(h.GetByID))

	req := httptest.NewRequest("GET", "/api/titles/notanumber", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTitleHandler_GetByID_NotFound(t *testing.T) {
	h, _ := setupHandler(t)

	r := chi.NewRouter()
	r.Get("/api/titles/{id}", httputil.WrapHandler(h.GetByID))

	req := httptest.NewRequest("GET", "/api/titles/99999", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestTitleHandler_Create_InvalidJSON(t *testing.T) {
	h, _ := setupHandler(t)

	req := httptest.NewRequest("POST", "/api/titles", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/api/titles", httputil.WrapHandler(h.Create))
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTitleHandler_Update(t *testing.T) {
	h, titleRepo := setupHandler(t)

	id, _ := titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	body, _ := json.Marshal(map[string]interface{}{"status": "completed"})
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/titles/%d", id), bytes.NewReader(body))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Put("/api/titles/{id}", httputil.WrapHandler(h.Update))
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var title model.Title
	_ = json.NewDecoder(rr.Body).Decode(&title)
	assert.Equal(t, model.TitleStatusCompleted, title.Status)
}

func TestTitleHandler_Update_InvalidID(t *testing.T) {
	h, _ := setupHandler(t)

	body, _ := json.Marshal(map[string]interface{}{"status": "completed"})
	req := httptest.NewRequest("PUT", "/api/titles/abc", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Put("/api/titles/{id}", httputil.WrapHandler(h.Update))
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTitleHandler_Update_InvalidJSON(t *testing.T) {
	h, titleRepo := setupHandler(t)

	id, _ := titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/titles/%d", id), bytes.NewReader([]byte("{invalid")))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Put("/api/titles/{id}", httputil.WrapHandler(h.Update))
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTitleHandler_List_WithPagination(t *testing.T) {
	h, titleRepo := setupHandler(t)

	for i := 0; i < 5; i++ {
		titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: fmt.Sprintf("Movie %d", i), Language: "en", IsPrimary: true}})
	}

	req := httptest.NewRequest("GET", "/api/titles?limit=2&offset=0", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.List(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	var result repository.PaginatedResult
	_ = json.NewDecoder(rr.Body).Decode(&result)
	assert.Len(t, result.Titles, 2)
	assert.Equal(t, 5, result.Total)
	assert.True(t, result.HasMore)
}
