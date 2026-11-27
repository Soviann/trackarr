package router_test

import (
	"context"
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/router"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRouterJWTSecret = "test-router-jwt-secret-0123456789abcdef"

func signRouterTestJWT(t *testing.T, secret string, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": jwt.NewNumericDate(exp),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

func TestAdminJellyfinSyncMoviesRoute_AuthAndBehavior(t *testing.T) {
	t.Run("rejects unauthenticated, API-key-only, and invalid JWT requests with 401", func(t *testing.T) {
		db := testutil.NewTestDB(t)
		t.Cleanup(func() { database.SetInvalidator(nil) })

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		cfg := &config.Config{JWTSecret: testRouterJWTSecret}
		mux := router.New(ctx, cfg, db, db, embed.FS{}, nil, nil, nil)

		apiKeySvc := service.NewAPIKeyService(db, repository.NewAPIKeyRepository(db))
		_, rawToken, err := apiKeySvc.CreateKey(ctx, "All Scopes Key", []string{
			model.ScopeLibraryRead,
			model.ScopeLibraryWrite,
			model.ScopeLibraryDelete,
			model.ScopeArrRead,
			model.ScopeArrWrite,
		})
		require.NoError(t, err)

		cases := []struct {
			name  string
			setup func(*http.Request)
		}{
			{
				name:  "unauthenticated",
				setup: func(r *http.Request) {},
			},
			{
				name: "API key in Authorization Bearer header",
				setup: func(r *http.Request) {
					r.Header.Set("Authorization", "Bearer "+rawToken)
				},
			},
			{
				name: "API key in X-Api-Key header",
				setup: func(r *http.Request) {
					r.Header.Set("X-Api-Key", rawToken)
				},
			},
			{
				name: "expired JWT cookie",
				setup: func(r *http.Request) {
					expired := signRouterTestJWT(t, testRouterJWTSecret, time.Now().Add(-time.Hour))
					r.AddCookie(&http.Cookie{Name: "token", Value: expired})
				},
			},
			{
				name: "wrong-secret JWT cookie",
				setup: func(r *http.Request) {
					wrong := signRouterTestJWT(t, "wrong-secret-0123456789abcdef0123456789", time.Now().Add(time.Hour))
					r.AddCookie(&http.Cookie{Name: "token", Value: wrong})
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", nil)
				tc.setup(req)
				rr := httptest.NewRecorder()

				mux.ServeHTTP(rr, req)
				assert.Equal(t, http.StatusUnauthorized, rr.Code)
			})
		}
	})

	t.Run("returns 400 when Jellyfin API settings are missing", func(t *testing.T) {
		db := testutil.NewTestDB(t)
		t.Cleanup(func() { database.SetInvalidator(nil) })

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		cfg := &config.Config{JWTSecret: testRouterJWTSecret}
		mux := router.New(ctx, cfg, db, db, embed.FS{}, nil, nil, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", nil)
		req.AddCookie(&http.Cookie{Name: "token", Value: signRouterTestJWT(t, testRouterJWTSecret, time.Now().Add(time.Hour))})
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 200 JSON summary with valid JWT and configured mock Jellyfin server", func(t *testing.T) {
		db := testutil.NewTestDB(t)
		t.Cleanup(func() { database.SetInvalidator(nil) })

		jfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Emby-Token") != "router-jf-api-key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{
						"Id":             "jf-router-1",
						"Name":           "Arrival",
						"Type":           "Movie",
						"ProductionYear": 2016,
						"ProviderIds":    map[string]string{"Tmdb": "329865", "Imdb": "tt2543164"},
						"UserData":       map[string]any{"Played": false},
					},
					{
						"Id":             "jf-router-2",
						"Name":           "Blade Runner 2049",
						"Type":           "Movie",
						"ProductionYear": 2017,
						"ProviderIds":    map[string]string{"Tmdb": "335984"},
						"UserData":       map[string]any{"Played": true},
					},
				},
			})
		}))
		defer jfServer.Close()

		testutil.SetSetting(t, db, repository.SettingKeyJellyfinURL, jfServer.URL)
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinAPIKey, "router-jf-api-key")
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinUserID, "user-router-1")

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		cfg := &config.Config{JWTSecret: testRouterJWTSecret}
		mux := router.New(ctx, cfg, db, db, embed.FS{}, nil, nil, nil)
		jwtCookie := &http.Cookie{Name: "token", Value: signRouterTestJWT(t, testRouterJWTSecret, time.Now().Add(time.Hour))}

		// Dry run first
		dryReq := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", strings.NewReader(`{"dry_run":true}`))
		dryReq.AddCookie(jwtCookie)
		dryRR := httptest.NewRecorder()
		mux.ServeHTTP(dryRR, dryReq)

		require.Equal(t, http.StatusOK, dryRR.Code)
		var dryPayload map[string]any
		require.NoError(t, json.NewDecoder(dryRR.Body).Decode(&dryPayload))
		assert.Equal(t, true, dryPayload["dry_run"])
		assert.Equal(t, float64(2), dryPayload["fetched"])
		assert.Equal(t, float64(1), dryPayload["unwatched"])
		assert.Equal(t, float64(1), dryPayload["created"])
		assert.Equal(t, float64(0), dryPayload["skipped"])
		assert.Equal(t, float64(0), dryPayload["errors"])

		// Live sync
		req := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", nil)
		req.AddCookie(jwtCookie)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		var payload map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&payload))
		assert.Equal(t, false, payload["dry_run"])
		assert.Equal(t, float64(2), payload["fetched"])
		assert.Equal(t, float64(1), payload["unwatched"])
		assert.Equal(t, float64(1), payload["created"])
		assert.Equal(t, float64(0), payload["skipped"])
		assert.Equal(t, float64(0), payload["errors"])

		titles, err := repository.NewTitleRepository(db).ListAll()
		require.NoError(t, err)
		require.Len(t, titles, 1)
		assert.Equal(t, model.TitleStatusPlanToWatch, titles[0].Status)
		assert.NotNil(t, titles[0].DownloadedAt)

		events, err := repository.NewWatchEventRepository(db).CountByTitleID(titles[0].ID)
		require.NoError(t, err)
		assert.Equal(t, 0, events)
	})

	t.Run("returns 500 without leaking API key on upstream Jellyfin error", func(t *testing.T) {
		db := testutil.NewTestDB(t)
		t.Cleanup(func() { database.SetInvalidator(nil) })

		const secretToken = "sensitive-jellyfin-token-do-not-leak"
		jfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal failure echoing token " + r.Header.Get("X-Emby-Token")))
		}))
		defer jfServer.Close()

		testutil.SetSetting(t, db, repository.SettingKeyJellyfinURL, jfServer.URL)
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinAPIKey, secretToken)
		testutil.SetSetting(t, db, repository.SettingKeyJellyfinUserID, "user-router-1")

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		cfg := &config.Config{JWTSecret: testRouterJWTSecret}
		mux := router.New(ctx, cfg, db, db, embed.FS{}, nil, nil, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/jellyfin/sync-movies", nil)
		req.AddCookie(&http.Cookie{Name: "token", Value: signRouterTestJWT(t, testRouterJWTSecret, time.Now().Add(time.Hour))})
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		body := rr.Body.String()
		assert.Contains(t, body, "Failed to sync movies from Jellyfin")
		assert.NotContains(t, body, secretToken)
		assert.NotContains(t, body, jfServer.URL)
	})
}

