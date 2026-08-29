package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/go-chi/chi/v5"
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
		if r.URL.Path == "/api/v3/movie/lookup" {
			_, _ = w.Write([]byte(`[{"title":"Movie Arr","tmdbId":1234}]`))
			return
		}
		if r.URL.Path == "/api/v3/movie" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":42,"title":"Movie Arr"}`))
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
	arrSvc := service.NewArrService(cfg, settingsRepo, titlesRepo, db)
	h := handler.NewArrHandler(arrSvc, titlesRepo, db)

	r := chi.NewRouter()
	r.Get("/api/arr/{app}/rootfolder", httputil.WrapHandler(h.ProxyRootFolder))
	r.Get("/api/arr/{app}/qualityprofile", httputil.WrapHandler(h.ProxyQualityProfile))
	r.Get("/api/arr/title/{id}", httputil.WrapHandler(h.GetTitleArr))
	r.Put("/api/arr/title/{id}", httputil.WrapHandler(h.UpdateTitleArr))
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

	t.Run("PushToArr pushes directly to Radarr and saves radarr_id", func(t *testing.T) {
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
		var res map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		assert.Equal(t, "ok", res["status"])
		assert.Equal(t, float64(42), res["arr_id"])

		updated, err := titlesRepo.GetByID(movieID)
		require.NoError(t, err)
		require.NotNil(t, updated.RadarrID)
		assert.Equal(t, int64(42), *updated.RadarrID)

		// Test GetTitleArr
		getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/arr/title/%d", movieID), nil)
		getRR := httptest.NewRecorder()
		r.ServeHTTP(getRR, getReq)
		assert.Equal(t, http.StatusOK, getRR.Code)
		var getRes map[string]any
		require.NoError(t, json.NewDecoder(getRR.Body).Decode(&getRes))
		assert.Equal(t, "radarr", getRes["app"])
	})
}
