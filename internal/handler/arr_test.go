package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArrHandler_Endpoints(t *testing.T) {
	db := testutil.NewTestDB(t)
	titlesRepo := repository.NewTitleRepository(db)
	settingsRepo := repository.NewSettingRepository(db)

	arrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v3/rootfolder" {
			_, _ = w.Write([]byte(`[{"path":"/movies"}]`))
			return
		}
		if r.URL.Path == "/api/v3/qualityprofile" {
			_, _ = w.Write([]byte(`[{"id":1,"name":"HD"}]`))
			return
		}
		http.NotFound(w, r)
	}))
	defer arrServer.Close()

	cfg := &config.Config{
		RadarrURL:    arrServer.URL,
		RadarrAPIKey: "fake-key",
		SonarrURL:    arrServer.URL,
		SonarrAPIKey: "fake-key",
	}
	arrSvc := service.NewArrService(cfg, settingsRepo, db)
	h := handler.NewArrHandler(arrSvc, titlesRepo, db)

	r := chi.NewRouter()
	r.Get("/api/arr/{app}/rootfolder", httputil.WrapHandler(h.ProxyRootFolder))
	r.Get("/api/arr/{app}/qualityprofile", httputil.WrapHandler(h.ProxyQualityProfile))
	r.Get("/api/admin/arr/queue", httputil.WrapHandler(h.ListArrQueue))
	r.Post("/api/titles/{id}/arr-push", httputil.WrapHandler(h.PushToArr))

	t.Run("ProxyRootFolder forwards to Arr instance", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/arr/radarr/rootfolder", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "/movies")
	})

	t.Run("ProxyQualityProfile forwards to Arr instance", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/arr/sonarr/qualityprofile", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "HD")
	})

	t.Run("ListArrQueue returns empty queue", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/arr/queue", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var res map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, false, res["has_more"])
	})

	t.Run("PushToArr enqueues movie push task", func(t *testing.T) {
		tmdb := int64(1234)
		movieID := testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2024,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
			TMDBID:      &tmdb,
		}, []model.TitleName{{Name: "Movie Arr", Language: "en", IsPrimary: true}})

		body, _ := json.Marshal(map[string]any{
			"monitored":       true,
			"search":          false,
			"root_folder":     "/movies",
			"quality_profile": 1,
		})
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/titles/%d/arr-push", movieID), bytes.NewReader(body))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		tasks, err := repository.NewTaskRepository(db).ListPending()
		require.NoError(t, err)
		require.Len(t, tasks, 1)
		assert.Equal(t, model.TaskTypeRadarrPush, tasks[0].TaskType)
	})
}
