package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })
	return db
}

func ptr[T any](v T) *T {
	return &v
}

func TestJellyfinSyncService_FetchUserItems(t *testing.T) {
	t.Run("successfully queries items and verifies headers", func(t *testing.T) {
		var receivedToken string
		var receivedPath string
		var receivedQuery string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedToken = r.Header.Get("X-Emby-Token")
			receivedPath = r.URL.Path
			receivedQuery = r.URL.RawQuery

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{
						"Id":   "jf-movie-1",
						"Name": "Inception",
						"Type": "Movie",
						"ProviderIds": map[string]string{
							"Tmdb": "27205",
							"Imdb": "tt1375666",
						},
					},
				},
				"TotalRecordCount": 1,
			})
		}))
		defer server.Close()

		cfg := service.JellyfinSyncConfig{
			BaseURL:    server.URL,
			APIKey:     "test-api-key",
			UserID:     "user-123",
			HTTPClient: server.Client(),
		}
		svc := service.NewJellyfinSyncService(nil, cfg, testutil.NopLogger())

		items, err := svc.FetchUserItems(context.Background())
		require.NoError(t, err)
		require.Len(t, items, 1)

		assert.Equal(t, "test-api-key", receivedToken)
		assert.Equal(t, "/Users/user-123/Items", receivedPath)
		assert.Equal(t, "includeItemTypes=Movie,Series&recursive=true&fields=ProviderIds", receivedQuery)
		assert.Equal(t, "jf-movie-1", items[0].ID)
		assert.Equal(t, "Inception", items[0].Name)
		assert.Equal(t, "Movie", items[0].Type)
		assert.Equal(t, "27205", items[0].ProviderIds["Tmdb"])
	})

	t.Run("returns error on HTTP 401 unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Invalid authentication token"))
		}))
		defer server.Close()

		cfg := service.JellyfinSyncConfig{
			BaseURL:    server.URL,
			APIKey:     "bad-token",
			UserID:     "user-123",
			HTTPClient: server.Client(),
		}
		svc := service.NewJellyfinSyncService(nil, cfg, testutil.NopLogger())

		items, err := svc.FetchUserItems(context.Background())
		require.Error(t, err)
		assert.Nil(t, items)
		assert.Contains(t, err.Error(), "status 401")
	})
}

func TestJellyfinSyncService_MatchItem(t *testing.T) {
	svc := service.NewJellyfinSyncService(nil, service.JellyfinSyncConfig{}, testutil.NopLogger())

	items := []service.JellyfinLibraryItem{
		{
			ID:   "movie-tmdb",
			Name: "Interstellar",
			Type: "Movie",
			ProviderIds: map[string]string{
				"Tmdb": "157336",
				"Imdb": "tt0816692",
			},
		},
		{
			ID:   "series-tvdb",
			Name: "Breaking Bad",
			Type: "Series",
			ProviderIds: map[string]string{
				"Tvdb": "81189",
			},
		},
		{
			ID:   "movie-imdb-only",
			Name: "Classic Film",
			Type: "Movie",
			ProviderIds: map[string]string{
				"Imdb": "tt0050083",
			},
		},
	}

	t.Run("matches movie by TMDB ID", func(t *testing.T) {
		title := model.Title{
			Type:   model.TitleTypeMovie,
			TMDBID: ptr(int64(157336)),
		}
		matched := svc.MatchItem(title, items)
		require.NotNil(t, matched)
		assert.Equal(t, "movie-tmdb", matched.ID)
	})

	t.Run("matches series by TVDB ID", func(t *testing.T) {
		title := model.Title{
			Type:   model.TitleTypeSeries,
			TVDBID: ptr(int64(81189)),
		}
		matched := svc.MatchItem(title, items)
		require.NotNil(t, matched)
		assert.Equal(t, "series-tvdb", matched.ID)
	})

	t.Run("matches movie by IMDb ID with case and prefix tolerance", func(t *testing.T) {
		title := model.Title{
			Type:   model.TitleTypeMovie,
			IMDBID: ptr("TT0050083"),
		}
		matched := svc.MatchItem(title, items)
		require.NotNil(t, matched)
		assert.Equal(t, "movie-imdb-only", matched.ID)
	})

	t.Run("fails to match if media type differs", func(t *testing.T) {
		title := model.Title{
			Type:   model.TitleTypeSeries, // Should not match movie-tmdb
			TMDBID: ptr(int64(157336)),
		}
		matched := svc.MatchItem(title, items)
		assert.Nil(t, matched)
	})

	t.Run("returns nil when no provider ID matches", func(t *testing.T) {
		title := model.Title{
			Type:   model.TitleTypeMovie,
			TMDBID: ptr(int64(999999)),
		}
		matched := svc.MatchItem(title, items)
		assert.Nil(t, matched)
	})
}