func TestPeopleAndPersonRoleRoutes_AuthAndBehavior(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { database.SetInvalidator(nil) })

	credits := `[{"name":"Denis Villeneuve","role":"Director"},{"name":"Hans Zimmer","role":"Composer"}]`
	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2021,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		Credits:     &credits,
	}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := &config.Config{JWTSecret: testRouterJWTSecret}
	mux := router.New(ctx, cfg, db, db, embed.FS{}, nil, nil, nil)

	apiKeySvc := service.NewAPIKeyService(db, repository.NewAPIKeyRepository(db))
	_, readToken, err := apiKeySvc.CreateKey(ctx, "Library Read Key", []string{model.ScopeLibraryRead})
	require.NoError(t, err)
	_, arrOnlyToken, err := apiKeySvc.CreateKey(ctx, "Arr Only Key", []string{model.ScopeArrRead})
	require.NoError(t, err)

	// Unauthenticated -> 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/people?search=Zimmer", nil)
	rrUnauth := httptest.NewRecorder()
	mux.ServeHTTP(rrUnauth, reqUnauth)
	assert.Equal(t, http.StatusUnauthorized, rrUnauth.Code)

	// Wrong scope -> 403
	reqForbidden := httptest.NewRequest(http.MethodGet, "/api/people?search=Zimmer", nil)
	reqForbidden.Header.Set("Authorization", "Bearer "+arrOnlyToken)
	rrForbidden := httptest.NewRecorder()
	mux.ServeHTTP(rrForbidden, reqForbidden)
	assert.Equal(t, http.StatusForbidden, rrForbidden.Code)

	// Authorized via ScopeLibraryRead -> 200
	reqOK := httptest.NewRequest(http.MethodGet, "/api/people?search=Zimmer", nil)
	reqOK.Header.Set("Authorization", "Bearer "+readToken)
	rrOK := httptest.NewRecorder()
	mux.ServeHTTP(rrOK, reqOK)
	require.Equal(t, http.StatusOK, rrOK.Code)
	var people []model.PersonSearchResult
	require.NoError(t, json.NewDecoder(rrOK.Body).Decode(&people))
	require.Len(t, people, 1)
	assert.Equal(t, "Hans Zimmer", people[0].Name)
	assert.Equal(t, []string{"composer"}, people[0].Roles)
	assert.Equal(t, 1, people[0].TitleCount)

	// Cache invalidation through router after TitleWriter commit
	c2 := `[{"name":"Hans Zimmer","role":"Composer"}]`
	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2010,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		Credits:     &c2,
	}, []model.TitleName{{Name: "Inception", Language: "en", IsPrimary: true}})

	reqAfter := httptest.NewRequest(http.MethodGet, "/api/people?search=Zimmer", nil)
	reqAfter.Header.Set("Authorization", "Bearer "+readToken)
	rrAfter := httptest.NewRecorder()
	mux.ServeHTTP(rrAfter, reqAfter)
	require.Equal(t, http.StatusOK, rrAfter.Code)
	var peopleAfter []model.PersonSearchResult
	require.NoError(t, json.NewDecoder(rrAfter.Body).Decode(&peopleAfter))
	require.Len(t, peopleAfter, 1)
	assert.Equal(t, 2, peopleAfter[0].TitleCount)

	// GET /api/titles?person=...&role=... valid and invalid
	jwtCookie := &http.Cookie{Name: "token", Value: signRouterTestJWT(t, testRouterJWTSecret, time.Now().Add(time.Hour))}
	reqTitles := httptest.NewRequest(http.MethodGet, "/api/titles?person=Denis+Villeneuve&role=director", nil)
	reqTitles.AddCookie(jwtCookie)
	rrTitles := httptest.NewRecorder()
	mux.ServeHTTP(rrTitles, reqTitles)
	require.Equal(t, http.StatusOK, rrTitles.Code)
	var titlesRes repository.PaginatedResult
	require.NoError(t, json.NewDecoder(rrTitles.Body).Decode(&titlesRes))
	require.Len(t, titlesRes.Titles, 1)
	assert.Equal(t, "Dune", titlesRes.Titles[0].PrimaryName())

	reqBadRole := httptest.NewRequest(http.MethodGet, "/api/titles?person=Denis+Villeneuve&role=invalid", nil)
	reqBadRole.AddCookie(jwtCookie)
	rrBadRole := httptest.NewRecorder()
	mux.ServeHTTP(rrBadRole, reqBadRole)
	assert.Equal(t, http.StatusBadRequest, rrBadRole.Code)
}
