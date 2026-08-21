package handler_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func setupEpisodeHandler(t *testing.T) (*handler.EpisodeHandler, *sql.DB) {
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
	// Real BackfillService (nil TMDB) so the post-commit current-season cascade
	// runs and the handler's reload-after-backfill path is exercised end-to-end.
	backfill := service.NewBackfillService(db, nil)
	libSvc := service.NewLibraryService(db, titleRepo, seasonRepo, episodeRepo, eventRepo, settingRepo, service.NewNoopNotifier(), backfill, nil)
	return handler.NewEpisodeHandler(db, libSvc, titleRepo), db
}

func TestEpisodeHandler_ToggleWatched_InvalidTitleID(t *testing.T) {
	h, _ := setupEpisodeHandler(t)

	r := chi.NewRouter()
	r.Patch("/titles/{titleID}/episodes/{episodeID}", httputil.WrapHandler(h.ToggleWatched))

	req := httptest.NewRequest(http.MethodPatch, "/titles/abc/episodes/1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestEpisodeHandler_ToggleWatched_InvalidEpisodeID(t *testing.T) {
	h, _ := setupEpisodeHandler(t)

	r := chi.NewRouter()
	r.Patch("/titles/{titleID}/episodes/{episodeID}", httputil.WrapHandler(h.ToggleWatched))

	req := httptest.NewRequest(http.MethodPatch, "/titles/1/episodes/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestEpisodeHandler_BatchMarkWatched_InvalidTitleID(t *testing.T) {
	h, _ := setupEpisodeHandler(t)

	r := chi.NewRouter()
	r.Post("/titles/{titleID}/episodes/batch-watch", httputil.WrapHandler(h.BatchMarkWatched))

	body := strings.NewReader(`{"episode_ids":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/titles/abc/episodes/batch-watch", body)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestEpisodeHandler_BatchMarkWatched_InvalidJSON(t *testing.T) {
	h, _ := setupEpisodeHandler(t)

	r := chi.NewRouter()
	r.Post("/titles/{titleID}/episodes/batch-watch", httputil.WrapHandler(h.BatchMarkWatched))

	body := strings.NewReader("not json")
	req := httptest.NewRequest(http.MethodPost, "/titles/1/episodes/batch-watch", body)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestEpisodeHandler_ToggleWatched_ResponseReflectsBackfillCascade guards the
// bug where marking an episode watched cascaded the previous episodes to
// watched server-side, but the PATCH response carried the pre-cascade title
// snapshot — so the UI showed stale season state until the next interaction.
// The handler now reloads the title after the post-commit backfill.
func TestEpisodeHandler_ToggleWatched_ResponseReflectsBackfillCascade(t *testing.T) {
	h, db := setupEpisodeHandler(t)

	titleID := testutil.InsertTitle(t, db, "Cascade Show", false)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	ep1 := testutil.GetOrCreateEpisode(t, db, seasonID, 1)
	ep2 := testutil.GetOrCreateEpisode(t, db, seasonID, 2)
	ep3 := testutil.GetOrCreateEpisode(t, db, seasonID, 3)

	r := chi.NewRouter()
	r.Patch("/titles/{titleID}/episodes/{episodeID}", httputil.WrapHandler(h.ToggleWatched))

	// Mark E03 watched — the backfill should cascade E01 and E02 to watched.
	req := httptest.NewRequest(http.MethodPatch,
		"/titles/"+strconv.FormatInt(titleID, 10)+"/episodes/"+strconv.FormatInt(ep3.ID, 10), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var got model.Title
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Len(t, got.Seasons, 1)

	watched := map[int64]bool{}
	for _, e := range got.Seasons[0].Episodes {
		watched[e.ID] = e.Watched
	}
	// All three must be watched in the SAME response — not just E03.
	assert.True(t, watched[ep1.ID], "E01 should be cascaded watched in the response")
	assert.True(t, watched[ep2.ID], "E02 should be cascaded watched in the response")
	assert.True(t, watched[ep3.ID], "E03 should be watched in the response")
}

func TestEpisodeHandler_ToggleWatched_PlanToWatchToWatching_WithReturningAndCaughtUp(t *testing.T) {
	h, db := setupEpisodeHandler(t)
	titleRepo := repository.NewTitleRepository(db)

	returning := model.SeriesStatusReturning
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:         model.TitleTypeSeries,
		IsAnime:      true,
		Year:         2024,
		Status:       model.TitleStatusPlanToWatch,
		SeriesStatus: &returning,
		MatchStatus:  model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Sword Anime", Language: "en", IsPrimary: true}})

	s1 := testutil.GetOrCreateSeason(t, db, titleID, 1)
	ep1 := testutil.SeedEpisode(t, db, s1.ID, 1, "2024-01-01", false)
	s2 := testutil.GetOrCreateSeason(t, db, titleID, 2)
	_ = testutil.SeedEpisode(t, db, s2.ID, 1, "2099-01-01", false) // future TBA episode

	r := chi.NewRouter()
	r.Patch("/titles/{titleID}/episodes/{episodeID}", httputil.WrapHandler(h.ToggleWatched))

	req := httptest.NewRequest(http.MethodPatch,
		"/titles/"+strconv.FormatInt(titleID, 10)+"/episodes/"+strconv.FormatInt(ep1.ID, 10), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var got model.Title
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, model.TitleStatusWatching, got.Status, "response title should transition to watching")

	// Verify in TitleRepository.List (which computes CaughtUp dynamically)
	res, err := titleRepo.List(repository.TitleFilter{Search: ptr("Sword Anime")})
	require.NoError(t, err)
	require.Len(t, res.Titles, 1)
	assert.Equal(t, model.TitleStatusWatching, res.Titles[0].Status)
	assert.True(t, res.Titles[0].CaughtUp, "should be caught up because future TBA episode is not counted as behind")
}

func TestEpisodeHandler_ToggleWatched_PlanToWatchToCompleted_WhenSeriesEnded(t *testing.T) {
	h, db := setupEpisodeHandler(t)

	ended := model.SeriesStatusEnded
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:         model.TitleTypeSeries,
		Year:         2024,
		Status:       model.TitleStatusPlanToWatch,
		SeriesStatus: &ended,
		MatchStatus:  model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Ended Show", Language: "en", IsPrimary: true}})

	s1 := testutil.GetOrCreateSeason(t, db, titleID, 1)
	ep1 := testutil.SeedEpisode(t, db, s1.ID, 1, "2024-01-01", false)

	r := chi.NewRouter()
	r.Patch("/titles/{titleID}/episodes/{episodeID}", httputil.WrapHandler(h.ToggleWatched))

	req := httptest.NewRequest(http.MethodPatch,
		"/titles/"+strconv.FormatInt(titleID, 10)+"/episodes/"+strconv.FormatInt(ep1.ID, 10), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var got model.Title
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, model.TitleStatusCompleted, got.Status, "response title should transition to completed when all episodes of ended series are watched")
}

func TestEpisodeHandler_BatchMarkWatched_PlanToWatchToCompleted(t *testing.T) {
	h, db := setupEpisodeHandler(t)

	ended := model.SeriesStatusEnded
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:         model.TitleTypeSeries,
		Year:         2024,
		Status:       model.TitleStatusPlanToWatch,
		SeriesStatus: &ended,
		MatchStatus:  model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Ended Batch", Language: "en", IsPrimary: true}})

	s1 := testutil.GetOrCreateSeason(t, db, titleID, 1)
	ep1 := testutil.SeedEpisode(t, db, s1.ID, 1, "2024-01-01", false)
	ep2 := testutil.SeedEpisode(t, db, s1.ID, 2, "2024-01-08", false)

	r := chi.NewRouter()
	r.Post("/titles/{titleID}/episodes/batch-watch", httputil.WrapHandler(h.BatchMarkWatched))

	body := strings.NewReader(fmt.Sprintf(`{"episode_ids": [%d, %d]}`, ep1.ID, ep2.ID))
	req := httptest.NewRequest(http.MethodPost, "/titles/"+strconv.FormatInt(titleID, 10)+"/episodes/batch-watch", body)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var got model.Title
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, model.TitleStatusCompleted, got.Status)
}

func ptr[T any](v T) *T {
	return &v
}
