package handler_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/service/matching"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/Soviann/trackarr/internal/version"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdminSettingsTestEnv(t *testing.T) (*sql.DB, *repository.SettingRepository, *service.DynamicConfigReloader, *handler.AdminSettingsHandler) {
	t.Helper()
	writeDB, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(writeDB))

	t.Cleanup(func() {
		writeDB.Close()
	})

	settingRepo := repository.NewSettingRepository(writeDB)
	cfg := &config.Config{
		TMDBAPIKey:            "env-tmdb-key-12345",
		JellyfinWebhookSecret: "env-jellyfin-secret",
		PlexWebhookSecret:     "env-plex-secret",
	}

	pipeline := matching.NewPipeline(nil, nil, nil, nil, "")
	reloader := service.NewDynamicConfigReloader(cfg, writeDB, settingRepo, pipeline, nil, nil, nil, nil, nil)
	h := handler.NewAdminSettingsHandler(writeDB, settingRepo, reloader)

	return writeDB, settingRepo, reloader, h
}

func TestGetSystemSettings(t *testing.T) {
	_, _, _, h := setupAdminSettingsTestEnv(t)

	req := httptest.NewRequest("GET", "/api/admin/system-settings", nil)
	req.Host = "tracker.example.com"
	rr := httptest.NewRecorder()

	require.NoError(t, h.GetSystemSettings(rr, req))
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp handler.SystemSettingsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	assert.Equal(t, version.Version, resp.AppVersion)

	// TMDB should fall back to env key (masked)
	assert.True(t, resp.TMDBConfigured)
	assert.True(t, strings.HasPrefix(resp.TMDBAPIKey, "••••••••"))

	// Webhook URLs
	assert.Equal(t, "env-jellyfin-secret", resp.JellyfinWebhookSecret)
	assert.Equal(t, "http://tracker.example.com/api/webhook/jellyfin/env-jellyfin-secret", resp.JellyfinWebhookURL)
	assert.Equal(t, "env-plex-secret", resp.PlexWebhookSecret)
	assert.Equal(t, "http://tracker.example.com/api/webhook/plex/env-plex-secret", resp.PlexWebhookURL)

	// Default metadata language is fr
	assert.Equal(t, "fr", resp.MetadataLanguage)
	assert.Equal(t, "netflix,prime,disney,apple,max,canal,crunchyroll,paramount,adn", resp.EnabledWatchProviders)
	assert.True(t, resp.SagaChronologicalNextUp)
	assert.False(t, resp.AutoOpenIMDbOnRate)
	assert.True(t, resp.ICalGroupSameDay)
}

func TestUpdateSystemSettings_AndReload(t *testing.T) {
	db, settings, reloader, h := setupAdminSettingsTestEnv(t)

	updateBody := `{
		"tmdb_api_key": "new-sqlite-tmdb-key",
		"tvdb_api_key": "new-sqlite-tvdb-key",
		"radarr_url": "http://192.168.1.50:7878",
		"radarr_api_key": "radarr-key-999",
		"metadata_language": "en",
		"enabled_watch_providers": "netflix,prime,crunchyroll",
		"saga_chronological_next_up": "false",
		"auto_open_imdb_on_rate": "true",
		"ical_group_same_day": "false"
	}`

	req := httptest.NewRequest("PUT", "/api/admin/system-settings", strings.NewReader(updateBody))
	rr := httptest.NewRecorder()

	require.NoError(t, h.UpdateSystemSettings(rr, req))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Verify SQLite persistence
	tmdbKey, err := settings.Get("tmdb_api_key")
	require.NoError(t, err)
	assert.Equal(t, "new-sqlite-tmdb-key", tmdbKey)

	metaLang, err := settings.Get("metadata_language")
	require.NoError(t, err)
	assert.Equal(t, "en", metaLang)

	watchProviders, err := settings.Get("enabled_watch_providers")
	require.NoError(t, err)
	assert.Equal(t, "netflix,prime,crunchyroll", watchProviders)

	radarrURL, err := settings.Get("radarr_url")
	require.NoError(t, err)
	assert.Equal(t, "http://192.168.1.50:7878", radarrURL)

	sagaNextUp, err := settings.Get("saga_chronological_next_up")
	require.NoError(t, err)
	assert.Equal(t, "false", sagaNextUp)

	autoOpenIMDb, err := settings.Get("auto_open_imdb_on_rate")
	require.NoError(t, err)
	assert.Equal(t, "true", autoOpenIMDb)

	icalGroupSameDay, err := settings.Get(repository.SettingKeyICalGroupSameDay)
	require.NoError(t, err)
	assert.Equal(t, "false", icalGroupSameDay)

	// Verify dynamic reloader returns the new value
	assert.Equal(t, "new-sqlite-tmdb-key", reloader.Get("tmdb_api_key"))
	assert.Equal(t, "en", reloader.Get("metadata_language"))
	assert.Equal(t, "http://192.168.1.50:7878", reloader.Get("radarr_url"))
	assert.Equal(t, "false", reloader.Get("saga_chronological_next_up"))
	assert.Equal(t, "true", reloader.Get("auto_open_imdb_on_rate"))
	assert.Equal(t, "false", reloader.Get(repository.SettingKeyICalGroupSameDay))

	// Verify GET reflects persisted false value
	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/system-settings", nil)
	getRR := httptest.NewRecorder()
	require.NoError(t, h.GetSystemSettings(getRR, getReq))
	assert.Equal(t, http.StatusOK, getRR.Code)

	var resp handler.SystemSettingsResponse
	require.NoError(t, json.NewDecoder(getRR.Body).Decode(&resp))
	assert.False(t, resp.ICalGroupSameDay)

	_ = db
}

