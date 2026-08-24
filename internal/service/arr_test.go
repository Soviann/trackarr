package service_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArrService_ProxyRequest_PreservesBasePath(t *testing.T) {
	var requestedURI string
	var apiKey string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedURI = r.RequestURI
		apiKey = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"path":"/movies"}]`))
	}))
	defer ts.Close()

	// Base URL includes a subpath (e.g. ts.URL + "/radarr")
	baseURL := ts.URL + "/radarr"
	cfg := &config.Config{
		RadarrURL:    baseURL,
		RadarrAPIKey: "secret-api-key",
	}

	db := testutil.NewTestDB(t)
	defer db.Close()
	settingsRepo := repository.NewSettingRepository(db)

	arrSvc := service.NewArrService(cfg, settingsRepo, repository.NewTitleRepository(db), db)

	resp, err := arrSvc.ProxyRequest(context.Background(), "radarr", "GET", "/api/v3/rootfolder", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/radarr/api/v3/rootfolder", requestedURI)
	assert.Equal(t, "secret-api-key", apiKey)
}

func TestArrService_ProxyRequest_FallbackToDBSettings(t *testing.T) {
	var requestedURI string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedURI = r.RequestURI
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &config.Config{} // Empty env config
	db := testutil.NewTestDB(t)
	defer db.Close()

	ctx := context.Background()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		writer := repository.NewSettingWriter(tx)
		if err := writer.Set(ctx, "sonarr_url", ts.URL); err != nil {
			return err
		}
		return writer.Set(ctx, "sonarr_api_key", "db-sonarr-key")
	}))

	settingsRepo := repository.NewSettingRepository(db)
	arrSvc := service.NewArrService(cfg, settingsRepo, repository.NewTitleRepository(db), db)

	resp, err := arrSvc.ProxyRequest(ctx, "sonarr", "GET", "/api/v3/qualityprofile", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/api/v3/qualityprofile", requestedURI)
}

func TestArrService_ProxyRequest_PreservesApiKeyOnRedirect(t *testing.T) {
	var finalApiKey string
	var finalURI string

	tsTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		finalApiKey = r.Header.Get("X-Api-Key")
		finalURI = r.RequestURI
		w.WriteHeader(http.StatusOK)
	}))
	defer tsTarget.Close()

	tsRedirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, tsTarget.URL+"/api/v3/rootfolder", http.StatusMovedPermanently)
	}))
	defer tsRedirect.Close()

	cfg := &config.Config{
		RadarrURL:    tsRedirect.URL,
		RadarrAPIKey: "redirect-key-123",
	}

	db := testutil.NewTestDB(t)
	defer db.Close()
	settingsRepo := repository.NewSettingRepository(db)

	arrSvc := service.NewArrService(cfg, settingsRepo, repository.NewTitleRepository(db), db)

	resp, err := arrSvc.ProxyRequest(context.Background(), "radarr", "GET", "/api/v3/rootfolder", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/api/v3/rootfolder", finalURI)
	assert.Equal(t, "redirect-key-123", finalApiKey)
}

func TestArrService_GetTitleArrDetails_And_UpdateTitle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v3/movie/42" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":42,"titleSlug":"test-movie","monitored":true,"qualityProfileId":1,"rootFolderPath":"/movies","hasFile":true,"sizeOnDisk":1000}`))
			return
		}
		if r.URL.Path == "/api/v3/movie" && r.Method == http.MethodPut {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":42,"titleSlug":"test-movie","monitored":false,"qualityProfileId":2,"rootFolderPath":"/movies","hasFile":true,"sizeOnDisk":1000}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	cfg := &config.Config{
		RadarrURL:    ts.URL,
		RadarrAPIKey: "key-42",
	}

	db := testutil.NewTestDB(t)
	defer db.Close()
	settingsRepo := repository.NewSettingRepository(db)
	titlesRepo := repository.NewTitleRepository(db)

	radarrID := int64(42)
	tmdbID := int64(550)
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		RadarrID:    &radarrID,
		TMDBID:      &tmdbID,
	}, []model.TitleName{{Name: "Test Movie", Language: "en", IsPrimary: true}})

	arrSvc := service.NewArrService(cfg, settingsRepo, titlesRepo, db)

	details, err := arrSvc.GetTitleArrDetails(context.Background(), titleID)
	require.NoError(t, err)
	assert.True(t, details.Exists)
	assert.Equal(t, int64(42), details.ArrID)
	assert.Equal(t, "test-movie", details.TitleSlug)
	assert.Equal(t, ts.URL+"/movie/test-movie", details.WebURL)
	assert.True(t, details.Monitored)
	assert.True(t, details.HasFile)

	// Update title options
	updated, err := arrSvc.UpdateTitle(context.Background(), titleID, service.PushPayload{
		TitleID:        titleID,
		Monitored:      false,
		QualityProfile: 2,
		RootFolder:     "/movies",
	})
	require.NoError(t, err)
	assert.True(t, updated.Exists)
}
