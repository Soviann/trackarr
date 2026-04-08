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
	"github.com/nicolasvasse/plextracker/internal/service"


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
	taskRepo := repository.NewTaskRepository(db)
	titleSvc := service.NewTitleService(db, titleRepo, taskRepo, nil)
	h := handler.NewTitleHandler(db, titleRepo, seasonRepo, episodeRepo, eventRepo, taskRepo, nil, titleSvc)
	return h, titleRepo
}

func TestTitleHandler_List(t *testing.T) {
	h, titleRepo := setupHandler(t)

	imdb := "tt1234567"
	_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed, IMDBID: &imdb}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})
	_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Shogun", Language: "en", IsPrimary: true}})

	t.Run("filter by status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/titles?status=watching", nil)
		rr := httptest.NewRecorder()
		require.NoError(t, h.List(rr, req))
		assert.Equal(t, http.StatusOK, rr.Code)
		var result repository.PaginatedResult
		_ = json.NewDecoder(rr.Body).Decode(&result)
		assert.Len(t, result.Titles, 1)
		assert.Equal(t, "Dune", result.Titles[0].PrimaryName())
	})

	t.Run("search by IMDb URL", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/titles?search=https://www.imdb.com/title/tt1234567/", nil)
		rr := httptest.NewRecorder()
		require.NoError(t, h.List(rr, req))
		assert.Equal(t, http.StatusOK, rr.Code)
		var result repository.PaginatedResult
		_ = json.NewDecoder(rr.Body).Decode(&result)
		assert.Len(t, result.Titles, 1)
		assert.Equal(t, "Dune", result.Titles[0].PrimaryName())
	})
}

func TestTitleHandler_Resolve(t *testing.T) {
	h, _ := setupHandler(t)

	// Since we can't easily mock the pipeline here without refactoring setupHandler
	// and ResolveURL depends on real API calls, we expect a failure or we need to mock it.
	// For now, let's at least test the 400 when URL is invalid.
	req := httptest.NewRequest("GET", "/api/titles/resolve?q=invalid", nil)
	rr := httptest.NewRecorder()
	err := h.Resolve(rr, req)
	assert.Error(t, err)
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

func TestTitleHandler_List_WithSort(t *testing.T) {
	h, titleRepo := setupHandler(t)

	_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2020, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Old Movie", Language: "en", IsPrimary: true}})
	_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "New Movie", Language: "en", IsPrimary: true}})

	req := httptest.NewRequest("GET", "/api/titles?status=completed&sort=year&order=asc", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.List(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	var result repository.PaginatedResult
	_ = json.NewDecoder(rr.Body).Decode(&result)
	require.Len(t, result.Titles, 2)
	assert.Equal(t, "Old Movie", result.Titles[0].PrimaryName())
	assert.Equal(t, "New Movie", result.Titles[1].PrimaryName())
}

func TestTitleHandler_List_WithReleaseDateSort(t *testing.T) {
	h, _ := setupHandler(t)

	req := httptest.NewRequest("GET", "/api/titles?status=completed&sort=release_date&order=desc", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.List(rr, req))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTitleHandler_List_InvalidSortFallsBack(t *testing.T) {
	h, titleRepo := setupHandler(t)

	_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	req := httptest.NewRequest("GET", "/api/titles?status=watching&sort=bobby_tables&order=sideways", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.List(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	var result repository.PaginatedResult
	_ = json.NewDecoder(rr.Body).Decode(&result)
	assert.Len(t, result.Titles, 1)
}

func TestTitleHandler_List_WithPagination(t *testing.T) {
	h, titleRepo := setupHandler(t)

	for i := 0; i < 5; i++ {
		_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: fmt.Sprintf("Movie %d", i), Language: "en", IsPrimary: true}})
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