func TestJellyfinSyncService_PushRating(t *testing.T) {
	t.Run("pushes rating >= 6 with likes=true", func(t *testing.T) {
		var receivedPath string
		var receivedRating string
		var receivedLikes string
		var receivedToken string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			receivedRating = r.URL.Query().Get("rating")
			receivedLikes = r.URL.Query().Get("likes")
			receivedToken = r.Header.Get("X-Emby-Token")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := service.JellyfinSyncConfig{
			BaseURL:    server.URL,
			APIKey:     "secret-token",
			UserID:     "user-456",
			HTTPClient: server.Client(),
		}
		svc := service.NewJellyfinSyncService(nil, cfg, testutil.NopLogger())

		err := svc.PushRating(context.Background(), "item-789", 8)
		require.NoError(t, err)

		assert.Equal(t, "/Users/user-456/Items/item-789/Rating", receivedPath)
		assert.Equal(t, "8", receivedRating)
		assert.Equal(t, "true", receivedLikes)
		assert.Equal(t, "secret-token", receivedToken)
	})

	t.Run("pushes rating < 6 with likes=false", func(t *testing.T) {
		var receivedLikes string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedLikes = r.URL.Query().Get("likes")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := service.JellyfinSyncConfig{
			BaseURL:    server.URL,
			APIKey:     "token",
			UserID:     "user-1",
			HTTPClient: server.Client(),
		}
		svc := service.NewJellyfinSyncService(nil, cfg, testutil.NopLogger())

		err := svc.PushRating(context.Background(), "item-1", 4)
		require.NoError(t, err)
		assert.Equal(t, "false", receivedLikes)
	})

	t.Run("returns error on server failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("database lock"))
		}))
		defer server.Close()

		cfg := service.JellyfinSyncConfig{
			BaseURL:    server.URL,
			APIKey:     "token",
			UserID:     "user-1",
			HTTPClient: server.Client(),
		}
		svc := service.NewJellyfinSyncService(nil, cfg, testutil.NopLogger())

		err := svc.PushRating(context.Background(), "item-1", 7)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 500")
	})
}