func TestSystemSettings_ICalGroupSameDay(t *testing.T) {
	t.Run("nominal default true and toggle round-trip", func(t *testing.T) {
		_, settings, _, h := setupAdminSettingsTestEnv(t)

		getRaw := func() map[string]any {
			req := httptest.NewRequest(http.MethodGet, "/api/admin/system-settings", nil)
			rr := httptest.NewRecorder()
			httputil.WrapHandler(h.GetSystemSettings)(rr, req)
			require.Equal(t, http.StatusOK, rr.Code)

			var payload map[string]any
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&payload))
			return payload
		}

		// Default when unset
		initial := getRaw()
		val, exists := initial["ical_group_same_day"]
		require.True(t, exists)
		assert.Equal(t, true, val)

		// Disable via PUT
		putFalse := httptest.NewRequest(http.MethodPut, "/api/admin/system-settings", strings.NewReader(`{"ical_group_same_day":"false"}`))
		rrFalse := httptest.NewRecorder()
		httputil.WrapHandler(h.UpdateSystemSettings)(rrFalse, putFalse)
		require.Equal(t, http.StatusNoContent, rrFalse.Code)

		stored, err := settings.Get(repository.SettingKeyICalGroupSameDay)
		require.NoError(t, err)
		assert.Equal(t, "false", stored)
		assert.Equal(t, false, getRaw()["ical_group_same_day"])

		// Re-enable via PUT
		putTrue := httptest.NewRequest(http.MethodPut, "/api/admin/system-settings", strings.NewReader(`{"ical_group_same_day":"true"}`))
		rrTrue := httptest.NewRecorder()
		httputil.WrapHandler(h.UpdateSystemSettings)(rrTrue, putTrue)
		require.Equal(t, http.StatusNoContent, rrTrue.Code)

		stored, err = settings.Get(repository.SettingKeyICalGroupSameDay)
		require.NoError(t, err)
		assert.Equal(t, "true", stored)
		assert.Equal(t, true, getRaw()["ical_group_same_day"])
	})

	t.Run("boundary values and nil reloader fallback", func(t *testing.T) {
		cases := []struct {
			name     string
			dbValue  string
			expected bool
		}{
			{name: "empty string defaults to true", dbValue: "", expected: true},
			{name: "lowercase false disables", dbValue: "false", expected: false},
			{name: "uppercase FALSE disables", dbValue: "FALSE", expected: false},
			{name: "whitespace around false disables", dbValue: " false ", expected: false},
			{name: "true enables", dbValue: "true", expected: true},
			{name: "arbitrary non-false string defaults to true", dbValue: "enabled", expected: true},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				db, settings, _, h := setupAdminSettingsTestEnv(t)
				testutil.SetSetting(t, db, repository.SettingKeyICalGroupSameDay, tc.dbValue)

				req := httptest.NewRequest(http.MethodGet, "/api/admin/system-settings", nil)
				rr := httptest.NewRecorder()
				httputil.WrapHandler(h.GetSystemSettings)(rr, req)
				require.Equal(t, http.StatusOK, rr.Code)

				var resp handler.SystemSettingsResponse
				require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
				assert.Equal(t, tc.expected, resp.ICalGroupSameDay)

				// Also verify direct SettingRepository read when reloader is nil
				noReloaderHandler := handler.NewAdminSettingsHandler(db, settings, nil)
				req2 := httptest.NewRequest(http.MethodGet, "/api/admin/system-settings", nil)
				rr2 := httptest.NewRecorder()
				httputil.WrapHandler(noReloaderHandler.GetSystemSettings)(rr2, req2)
				require.Equal(t, http.StatusOK, rr2.Code)

				var resp2 handler.SystemSettingsResponse
				require.NoError(t, json.NewDecoder(rr2.Body).Decode(&resp2))
				assert.Equal(t, tc.expected, resp2.ICalGroupSameDay)
			})
		}

		// Whitespace passed via PUT is trimmed before saving
		_, settings, _, h := setupAdminSettingsTestEnv(t)
		putReq := httptest.NewRequest(http.MethodPut, "/api/admin/system-settings", strings.NewReader(`{"ical_group_same_day":" false "}`))
		putRR := httptest.NewRecorder()
		httputil.WrapHandler(h.UpdateSystemSettings)(putRR, putReq)
		require.Equal(t, http.StatusNoContent, putRR.Code)

		stored, err := settings.Get(repository.SettingKeyICalGroupSameDay)
		require.NoError(t, err)
		assert.Equal(t, "false", stored)
	})

	t.Run("error and rejected input handling", func(t *testing.T) {
		_, settings, _, h := setupAdminSettingsTestEnv(t)

		// Unknown keys are ignored while allowed keys are persisted
		reqUnknown := httptest.NewRequest(http.MethodPut, "/api/admin/system-settings", strings.NewReader(`{"unknown_key":"val","ical_group_same_day":"false"}`))
		rrUnknown := httptest.NewRecorder()
		httputil.WrapHandler(h.UpdateSystemSettings)(rrUnknown, reqUnknown)
		require.Equal(t, http.StatusNoContent, rrUnknown.Code)

		_, err := settings.Get("unknown_key")
		assert.True(t, errors.Is(err, sql.ErrNoRows))

		stored, err := settings.Get(repository.SettingKeyICalGroupSameDay)
		require.NoError(t, err)
		assert.Equal(t, "false", stored)

		// Non-string JSON value returns 400
		reqBadType := httptest.NewRequest(http.MethodPut, "/api/admin/system-settings", strings.NewReader(`{"ical_group_same_day":false}`))
		rrBadType := httptest.NewRecorder()
		httputil.WrapHandler(h.UpdateSystemSettings)(rrBadType, reqBadType)
		assert.Equal(t, http.StatusBadRequest, rrBadType.Code)

		// Malformed JSON returns 400
		reqMalformed := httptest.NewRequest(http.MethodPut, "/api/admin/system-settings", strings.NewReader(`{"ical_group_same_day":`))
		rrMalformed := httptest.NewRecorder()
		httputil.WrapHandler(h.UpdateSystemSettings)(rrMalformed, reqMalformed)
		assert.Equal(t, http.StatusBadRequest, rrMalformed.Code)

		// Nil writeDB returns 500
		nilDBHandler := handler.NewAdminSettingsHandler(nil, settings, nil)
		reqNilDB := httptest.NewRequest(http.MethodPut, "/api/admin/system-settings", strings.NewReader(`{"ical_group_same_day":"false"}`))
		rrNilDB := httptest.NewRecorder()
		httputil.WrapHandler(nilDBHandler.UpdateSystemSettings)(rrNilDB, reqNilDB)
		assert.Equal(t, http.StatusInternalServerError, rrNilDB.Code)
	})
}

