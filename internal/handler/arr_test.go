package handler_test

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
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
	"github.com/Soviann/trackarr/internal/router"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
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

func TestArrHandler_DeleteTitleArr_Nominal(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		nilBody       bool
		arrIgnored    bool
		expectedQuery string
	}{
		{
			name:          "nil body defaults deleteFiles and addImportExclusion to true",
			nilBody:       true,
			expectedQuery: "/api/v3/movie/42?deleteFiles=true&addImportExclusion=true",
		},
		{
			name:          "empty string body defaults both flags to true",
			body:          "",
			expectedQuery: "/api/v3/movie/42?deleteFiles=true&addImportExclusion=true",
		},
		{
			name:          "empty JSON object defaults both flags to true",
			body:          `{}`,
			expectedQuery: "/api/v3/movie/42?deleteFiles=true&addImportExclusion=true",
		},
		{
			name:          "explicit false flags are forwarded to Radarr",
			body:          `{"delete_files":false,"add_import_exclusion":false}`,
			arrIgnored:    true,
			expectedQuery: "/api/v3/movie/42?deleteFiles=false&addImportExclusion=false",
		},
		{
			name:          "partial payload preserves true default for omitted delete_files",
			body:          `{"add_import_exclusion":false}`,
			expectedQuery: "/api/v3/movie/42?deleteFiles=true&addImportExclusion=false",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var deletedURI string
			var deleteCalls int
			arrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete && r.URL.Path == "/api/v3/movie/42" {
					deleteCalls++
					deletedURI = r.URL.String()
					w.WriteHeader(http.StatusOK)
					return
				}
				http.NotFound(w, r)
			}))
			defer arrServer.Close()

			db := testutil.NewTestDB(t)
			titlesRepo := repository.NewTitleRepository(db)
			settingsRepo := repository.NewSettingRepository(db)
			watchEventRepo := repository.NewWatchEventRepository(db)

			radarrID := int64(42)
			rating := 9
			movieID := testutil.CreateTitle(t, db, &model.Title{
				Type:        model.TitleTypeMovie,
				Year:        2024,
				Status:      model.TitleStatusCompleted,
				MatchStatus: model.MatchStatusConfirmed,
				MyRating:    &rating,
				RadarrID:    &radarrID,
				ArrIgnored:  tc.arrIgnored,
			}, []model.TitleName{{Name: "Delete Target", Language: "en", IsPrimary: true}})
			testutil.UpdateTitle(t, db, movieID, repository.TitleUpdate{SetDownloadedAt: true})
			require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
				_, err := repository.NewWatchEventWriter(tx).Create(context.Background(), &model.WatchEvent{
					TitleID: movieID,
					Source:  model.WatchEventSourceManual,
				})
				return err
			}))

			cfg := &config.Config{
				RadarrURL:    arrServer.URL,
				RadarrAPIKey: "radarr-key",
			}
			arrSvc := service.NewArrService(cfg, settingsRepo, titlesRepo, db)
			h := handler.NewArrHandler(arrSvc, titlesRepo, db)

			r := chi.NewRouter()
			r.Delete("/api/arr/title/{id}", httputil.WrapHandler(h.DeleteTitleArr))

			var req *http.Request
			if tc.nilBody {
				req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/arr/title/%d", movieID), nil)
			} else {
				req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/arr/title/%d", movieID), strings.NewReader(tc.body))
			}
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			var res map[string]string
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &res))
			assert.Equal(t, "ok", res["status"])
			assert.Equal(t, 1, deleteCalls)
			assert.Equal(t, tc.expectedQuery, deletedURI)

			after, err := titlesRepo.GetByID(movieID)
			require.NoError(t, err)
			assert.Nil(t, after.RadarrID)
			assert.Nil(t, after.DownloadedAt)
			assert.Equal(t, model.TitleStatusCompleted, after.Status)
			require.NotNil(t, after.MyRating)
			assert.Equal(t, 9, *after.MyRating)
			assert.Equal(t, tc.arrIgnored, after.ArrIgnored)

			evCount, err := watchEventRepo.CountByTitleID(movieID)
			require.NoError(t, err)
			assert.Equal(t, 1, evCount)
		})
	}
}

