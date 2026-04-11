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
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupEpisodeHandler(t *testing.T) *handler.EpisodeHandler {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	eventRepo := repository.NewWatchEventRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	libSvc := service.NewLibraryService(db, titleRepo, seasonRepo, episodeRepo, eventRepo, settingRepo, service.NewNoopNotifier(), nil, nil)
	return handler.NewEpisodeHandler(db, libSvc)
}

func TestEpisodeHandler_ToggleWatched_InvalidTitleID(t *testing.T) {
	h := setupEpisodeHandler(t)

	r := chi.NewRouter()
	r.Patch("/titles/{titleID}/episodes/{episodeID}", httputil.WrapHandler(h.ToggleWatched))

	req := httptest.NewRequest(http.MethodPatch, "/titles/abc/episodes/1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestEpisodeHandler_ToggleWatched_InvalidEpisodeID(t *testing.T) {
	h := setupEpisodeHandler(t)

	r := chi.NewRouter()
	r.Patch("/titles/{titleID}/episodes/{episodeID}", httputil.WrapHandler(h.ToggleWatched))

	req := httptest.NewRequest(http.MethodPatch, "/titles/1/episodes/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestEpisodeHandler_BatchMarkWatched_InvalidTitleID(t *testing.T) {
	h := setupEpisodeHandler(t)

	r := chi.NewRouter()
	r.Post("/titles/{titleID}/episodes/batch-watch", httputil.WrapHandler(h.BatchMarkWatched))

	body := strings.NewReader(`{"episode_ids":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/titles/abc/episodes/batch-watch", body)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestEpisodeHandler_BatchMarkWatched_InvalidJSON(t *testing.T) {
	h := setupEpisodeHandler(t)

	r := chi.NewRouter()
	r.Post("/titles/{titleID}/episodes/batch-watch", httputil.WrapHandler(h.BatchMarkWatched))

	body := strings.NewReader("not json")
	req := httptest.NewRequest(http.MethodPost, "/titles/1/episodes/batch-watch", body)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