func TestTestArr_Validation(t *testing.T) {
	_, _, _, h := setupAdminSettingsTestEnv(t)

	r := chi.NewRouter()
	r.Post("/api/admin/system-settings/test/{app}", func(w http.ResponseWriter, req *http.Request) {
		_ = h.TestArr(w, req)
	})

	// Missing URL/Key
	req := httptest.NewRequest("POST", "/api/admin/system-settings/test/radarr", strings.NewReader(`{"url":"","api_key":""}`))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, false, resp["ok"])
	assert.Contains(t, resp["error"], "URL et clé d'API requises")
}

func TestGenerateVAPIDKeys(t *testing.T) {
	_, settings, _, h := setupAdminSettingsTestEnv(t)

	req := httptest.NewRequest("POST", "/api/admin/system-settings/vapid/generate", strings.NewReader(`{"subject":"mailto:admin@domain.com"}`))
	rr := httptest.NewRecorder()

	require.NoError(t, h.GenerateVAPIDKeys(rr, req))
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, true, resp["ok"])
	assert.NotEmpty(t, resp["vapid_public_key"])
	assert.Equal(t, "mailto:admin@domain.com", resp["vapid_subject"])

	// Check DB persistence
	pubKey, err := settings.Get("vapid_public_key")
	require.NoError(t, err)
	assert.NotEmpty(t, pubKey)

	privKey, err := settings.Get("vapid_private_key")
	require.NoError(t, err)
	assert.NotEmpty(t, privKey)
}

