package service_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/testutil"
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

	arrSvc := service.NewArrService(cfg, settingsRepo, db)

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
	arrSvc := service.NewArrService(cfg, settingsRepo, db)

	resp, err := arrSvc.ProxyRequest(ctx, "sonarr", "GET", "/api/v3/qualityprofile", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/api/v3/qualityprofile", requestedURI)
}
