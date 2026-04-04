package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
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
	h.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var titles []model.Title
	json.NewDecoder(rr.Body).Decode(&titles)
	assert.Len(t, titles, 1)
	assert.Equal(t, "Dune", titles[0].PrimaryName())
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
	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var title model.Title
	json.NewDecoder(rr.Body).Decode(&title)
	assert.Equal(t, "Dune: Part Two", title.PrimaryName())
}

func TestTitleHandler_GetByID(t *testing.T) {
	h, titleRepo := setupHandler(t)

	id, _ := titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	r := chi.NewRouter()
	r.Get("/api/titles/{id}", h.GetByID)

	req := httptest.NewRequest("GET", "/api/titles/"+itoa(id), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var title model.Title
	json.NewDecoder(rr.Body).Decode(&title)
	assert.Equal(t, "Test", title.PrimaryName())
}

func itoa(i int64) string {
	return json.Number(json.Number(string(rune('0') + rune(i)))).String()
}