func TestSyncJellyfinMovies(t *testing.T) {
	t.Run("nominal sync with empty body creates unwatched movies and skips idempotently", func(t *testing.T) {
		db, _, _, h := setupAdminSettingsTestEnv(t)

		var gotToken, gotPath, gotQuery string
		jfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotToken = r.Header.Get("X-Emby-Token")
			gotPath = r.URL.Path
			gotQuery = r.URL.RawQuery

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{
						"Id":             "jf-m1",
						"Name":           "Dune: Part Two",
						"Type":           "Movie",
						"ProductionYear": 2024,
						"ProviderIds":    map[string]string{"Tmdb": "693134", "Imdb": "tt15239678"},
						"UserData":       map[string]any{"Played": false},
					},
					{
						"Id":             "jf-m2",
						"Name":           "Watched Film",
						"Type":           "Movie",
						"ProductionYear": 2020,
						"ProviderIds":    map[string]string{"Tmdb": "111"},
						"UserData":       map[string]any{"Played": true},
					},
					{
						"Id":             "jf-s1",
						"Name":           "Some Series",
						"Type":           "Series",
						"ProductionYear": 2023,
						"ProviderIds":    map[string]string{"Tmdb": "222"},
					},
				},
				"TotalRecordCount": 3,
			})
		}))
		defer jfServer.Close()

		testutil.SetSetting(t, db, repository.SettingKeyJellyfinURL, jfServer.URL)
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinAPIKey, "jf-secret-token")
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinUserID, "user-42")

		req := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", nil)
		rr := httptest.NewRecorder()
		httputil.WrapHandler(h.SyncJellyfinMovies)(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "jf-secret-token", gotToken)
		assert.Equal(t, "/Users/user-42/Items", gotPath)
		assert.Equal(t, "includeItemTypes=Movie&recursive=true&fields=ProviderIds", gotQuery)

		var summary service.JellyfinMovieSyncSummary
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&summary))
		assert.False(t, summary.DryRun)
		assert.Equal(t, 3, summary.Fetched)
		assert.Equal(t, 1, summary.Unwatched)
		assert.Equal(t, 1, summary.Created)
		assert.Equal(t, 0, summary.Skipped)
		assert.Equal(t, 0, summary.Errors)

		// Second call skips existing movie
		req2 := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", strings.NewReader(""))
		rr2 := httptest.NewRecorder()
		httputil.WrapHandler(h.SyncJellyfinMovies)(rr2, req2)
		require.Equal(t, http.StatusOK, rr2.Code)

		var summary2 service.JellyfinMovieSyncSummary
		require.NoError(t, json.NewDecoder(rr2.Body).Decode(&summary2))
		assert.Equal(t, 0, summary2.Created)
		assert.Equal(t, 1, summary2.Skipped)

		titles, err := repository.NewTitleRepository(db).ListAll()
		require.NoError(t, err)
		require.Len(t, titles, 1)
		assert.Equal(t, model.TitleStatusPlanToWatch, titles[0].Status)
		assert.NotNil(t, titles[0].DownloadedAt)
		assert.True(t, titles[0].ArrIgnored)

		tasks, err := repository.NewTaskRepository(db).ListPending()
		require.NoError(t, err)
		require.Len(t, tasks, 1)
		assert.Equal(t, model.TaskTypeEnrichment, tasks[0].TaskType)
	})

	t.Run("dry_run true counts without persisting", func(t *testing.T) {
		db, _, _, h := setupAdminSettingsTestEnv(t)

		jfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{
						"Id":             "jf-dry-1",
						"Name":           "Dry Movie",
						"Type":           "Movie",
						"ProductionYear": 2024,
						"ProviderIds":    map[string]string{"Tmdb": "99999"},
					},
				},
			})
		}))
		defer jfServer.Close()

		testutil.SetSetting(t, db, repository.SettingKeyJellyfinURL, jfServer.URL)
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinAPIKey, "jf-secret-token")
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinUserID, "user-42")

		req := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", strings.NewReader(`{"dry_run":true}`))
		rr := httptest.NewRecorder()
		httputil.WrapHandler(h.SyncJellyfinMovies)(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		var summary service.JellyfinMovieSyncSummary
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&summary))
		assert.True(t, summary.DryRun)
		assert.Equal(t, 1, summary.Fetched)
		assert.Equal(t, 1, summary.Unwatched)
		assert.Equal(t, 1, summary.Created)
		assert.Equal(t, 0, summary.Skipped)

		titles, err := repository.NewTitleRepository(db).ListAll()
		require.NoError(t, err)
		assert.Empty(t, titles)

		tasks, err := repository.NewTaskRepository(db).ListPending()
		require.NoError(t, err)
		assert.Empty(t, tasks)
	})

	t.Run("missing or partial Jellyfin credentials return 400", func(t *testing.T) {
		cases := []struct {
			name   string
			url    string
			apiKey string
			userID string
		}{
			{name: "all empty"},
			{name: "missing api key", url: "http://jellyfin.local", userID: "u1"},
			{name: "missing user id", url: "http://jellyfin.local", apiKey: "k1"},
			{name: "whitespace only", url: "   ", apiKey: "k1", userID: "u1"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				db, _, _, h := setupAdminSettingsTestEnv(t)
				if tc.url != "" {
					testutil.SetSetting(t, db, repository.SettingKeyJellyfinURL, tc.url)
				}
				if tc.apiKey != "" {
					testutil.SetSetting(t, db, repository.SettingKeyJellyfinAPIKey, tc.apiKey)
				}
				if tc.userID != "" {
					testutil.SetSetting(t, db, repository.SettingKeyJellyfinUserID, tc.userID)
				}

				req := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", nil)
				rr := httptest.NewRecorder()
				httputil.WrapHandler(h.SyncJellyfinMovies)(rr, req)
				assert.Equal(t, http.StatusBadRequest, rr.Code)
			})
		}
	})

	t.Run("malformed JSON body returns 400", func(t *testing.T) {
		db, _, _, h := setupAdminSettingsTestEnv(t)
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinURL, "http://jellyfin.local")
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinAPIKey, "jf-secret-token")
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinUserID, "user-42")

		req := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", strings.NewReader(`{"dry_run":`))
		rr := httptest.NewRecorder()
		httputil.WrapHandler(h.SyncJellyfinMovies)(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("upstream Jellyfin failure returns 500 without leaking secrets", func(t *testing.T) {
		db, _, _, h := setupAdminSettingsTestEnv(t)
		secretKey := "super-secret-jellyfin-api-key-999"

		jfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("invalid token: " + r.Header.Get("X-Emby-Token")))
		}))
		defer jfServer.Close()

		testutil.SetSetting(t, db, repository.SettingKeyJellyfinURL, jfServer.URL)
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinAPIKey, secretKey)
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinUserID, "user-42")

		req := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", nil)
		rr := httptest.NewRecorder()
		httputil.WrapHandler(h.SyncJellyfinMovies)(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := rr.Body.String()
		assert.Contains(t, body, "Failed to sync movies from Jellyfin")
		assert.NotContains(t, body, secretKey)
		assert.NotContains(t, body, jfServer.URL)
	})

	t.Run("nil writeDB returns 500", func(t *testing.T) {
		h := handler.NewAdminSettingsHandler(nil, nil, nil)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", nil)
		rr := httptest.NewRecorder()
		httputil.WrapHandler(h.SyncJellyfinMovies)(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