func TestArrHandler_DeleteTitleArr_FallbackTMDBLookup(t *testing.T) {
	var deletedURI string
	arrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/movie" && r.URL.Query().Get("tmdbId") == "9988":
			_, _ = w.Write([]byte(`[{"id":77,"tmdbId":9988,"title":"Lookup Movie"}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v3/movie/77":
			deletedURI = r.URL.String()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer arrServer.Close()

	db := testutil.NewTestDB(t)
	titlesRepo := repository.NewTitleRepository(db)
	settingsRepo := repository.NewSettingRepository(db)

	tmdbID := int64(9988)
	movieID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		TMDBID:      &tmdbID,
	}, []model.TitleName{{Name: "Lookup Movie", Language: "en", IsPrimary: true}})
	testutil.UpdateTitle(t, db, movieID, repository.TitleUpdate{SetDownloadedAt: true})

	cfg := &config.Config{
		RadarrURL:    arrServer.URL,
		RadarrAPIKey: "radarr-key",
	}
	arrSvc := service.NewArrService(cfg, settingsRepo, titlesRepo, db)
	h := handler.NewArrHandler(arrSvc, titlesRepo, db)

	r := chi.NewRouter()
	r.Delete("/api/arr/title/{id}", httputil.WrapHandler(h.DeleteTitleArr))

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/arr/title/%d", movieID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "/api/v3/movie/77?deleteFiles=true&addImportExclusion=true", deletedURI)

	after, err := titlesRepo.GetByID(movieID)
	require.NoError(t, err)
	assert.Nil(t, after.RadarrID)
	assert.Nil(t, after.DownloadedAt)
}

func TestArrHandler_DeleteTitleArr_Errors(t *testing.T) {
	cases := []struct {
		name           string
		pathID         string // overrides created title ID when non-empty
		titleType      model.TitleType
		noRadarrID     bool
		body           string
		radarrStatus   int
		radarrBody     string
		closeServer    bool
		unconfigured   bool
		expectedStatus int
		expectRadarr   bool
		errContains    string
		exactError     string
	}{
		{
			name:           "invalid ID param returns 400",
			pathID:         "notanumber",
			expectedStatus: http.StatusBadRequest,
			errContains:    "Invalid ID",
		},
		{
			name:           "missing title returns 404",
			pathID:         "99999",
			expectedStatus: http.StatusNotFound,
			errContains:    "not found",
		},
		{
			name:           "series title returns 400",
			titleType:      model.TitleTypeSeries,
			expectedStatus: http.StatusBadRequest,
			errContains:    "movie",
		},
		{
			name:           "malformed JSON body returns 400",
			titleType:      model.TitleTypeMovie,
			body:           `{invalid`,
			expectedStatus: http.StatusBadRequest,
			errContains:    "Invalid JSON",
		},
		{
			name:           "truncated JSON body returns 400",
			titleType:      model.TitleTypeMovie,
			body:           `{"delete_files": `,
			expectedStatus: http.StatusBadRequest,
			errContains:    "Invalid JSON",
		},
		{
			name:           "wrong field type in JSON returns 400",
			titleType:      model.TitleTypeMovie,
			body:           `{"delete_files":"yes"}`,
			expectedStatus: http.StatusBadRequest,
			errContains:    "Invalid JSON",
		},
		{
			name:           "Radarr 500 error returns sanitized 500 and preserves DB",
			titleType:      model.TitleTypeMovie,
			radarrStatus:   http.StatusInternalServerError,
			radarrBody:     `{"message":"radarr error","path":"/nas/movies/secret"}`,
			expectedStatus: http.StatusInternalServerError,
			expectRadarr:   true,
			errContains:    "Failed to delete from Radarr",
			exactError:     "Failed to delete from Radarr",
		},
		{
			name:           "Radarr lookup failure returns sanitized 500 without leaking URL or upstream body",
			titleType:      model.TitleTypeMovie,
			noRadarrID:     true,
			radarrStatus:   http.StatusBadGateway,
			radarrBody:     `{"message":"radarr lookup failure","path":"/nas/movies/secret"}`,
			expectedStatus: http.StatusInternalServerError,
			expectRadarr:   true,
			errContains:    "Failed to delete from Radarr",
			exactError:     "Failed to delete from Radarr",
		},
		{
			name:           "Radarr network error returns sanitized 500 without leaking internal URL",
			titleType:      model.TitleTypeMovie,
			closeServer:    true,
			expectedStatus: http.StatusInternalServerError,
			errContains:    "Failed to delete from Radarr",
			exactError:     "Failed to delete from Radarr",
		},
		{
			name:           "unconfigured Radarr returns sanitized 500 and preserves DB",
			titleType:      model.TitleTypeMovie,
			unconfigured:   true,
			expectedStatus: http.StatusInternalServerError,
			errContains:    "Failed to delete from Radarr",
			exactError:     "Failed to delete from Radarr",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var radarrCalls int
			arrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				radarrCalls++
				status := tc.radarrStatus
				if status == 0 {
					status = http.StatusOK
				}
				body := tc.radarrBody
				if body == "" {
					body = `{"message":"radarr error"}`
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(body))
			}))
			defer arrServer.Close()
			if tc.closeServer {
				arrServer.Close()
			}

			db := testutil.NewTestDB(t)
			titlesRepo := repository.NewTitleRepository(db)
			settingsRepo := repository.NewSettingRepository(db)

			var createdID int64
			arrID := int64(42)
			tmdbID := int64(9988)
			if tc.titleType != "" {
				title := &model.Title{
					Type:        tc.titleType,
					Year:        2024,
					Status:      model.TitleStatusCompleted,
					MatchStatus: model.MatchStatusConfirmed,
					TMDBID:      &tmdbID,
				}
				if tc.titleType == model.TitleTypeMovie {
					if !tc.noRadarrID {
						title.RadarrID = &arrID
					}
				} else {
					title.SonarrID = &arrID
				}
				createdID = testutil.CreateTitle(t, db, title, []model.TitleName{{Name: "Error Title", Language: "en", IsPrimary: true}})
				testutil.UpdateTitle(t, db, createdID, repository.TitleUpdate{SetDownloadedAt: true})
			}

			cfg := &config.Config{}
			if !tc.unconfigured {
				cfg.RadarrURL = arrServer.URL
				cfg.RadarrAPIKey = "radarr-key"
			}
			arrSvc := service.NewArrService(cfg, settingsRepo, titlesRepo, db)
			h := handler.NewArrHandler(arrSvc, titlesRepo, db)

			r := chi.NewRouter()
			r.Delete("/api/arr/title/{id}", httputil.WrapHandler(h.DeleteTitleArr))

			targetID := tc.pathID
			if targetID == "" {
				targetID = fmt.Sprintf("%d", createdID)
			}
			var bodyReader *strings.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}
			var req *http.Request
			if bodyReader != nil {
				req = httptest.NewRequest(http.MethodDelete, "/api/arr/title/"+targetID, bodyReader)
			} else {
				req = httptest.NewRequest(http.MethodDelete, "/api/arr/title/"+targetID, nil)
			}
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			if tc.errContains != "" {
				assert.Contains(t, strings.ToLower(rr.Body.String()), strings.ToLower(tc.errContains))
			}
			if tc.exactError != "" {
				var errResp map[string]string
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
				assert.Equal(t, map[string]string{"error": tc.exactError}, errResp)
				assert.NotContains(t, rr.Body.String(), arrServer.URL)
				assert.NotContains(t, rr.Body.String(), "/nas/movies/secret")
				assert.NotContains(t, rr.Body.String(), "radarr error")
				assert.NotContains(t, rr.Body.String(), "not configured")
			}
			if tc.expectRadarr {
				assert.Equal(t, 1, radarrCalls)
			} else {
				assert.Equal(t, 0, radarrCalls)
			}

			if createdID > 0 {
				after, err := titlesRepo.GetByID(createdID)
				require.NoError(t, err)
				assert.NotNil(t, after.DownloadedAt)
				if tc.titleType == model.TitleTypeMovie {
					if tc.noRadarrID {
						assert.Nil(t, after.RadarrID)
					} else {
						require.NotNil(t, after.RadarrID)
						assert.Equal(t, int64(42), *after.RadarrID)
					}
				} else {
					require.NotNil(t, after.SonarrID)
					assert.Equal(t, int64(42), *after.SonarrID)
				}
			}
		})
	}
}

func TestArrHandler_DeleteTitleArr_RouterScope(t *testing.T) {
	const testJWTSecret = "router-test-jwt-secret"

	cases := []struct {
		name           string
		authMode       string // "none", "bearer", "x_api_key", "cookie"
		scopes         []string
		expectedStatus int
		expectDeleted  bool
	}{
		{
			name:           "unauthenticated returns 401",
			authMode:       "none",
			expectedStatus: http.StatusUnauthorized,
			expectDeleted:  false,
		},
		{
			name:           "API key with arr:read only returns 403",
			authMode:       "bearer",
			scopes:         []string{model.ScopeArrRead, model.ScopeLibraryWrite},
			expectedStatus: http.StatusForbidden,
			expectDeleted:  false,
		},
		{
			name:           "API key with arr:write via Bearer succeeds",
			authMode:       "bearer",
			scopes:         []string{model.ScopeArrWrite},
			expectedStatus: http.StatusOK,
			expectDeleted:  true,
		},
		{
			name:           "API key with arr:write via X-Api-Key succeeds",
			authMode:       "x_api_key",
			scopes:         []string{model.ScopeArrWrite},
			expectedStatus: http.StatusOK,
			expectDeleted:  true,
		},
		{
			name:           "interactive session JWT cookie succeeds",
			authMode:       "cookie",
			expectedStatus: http.StatusOK,
			expectDeleted:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var deleteCalls int
			arrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete && r.URL.Path == "/api/v3/movie/42" {
					deleteCalls++
					w.WriteHeader(http.StatusOK)
					return
				}
				http.NotFound(w, r)
			}))
			defer arrServer.Close()

			db := testutil.NewTestDB(t)
			t.Cleanup(func() { database.SetInvalidator(nil) })

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			cfg := &config.Config{
				JWTSecret:    testJWTSecret,
				RadarrURL:    arrServer.URL,
				RadarrAPIKey: "radarr-key",
			}

			mux := router.New(ctx, cfg, db, db, embed.FS{}, nil, nil, nil)

			radarrID := int64(42)
			movieID := testutil.CreateTitle(t, db, &model.Title{
				Type:        model.TitleTypeMovie,
				Year:        2024,
				Status:      model.TitleStatusCompleted,
				MatchStatus: model.MatchStatusConfirmed,
				RadarrID:    &radarrID,
			}, []model.TitleName{{Name: "Scoped Movie", Language: "en", IsPrimary: true}})
			testutil.UpdateTitle(t, db, movieID, repository.TitleUpdate{SetDownloadedAt: true})

			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/arr/title/%d", movieID), strings.NewReader(`{"add_import_exclusion":true}`))

			switch tc.authMode {
			case "bearer", "x_api_key":
				apiKeySvc := service.NewAPIKeyService(db, repository.NewAPIKeyRepository(db))
				_, rawToken, err := apiKeySvc.CreateKey(ctx, "Test Key", tc.scopes)
				require.NoError(t, err)
				if tc.authMode == "bearer" {
					req.Header.Set("Authorization", "Bearer "+rawToken)
				} else {
					req.Header.Set("X-Api-Key", rawToken)
				}
			case "cookie":
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"email": "admin@example.com"})
				signed, err := token.SignedString([]byte(testJWTSecret))
				require.NoError(t, err)
				req.AddCookie(&http.Cookie{Name: "token", Value: signed})
			}

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)

			titlesRepo := repository.NewTitleRepository(db)
			after, err := titlesRepo.GetByID(movieID)
			require.NoError(t, err)

			if tc.expectDeleted {
				assert.Equal(t, 1, deleteCalls)
				assert.Nil(t, after.RadarrID)
				assert.Nil(t, after.DownloadedAt)
			} else {
				assert.Equal(t, 0, deleteCalls)
				require.NotNil(t, after.RadarrID)
				assert.Equal(t, int64(42), *after.RadarrID)
				assert.NotNil(t, after.DownloadedAt)
				if tc.expectedStatus == http.StatusForbidden {
					assert.Contains(t, rr.Body.String(), "insufficient_scope")
					assert.Contains(t, rr.Body.String(), model.ScopeArrWrite)
				}
			}
		})
	}
}
