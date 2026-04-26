package handler_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHandler(t *testing.T) (*handler.TitleHandler, *sql.DB, *repository.TitleRepository) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	eventRepo := repository.NewWatchEventRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	titleSvc := service.NewTitleService(db, titleRepo, taskRepo, nil)
	h := handler.NewTitleHandler(context.Background(), db, titleRepo, titleRepo, seasonRepo, episodeRepo, eventRepo, taskRepo, nil, titleSvc, nil)
	return h, db, titleRepo
}

func TestTitleHandler_List(t *testing.T) {
	h, db, _ := setupHandler(t)

	imdb := "tt1234567"
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed, IMDBID: &imdb}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Shogun", Language: "en", IsPrimary: true}})

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
	h, _, _ := setupHandler(t)

	// Since we can't easily mock the pipeline here without refactoring setupHandler
	// and ResolveURL depends on real API calls, we expect a failure or we need to mock it.
	// For now, let's at least test the 400 when URL is invalid.
	req := httptest.NewRequest("GET", "/api/titles/resolve?q=invalid", nil)
	rr := httptest.NewRecorder()
	err := h.Resolve(rr, req)
	assert.Error(t, err)
}

func TestTitleHandler_Create(t *testing.T) {
	h, _, _ := setupHandler(t)

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
	h, db, _ := setupHandler(t)

	id := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})
	accent := "#d4ad7a"
	testutil.UpdateTitle(t, db, id, repository.TitleUpdate{AccentHex: &accent})

	r := chi.NewRouter()
	r.Get("/api/titles/{id}", httputil.WrapHandler(h.GetByID))

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/titles/%d", id), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	body := rr.Body.Bytes()
	var title model.Title
	require.NoError(t, json.Unmarshal(body, &title))
	assert.Equal(t, "Test", title.PrimaryName())
	require.NotNil(t, title.AccentHex)
	assert.Equal(t, accent, *title.AccentHex)

	// JSON contract: the field is exposed under the snake_case key the frontend
	// reads, not just via the Go struct.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(body, &raw))
	assert.Equal(t, accent, raw["accent_hex"])
}