func TestJellyfinSyncService_SyncRatings(t *testing.T) {
	t.Run("validates missing configuration", func(t *testing.T) {
		svc := service.NewJellyfinSyncService(nil, service.JellyfinSyncConfig{}, testutil.NopLogger())
		summary, err := svc.SyncRatings(context.Background())
		require.Error(t, err)
		assert.Equal(t, 0, summary.Matched)
		assert.Contains(t, err.Error(), "missing required configuration")
	})

	t.Run("dry run mode previews updates without executing POST", func(t *testing.T) {
		db := setupTestDB(t)

		rating9 := 9
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2010,
			Status:      model.TitleStatusCompleted,
			MatchStatus: model.MatchStatusConfirmed,
			MyRating:    &rating9,
			TMDBID:      ptr(int64(27205)),
		}, []model.TitleName{{Name: "Inception", Language: "en", IsPrimary: true}})

		var postCallCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{
							"Id":   "jf-inception",
							"Name": "Inception",
							"Type": "Movie",
							"ProviderIds": map[string]string{
								"Tmdb": "27205",
							},
						},
					},
					"TotalRecordCount": 1,
				})
				return
			}
			if r.Method == http.MethodPost {
				atomic.AddInt32(&postCallCount, 1)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		cfg := service.JellyfinSyncConfig{
			BaseURL:       server.URL,
			APIKey:        "test-token",
			UserID:        "user-abc",
			DryRun:        true,
			HTTPClient:    server.Client(),
			ThrottleDelay: -1, // disable throttling
		}
		svc := service.NewJellyfinSyncService(db, cfg, testutil.NopLogger())

		summary, err := svc.SyncRatings(context.Background())
		require.NoError(t, err)

		assert.Equal(t, 1, summary.Matched)
		assert.Equal(t, 1, summary.Skipped)
		assert.Equal(t, 0, summary.Updated)
		assert.Equal(t, 0, summary.Missing)
		assert.Equal(t, 0, summary.Errors)
		assert.Equal(t, int32(0), atomic.LoadInt32(&postCallCount), "No POST should be performed in dry-run mode")
	})

	t.Run("live sync mode updates ratings, skips identical ratings, and tracks missing items", func(t *testing.T) {
		db := setupTestDB(t)

		rating9 := 9
		rating8 := 8
		rating7 := 7

		// Title 1: matched and needs update
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2014,
			Status:      model.TitleStatusCompleted,
			MatchStatus: model.MatchStatusConfirmed,
			MyRating:    &rating9,
			TMDBID:      ptr(int64(157336)),
		}, []model.TitleName{{Name: "Interstellar", Language: "en", IsPrimary: true}})

		// Title 2: matched and already has identical rating on Jellyfin
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeSeries,
			Year:        2008,
			Status:      model.TitleStatusCompleted,
			MatchStatus: model.MatchStatusConfirmed,
			MyRating:    &rating8,
			TVDBID:      ptr(int64(81189)),
		}, []model.TitleName{{Name: "Breaking Bad", Language: "en", IsPrimary: true}})

		// Title 3: rated in Trackarr but missing in Jellyfin
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2023,
			Status:      model.TitleStatusCompleted,
			MatchStatus: model.MatchStatusConfirmed,
			MyRating:    &rating7,
			TMDBID:      ptr(int64(872585)),
		}, []model.TitleName{{Name: "Oppenheimer", Language: "en", IsPrimary: true}})

		var postedItemIDs []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{
							"Id":   "jf-interstellar",
							"Name": "Interstellar",
							"Type": "Movie",
							"ProviderIds": map[string]string{
								"Tmdb": "157336",
							},
							"UserData": map[string]any{
								"Rating": 6.0, // Different rating
							},
						},
						{
							"Id":   "jf-bb",
							"Name": "Breaking Bad",
							"Type": "Series",
							"ProviderIds": map[string]string{
								"Tvdb": "81189",
							},
							"UserData": map[string]any{
								"Rating": 8.0, // Same rating -> skipped
							},
						},
					},
					"TotalRecordCount": 2,
				})
				return
			}
			if r.Method == http.MethodPost {
				postedItemIDs = append(postedItemIDs, r.URL.Path)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		var sleptDurations []time.Duration
		cfg := service.JellyfinSyncConfig{
			BaseURL:       server.URL,
			APIKey:        "valid-token",
			UserID:        "user-xyz",
			DryRun:        false,
			HTTPClient:    server.Client(),
			ThrottleDelay: 10 * time.Millisecond,
		}
		svc := service.NewJellyfinSyncService(db, cfg, testutil.NopLogger())
		svc.SetSleepFn(func(d time.Duration) {
			sleptDurations = append(sleptDurations, d)
		})

		summary, err := svc.SyncRatings(context.Background())
		require.NoError(t, err)

		assert.Equal(t, 2, summary.Matched)
		assert.Equal(t, 1, summary.Updated)
		assert.Equal(t, 1, summary.Skipped)
		assert.Equal(t, 1, summary.Missing)
		assert.Equal(t, 0, summary.Errors)

		require.Len(t, postedItemIDs, 1)
		assert.Equal(t, "/Users/user-xyz/Items/jf-interstellar/Rating", postedItemIDs[0])
		assert.Equal(t, []time.Duration{10 * time.Millisecond}, sleptDurations)
	})

	t.Run("handles post rating HTTP errors gracefully", func(t *testing.T) {
		db := setupTestDB(t)

		rating9 := 9
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2014,
			Status:      model.TitleStatusCompleted,
			MatchStatus: model.MatchStatusConfirmed,
			MyRating:    &rating9,
			TMDBID:      ptr(int64(157336)),
		}, []model.TitleName{{Name: "Interstellar", Language: "en", IsPrimary: true}})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Items": []map[string]any{
						{
							"Id":   "jf-interstellar",
							"Name": "Interstellar",
							"Type": "Movie",
							"ProviderIds": map[string]string{
								"Tmdb": "157336",
							},
						},
					},
					"TotalRecordCount": 1,
				})
				return
			}
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal database error"))
			}
		}))
		defer server.Close()

		cfg := service.JellyfinSyncConfig{
			BaseURL:       server.URL,
			APIKey:        "valid-token",
			UserID:        "user-xyz",
			DryRun:        false,
			HTTPClient:    server.Client(),
			ThrottleDelay: -1,
		}
		svc := service.NewJellyfinSyncService(db, cfg, testutil.NopLogger())

		summary, err := svc.SyncRatings(context.Background())
		require.NoError(t, err) // Overall process does not fail on per-item error

		assert.Equal(t, 1, summary.Matched)
		assert.Equal(t, 0, summary.Updated)
		assert.Equal(t, 0, summary.Skipped)
		assert.Equal(t, 0, summary.Missing)
		assert.Equal(t, 1, summary.Errors)
	})
}