func TestTitleHandler_GetByID_InvalidID(t *testing.T) {
	h, _, _ := setupHandler(t)

	r := chi.NewRouter()
	r.Get("/api/titles/{id}", httputil.WrapHandler(h.GetByID))

	req := httptest.NewRequest("GET", "/api/titles/notanumber", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTitleHandler_GetByID_NotFound(t *testing.T) {
	h, _, _ := setupHandler(t)

	r := chi.NewRouter()
	r.Get("/api/titles/{id}", httputil.WrapHandler(h.GetByID))

	req := httptest.NewRequest("GET", "/api/titles/99999", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestTitleHandler_Create_InvalidJSON(t *testing.T) {
	h, _, _ := setupHandler(t)

	req := httptest.NewRequest("POST", "/api/titles", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/api/titles", httputil.WrapHandler(h.Create))
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTitleHandler_Update(t *testing.T) {
	h, db, _ := setupHandler(t)

	id := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

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

// TestTitleHandler_Update_StatusChange_EnqueuesSeasonPushes verifies that
// changing a series status enqueues one push task per season regardless of
// their watched-progress — status is the one signal AniList needs for every
// season, whether COMPLETED, CURRENT, or PLANNING.
func TestTitleHandler_Update_StatusChange_EnqueuesSeasonPushes(t *testing.T) {
	h, db, _ := setupHandler(t)

	titleID := testutil.InsertTitle(t, db, "JJK", true)
	s1 := testutil.InsertSeason(t, db, titleID, 1)
	s2 := testutil.InsertSeason(t, db, titleID, 2)

	body, _ := json.Marshal(map[string]any{"status": "dropped"})
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/titles/%d", titleID), bytes.NewReader(body))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Patch("/api/titles/{id}", httputil.WrapHandler(h.Update))
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	tasks, err := repository.NewTaskRepository(db).ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	seen := map[int64]bool{}
	for _, task := range tasks {
		assert.Equal(t, model.TaskTypeAniListPushSeason, task.TaskType)
		var p service.AniListPushSeasonPayload
		require.NoError(t, json.Unmarshal([]byte(task.Payload), &p))
		seen[p.SeasonID] = true
	}
	assert.True(t, seen[s1], "s1 push missing")
	assert.True(t, seen[s2], "s2 push missing")
}

// TestTitleHandler_Update_RatingOnly_SkipsNonEligibleSeasons verifies the
// rating-only path: AniList rejects scores on CURRENT/PLANNING entries, so
// the handler must filter seasons via ShouldPushRating. Only the COMPLETED
// season receives a push.
func TestTitleHandler_Update_RatingOnly_SkipsNonEligibleSeasons(t *testing.T) {
	h, db, _ := setupHandler(t)

	titleID := testutil.InsertTitle(t, db, "JJK", true)
	sCompleted := testutil.InsertSeason(t, db, titleID, 1)
	testutil.SetSeasonEpisodeCount(t, db, sCompleted, 12)
	testutil.MarkEpisodesWatched(t, db, sCompleted, 12)

	sCurrent := testutil.InsertSeason(t, db, titleID, 2)
	testutil.SetSeasonEpisodeCount(t, db, sCurrent, 12)
	testutil.MarkEpisodesWatched(t, db, sCurrent, 5)

	body, _ := json.Marshal(map[string]any{"my_rating": 9})
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/titles/%d", titleID), bytes.NewReader(body))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Patch("/api/titles/{id}", httputil.WrapHandler(h.Update))
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	tasks, err := repository.NewTaskRepository(db).ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1, "only the COMPLETED season should be pushed")

	var p service.AniListPushSeasonPayload
	require.NoError(t, json.Unmarshal([]byte(tasks[0].Payload), &p))
	assert.Equal(t, sCompleted, p.SeasonID)
}

// TestTitleHandler_Update_MovieStatusChange_EnqueuesMoviePush exercises the
// movie branch: no seasons, one movie push, gated by IsAnime + AniListID.
func TestTitleHandler_Update_MovieStatusChange_EnqueuesMoviePush(t *testing.T) {
	h, db, _ := setupHandler(t)

	titleID := testutil.InsertMovieTitle(t, db, "Your Name", 21519)

	body, _ := json.Marshal(map[string]any{"status": "completed"})
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/titles/%d", titleID), bytes.NewReader(body))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Patch("/api/titles/{id}", httputil.WrapHandler(h.Update))
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	tasks, err := repository.NewTaskRepository(db).ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskTypeAniListPushMovie, tasks[0].TaskType)
}

// TestTitleHandler_Update_NoEffectiveChange_NoPush verifies the short-circuit
// that prevents a re-PATCH of current values from spamming AniList.
func TestTitleHandler_Update_NoEffectiveChange_NoPush(t *testing.T) {
	h, db, _ := setupHandler(t)

	rating := 8
	titleID := testutil.CreateTitle(t, db,
		&model.Title{
			Type:        model.TitleTypeSeries,
			IsAnime:     true,
			Year:        2024,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
			MyRating:    &rating,
		},
		[]model.TitleName{{Name: "JJK", Language: "en", IsPrimary: true}},
	)
	testutil.InsertSeason(t, db, titleID, 1)

	body, _ := json.Marshal(map[string]any{"status": "watching", "my_rating": 8})
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/titles/%d", titleID), bytes.NewReader(body))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Patch("/api/titles/{id}", httputil.WrapHandler(h.Update))
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	tasks, err := repository.NewTaskRepository(db).ListPending()
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

// TestTitleHandler_Update_StatusAndRating_DedupesSeasonPushes guards against
// the combined-PATCH case (status + rating together): status alone would
// enqueue every season, and rating alone would enqueue eligible seasons
// again — de-duping avoids two tasks for the same season.
func TestTitleHandler_Update_StatusAndRating_DedupesSeasonPushes(t *testing.T) {
	h, db, _ := setupHandler(t)

	titleID := testutil.InsertTitle(t, db, "JJK", true)
	s1 := testutil.InsertSeason(t, db, titleID, 1)
	testutil.SetSeasonEpisodeCount(t, db, s1, 12)
	testutil.MarkEpisodesWatched(t, db, s1, 12)

	body, _ := json.Marshal(map[string]any{"status": "completed", "my_rating": 10})
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/titles/%d", titleID), bytes.NewReader(body))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Patch("/api/titles/{id}", httputil.WrapHandler(h.Update))
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	tasks, err := repository.NewTaskRepository(db).ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1, "combined PATCH must not double-push the same season")
}

func TestTitleHandler_Update_InvalidID(t *testing.T) {
	h, _, _ := setupHandler(t)

	body, _ := json.Marshal(map[string]interface{}{"status": "completed"})
	req := httptest.NewRequest("PUT", "/api/titles/abc", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Put("/api/titles/{id}", httputil.WrapHandler(h.Update))
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTitleHandler_Update_InvalidJSON(t *testing.T) {
	h, db, _ := setupHandler(t)

	id := testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/titles/%d", id), bytes.NewReader([]byte("{invalid")))
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Put("/api/titles/{id}", httputil.WrapHandler(h.Update))
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTitleHandler_List_WithSort(t *testing.T) {
	h, db, _ := setupHandler(t)

	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2020, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Old Movie", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "New Movie", Language: "en", IsPrimary: true}})

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
	h, _, _ := setupHandler(t)

	req := httptest.NewRequest("GET", "/api/titles?status=completed&sort=release_date&order=desc", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.List(rr, req))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTitleHandler_List_InvalidSortFallsBack(t *testing.T) {
	h, db, _ := setupHandler(t)

	testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	req := httptest.NewRequest("GET", "/api/titles?status=watching&sort=bobby_tables&order=sideways", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.List(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	var result repository.PaginatedResult
	_ = json.NewDecoder(rr.Body).Decode(&result)
	assert.Len(t, result.Titles, 1)
}

func TestTitleHandler_List_WithPagination(t *testing.T) {
	h, db, _ := setupHandler(t)

	for i := 0; i < 5; i++ {
		testutil.CreateTitle(t, db, &model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: fmt.Sprintf("Movie %d", i), Language: "en", IsPrimary: true}})
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

func TestTitleHandler_Delete(t *testing.T) {
	h, db, titleRepo := setupHandler(t)

	id := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "To Delete", Language: "en", IsPrimary: true}},
	)

	router := chi.NewRouter()
	router.Delete("/api/titles/{id}", httputil.WrapHandler(h.Delete))

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/titles/%d", id), nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Verify deleted
	_, err := titleRepo.GetByID(id)
	assert.Error(t, err)
}

func TestTitleHandler_BatchDelete(t *testing.T) {
	h, db, titleRepo := setupHandler(t)

	id1 := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Title 1", Language: "en", IsPrimary: true}},
	)
	id2 := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Title 2", Language: "en", IsPrimary: true}},
	)

	body := fmt.Sprintf(`{"ids":[%d,%d]}`, id1, id2)
	req := httptest.NewRequest(http.MethodPost, "/api/titles/batch-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	require.NoError(t, h.BatchDelete(rr, req))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	_, err := titleRepo.GetByID(id1)
	assert.Error(t, err)
	_, err = titleRepo.GetByID(id2)
	assert.Error(t, err)
}

func TestTitleHandler_ReviewCount(t *testing.T) {
	h, db, _ := setupHandler(t)

	// Insert titles with varying match_status
	testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusPendingReview},
		[]model.TitleName{{Name: "Pending Title", Language: "en", IsPrimary: true}},
	)
	testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusUnconfirmed},
		[]model.TitleName{{Name: "Unconfirmed Title", Language: "en", IsPrimary: true}},
	)
	testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, Year: 2022, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Confirmed Title", Language: "en", IsPrimary: true}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/titles/review-count", nil)
	rr := httptest.NewRecorder()

	require.NoError(t, h.ReviewCount(rr, req))
	assert.Equal(t, http.StatusOK, rr.Code)

	var result map[string]int
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.Equal(t, 2, result["count"])
}

func TestTitleHandler_BatchStatus(t *testing.T) {
	h, db, titleRepo := setupHandler(t)

	id1 := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Title A", Language: "en", IsPrimary: true}},
	)
	id2 := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Title B", Language: "en", IsPrimary: true}},
	)

	body := fmt.Sprintf(`{"ids":[%d,%d],"status":"completed"}`, id1, id2)
	req := httptest.NewRequest(http.MethodPost, "/api/titles/batch-status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	require.NoError(t, h.BatchStatus(rr, req))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	t1, err := titleRepo.GetByID(id1)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusCompleted, t1.Status)
}
