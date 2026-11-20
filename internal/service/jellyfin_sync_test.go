package service_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/service/matching"
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

func TestJellyfinSyncService_TestConnectionAndSingleTitle(t *testing.T) {
	db := setupTestDB(t)
	rating8 := 8
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2014, Status: model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed, MyRating: &rating8, TMDBID: ptr(int64(157336)),
	}, []model.TitleName{{Name: "Interstellar", Language: "en", IsPrimary: true}})

	var pushedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "limit=1") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Items":[]}`))
			return
		}
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{{
					"Id":          "jf-1",
					"Name":        "Interstellar",
					"Type":        "Movie",
					"ProviderIds": map[string]string{"Tmdb": "157336"},
				}},
			})
			return
		}
		if r.Method == http.MethodPost {
			pushedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	svc := service.NewJellyfinSyncService(db, service.JellyfinSyncConfig{
		BaseURL: server.URL, APIKey: "tok", UserID: "u1", HTTPClient: server.Client(), ThrottleDelay: -1,
	}, testutil.NopLogger())

	require.NoError(t, svc.TestConnection(context.Background()))
	require.NoError(t, svc.SyncSingleTitle(context.Background(), titleID))
	assert.Equal(t, "/Users/u1/Items/jf-1/Rating", pushedPath)
}

func TestTitleService_Update_EnqueuesJellyfinRatingPush(t *testing.T) {
	db := setupTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	titleSvc := service.NewTitleService(db, titleRepo, taskRepo, nil)

	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Test Movie", Language: "en", IsPrimary: true}})

	before, err := titleRepo.GetByID(titleID)
	require.NoError(t, err)

	// 1. Not configured -> does NOT enqueue
	r7 := 7
	require.NoError(t, titleSvc.Update(context.Background(), titleID, repository.TitleUpdate{MyRating: &r7}, before, nil, &r7, false))
	tasks, err := taskRepo.ListPending()
	require.NoError(t, err)
	assert.Empty(t, tasks)

	// 2. Configured -> enqueues TaskTypeJellyfinRatingPush on rating change
	tx, err := db.Begin()
	require.NoError(t, err)
	sw := repository.NewSettingWriter(tx)
	require.NoError(t, sw.Set(context.Background(), "jellyfin_url", "http://localhost:8096"))
	require.NoError(t, sw.Set(context.Background(), "jellyfin_api_key", "key"))
	require.NoError(t, sw.Set(context.Background(), "jellyfin_user_id", "user1"))
	require.NoError(t, tx.Commit())

	before, err = titleRepo.GetByID(titleID)
	require.NoError(t, err)
	r9 := 9
	require.NoError(t, titleSvc.Update(context.Background(), titleID, repository.TitleUpdate{MyRating: &r9}, before, nil, &r9, false))
	tasks, err = taskRepo.ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskTypeJellyfinRatingPush, tasks[0].TaskType)
}

func TestJellyfinSyncService_FetchUserMovies(t *testing.T) {
	t.Run("queries Movie items and decodes ProductionYear and UserData.Played", func(t *testing.T) {
		var receivedToken, receivedAccept, receivedPath, receivedQuery string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedToken = r.Header.Get("X-Emby-Token")
			receivedAccept = r.Header.Get("Accept")
			receivedPath = r.URL.Path
			receivedQuery = r.URL.RawQuery

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{
						"Id":             "jf-m1",
						"Name":           "Inception",
						"Type":           "Movie",
						"ProductionYear": 2010,
						"ProviderIds":    map[string]string{"Tmdb": "27205", "Imdb": "tt1375666"},
						"UserData":       map[string]any{"Played": false},
					},
				},
				"TotalRecordCount": 1,
			})
		}))
		defer server.Close()

		svc := service.NewJellyfinSyncService(nil, service.JellyfinSyncConfig{
			BaseURL:    server.URL,
			APIKey:     "secret-api-key",
			UserID:     "user-99",
			HTTPClient: server.Client(),
		}, testutil.NopLogger())

		items, err := svc.FetchUserMovies(context.Background())
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "secret-api-key", receivedToken)
		assert.Equal(t, "application/json", receivedAccept)
		assert.Equal(t, "/Users/user-99/Items", receivedPath)
		assert.Equal(t, "includeItemTypes=Movie&recursive=true&fields=ProviderIds", receivedQuery)
		assert.Equal(t, 2010, items[0].ProductionYear)
		require.NotNil(t, items[0].UserData)
		assert.False(t, items[0].UserData.Played)
	})

	t.Run("validates baseURL scheme and host", func(t *testing.T) {
		for _, badURL := range []string{"", "   ", "/relative", "ftp://jellyfin.local:8096", "http://", "://broken"} {
			svc := service.NewJellyfinSyncService(nil, service.JellyfinSyncConfig{
				BaseURL: badURL,
				APIKey:  "api-key",
				UserID:  "user-1",
			}, testutil.NopLogger())

			items, err := svc.FetchUserMovies(context.Background())
			require.Error(t, err, "expected error for URL %q", badURL)
			assert.Nil(t, items)
		}
	})
}

func TestJellyfinSyncService_SyncUnwatchedMovies_CreatesAndIdempotent(t *testing.T) {
	db := setupTestDB(t)

	var reqCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		assert.Equal(t, "jf-token", r.Header.Get("X-Emby-Token"))
		assert.Equal(t, "/Users/u-42/Items", r.URL.Path)
		assert.Equal(t, "includeItemTypes=Movie&recursive=true&fields=ProviderIds", r.URL.RawQuery)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{
					"Id":             "jf-unwatched-1",
					"Name":           "  The Matrix  ",
					"Type":           "Movie",
					"ProductionYear": 1999,
					"ProviderIds": map[string]string{
						"tmdb": "603",
						"IMDB": "  TT0133093  ",
						"tvdb": "155",
					},
					"UserData": map[string]any{"Played": false},
				},
				{
					"Id":             "jf-unwatched-2",
					"Name":           "Obscure Indie Film",
					"Type":           "movie",
					"ProductionYear": 2250, // out of bounds -> clamped to 0
					"ProviderIds": map[string]string{
						"Imdb": "tt9990001",
						"Tmdb": "-5", // non-positive ignored
					},
					// UserData omitted -> nil -> unwatched
				},
				{
					"Id":             "jf-watched-3",
					"Name":           "Already Watched",
					"Type":           "Movie",
					"ProductionYear": 2020,
					"ProviderIds":    map[string]string{"Tmdb": "55555"},
					"UserData":       map[string]any{"Played": true},
				},
				{
					"Id":             "jf-series-4",
					"Name":           "Some Series",
					"Type":           "Series",
					"ProductionYear": 2021,
					"ProviderIds":    map[string]string{"Tmdb": "77777"},
					"UserData":       map[string]any{"Played": false},
				},
			},
			"TotalRecordCount": 4,
		})
	}))
	defer server.Close()

	svc := service.NewJellyfinSyncService(db, service.JellyfinSyncConfig{
		BaseURL:    server.URL,
		APIKey:     "jf-token",
		UserID:     "u-42",
		HTTPClient: server.Client(),
	}, testutil.NopLogger())

	summary, err := svc.SyncUnwatchedMovies(context.Background())
	require.NoError(t, err)
	assert.Equal(t, service.JellyfinMovieSyncSummary{
		DryRun:    false,
		Fetched:   4,
		Unwatched: 2,
		Created:   2,
		Skipped:   0,
		Errors:    0,
	}, summary)

	titleRepo := repository.NewTitleRepository(db)
	titles, err := titleRepo.ListAll()
	require.NoError(t, err)
	require.Len(t, titles, 2)

	byExtID := map[string]model.Title{}
	for _, title := range titles {
		require.NotNil(t, title.ExternalSourceID)
		byExtID[*title.ExternalSourceID] = title
	}

	m1, ok := byExtID["jf-unwatched-1"]
	require.True(t, ok)
	assert.Equal(t, model.TitleTypeMovie, m1.Type)
	assert.Equal(t, 1999, m1.Year)
	assert.Equal(t, model.TitleStatusPlanToWatch, m1.Status)
	assert.Equal(t, model.MatchStatusConfirmed, m1.MatchStatus)
	require.NotNil(t, m1.MatchSource)
	assert.Equal(t, "jellyfin_sync", *m1.MatchSource)
	require.NotNil(t, m1.OriginalTitle)
	assert.Equal(t, "The Matrix", *m1.OriginalTitle)
	assert.Equal(t, "The Matrix", m1.PrimaryName())
	assert.True(t, m1.ArrIgnored)
	assert.NotNil(t, m1.DownloadedAt)
	require.NotNil(t, m1.TMDBID)
	assert.Equal(t, int64(603), *m1.TMDBID)
	require.NotNil(t, m1.IMDBID)
	assert.Equal(t, "tt0133093", *m1.IMDBID)
	require.NotNil(t, m1.TVDBID)
	assert.Equal(t, int64(155), *m1.TVDBID)

	m2, ok := byExtID["jf-unwatched-2"]
	require.True(t, ok)
	assert.Equal(t, model.TitleTypeMovie, m2.Type)
	assert.Equal(t, 0, m2.Year)
	assert.Equal(t, model.TitleStatusPlanToWatch, m2.Status)
	assert.Equal(t, model.MatchStatusPendingReview, m2.MatchStatus)
	assert.Nil(t, m2.TMDBID)
	require.NotNil(t, m2.IMDBID)
	assert.Equal(t, "tt9990001", *m2.IMDBID)
	assert.True(t, m2.ArrIgnored)
	assert.NotNil(t, m2.DownloadedAt)

	var watchEventCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM watch_events`).Scan(&watchEventCount))
	assert.Equal(t, 0, watchEventCount)

	taskRepo := repository.NewTaskRepository(db)
	tasks, err := taskRepo.ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	for _, task := range tasks {
		assert.Equal(t, model.TaskTypeEnrichment, task.TaskType)
		var payload service.EnrichmentPayload
		require.NoError(t, json.Unmarshal([]byte(task.Payload), &payload))
		require.NotNil(t, task.DedupKey)
		assert.Equal(t, "enrichment:"+strconv.FormatInt(payload.TitleID, 10), *task.DedupKey)
		assert.False(t, payload.PreserveMatch)
		assert.Equal(t, model.TitleTypeMovie, payload.TitleType)
	}

	// Second run must skip all existing movies without creating duplicates or new tasks
	summary2, err := svc.SyncUnwatchedMovies(context.Background())
	require.NoError(t, err)
	assert.Equal(t, service.JellyfinMovieSyncSummary{
		DryRun:    false,
		Fetched:   4,
		Unwatched: 2,
		Created:   0,
		Skipped:   2,
		Errors:    0,
	}, summary2)

	titlesAfter, err := titleRepo.ListAll()
	require.NoError(t, err)
	assert.Len(t, titlesAfter, 2)
	tasksAfter, err := taskRepo.ListPending()
	require.NoError(t, err)
	assert.Len(t, tasksAfter, 2)
	assert.Equal(t, int32(2), reqCount.Load())
}

func TestJellyfinSyncService_SyncUnwatchedMovies_DeduplicationAndPreservation(t *testing.T) {
	db := setupTestDB(t)

	rating9 := 9
	rating8 := 8
	rating3 := 3

	idByTMDB := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2010, Status: model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed, TMDBID: ptr(int64(27205)), MyRating: &rating9,
	}, []model.TitleName{{Name: "Inception", Language: "en", IsPrimary: true}})
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: idByTMDB, Source: model.WatchEventSourceManual})

	idByIMDB := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 1999, Status: model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed, IMDBID: ptr("tt0137523"), MyRating: &rating8,
	}, []model.TitleName{{Name: "Fight Club", Language: "en", IsPrimary: true}})

	idByExtID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2005, Status: model.TitleStatusDropped,
		MatchStatus: model.MatchStatusConfirmed, ExternalSourceID: ptr("jf-ext-3"), MyRating: &rating3,
	}, []model.TitleName{{Name: "Dropped Movie", Language: "en", IsPrimary: true}})

	idByNameYear := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2021, Status: model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{
					"Id": "jf-new-ext-1", "Name": "Inception Renamed", "Type": "Movie", "ProductionYear": 2010,
					"ProviderIds": map[string]string{"Tmdb": "27205"},
				},
				{
					"Id": "jf-new-ext-2", "Name": "Fight Club", "Type": "Movie", "ProductionYear": 1999,
					"ProviderIds": map[string]string{"Imdb": "TT0137523"},
				},
				{
					"Id": "jf-ext-3", "Name": "Dropped Movie", "Type": "Movie", "ProductionYear": 2005,
					"ProviderIds": map[string]string{},
				},
				{
					"Id": "jf-dune-2021", "Name": "Dune", "Type": "Movie", "ProductionYear": 2021,
					"ProviderIds": map[string]string{},
				},
				{
					// Different year -> must NOT collide with Dune (2021)
					"Id": "jf-dune-1984", "Name": "Dune", "Type": "Movie", "ProductionYear": 1984,
					"ProviderIds": map[string]string{},
				},
				{
					// Empty name and no TMDB/IMDb -> unidentifiable -> skipped
					"Id": "jf-no-identity", "Name": "   ", "Type": "", "ProductionYear": 2022,
					"ProviderIds": map[string]string{"Tvdb": "1234"},
				},
				{
					// Empty name but valid TMDB -> created with fallback name
					"Id": "jf-id-only", "Name": "", "Type": "", "ProductionYear": 2023,
					"ProviderIds": map[string]string{"Tmdb": "998877"},
				},
			},
		})
	}))
	defer server.Close()

	svc := service.NewJellyfinSyncService(db, service.JellyfinSyncConfig{
		BaseURL:    server.URL,
		APIKey:     "k",
		UserID:     "u",
		HTTPClient: server.Client(),
	}, testutil.NopLogger())

	summary, err := svc.SyncUnwatchedMovies(context.Background())
	require.NoError(t, err)
	assert.Equal(t, service.JellyfinMovieSyncSummary{
		DryRun:    false,
		Fetched:   7,
		Unwatched: 7,
		Created:   2,
		Skipped:   5,
		Errors:    0,
	}, summary)

	titleRepo := repository.NewTitleRepository(db)
	weRepo := repository.NewWatchEventRepository(db)

	// Existing titles untouched
	gotTMDB, err := titleRepo.GetByID(idByTMDB)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusCompleted, gotTMDB.Status)
	require.NotNil(t, gotTMDB.MyRating)
	assert.Equal(t, 9, *gotTMDB.MyRating)
	assert.Nil(t, gotTMDB.DownloadedAt)
	countWE, err := weRepo.CountByTitleID(idByTMDB)
	require.NoError(t, err)
	assert.Equal(t, 1, countWE)

	gotIMDB, err := titleRepo.GetByID(idByIMDB)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusWatching, gotIMDB.Status)
	require.NotNil(t, gotIMDB.MyRating)
	assert.Equal(t, 8, *gotIMDB.MyRating)

	gotExt, err := titleRepo.GetByID(idByExtID)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusDropped, gotExt.Status)
	require.NotNil(t, gotExt.MyRating)
	assert.Equal(t, 3, *gotExt.MyRating)

	gotNameYear, err := titleRepo.GetByID(idByNameYear)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusCompleted, gotNameYear.Status)

	// Dune 1984 and TMDB 998877 were created
	dune1984, err := titleRepo.FindByNameAndType("Dune", model.TitleTypeMovie, 1984)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusPlanToWatch, dune1984.Status)
	assert.Equal(t, model.MatchStatusPendingReview, dune1984.MatchStatus)

	tmdbOnlyID := int64(998877)
	movieType := model.TitleTypeMovie
	tmdbOnly, err := titleRepo.FindByExternalID(nil, &tmdbOnlyID, nil, nil, nil, &movieType)
	require.NoError(t, err)
	assert.Equal(t, "TMDB 998877", tmdbOnly.PrimaryName())
	assert.Equal(t, model.MatchStatusConfirmed, tmdbOnly.MatchStatus)
}

func TestJellyfinSyncService_SyncUnwatchedMovies_DynamicSettingsAndMissingConfig(t *testing.T) {
	var httpCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{
					"Id":          "jf-dyn-1",
					"Name":        "Dynamic Movie",
					"Type":        "Movie",
					"ProviderIds": map[string]string{"Tmdb": "101"},
				},
			},
		})
	}))
	defer server.Close()

	for _, missingKey := range []string{"none", repository.SettingKeyJellyfinURL, repository.SettingKeyJellyfinAPIKey, repository.SettingKeyJellyfinUserID} {
		db := setupTestDB(t)
		if missingKey != "none" {
			tx, err := db.Begin()
			require.NoError(t, err)
			sw := repository.NewSettingWriter(tx)
			if missingKey != repository.SettingKeyJellyfinURL {
				require.NoError(t, sw.Set(context.Background(), repository.SettingKeyJellyfinURL, server.URL))
			}
			if missingKey != repository.SettingKeyJellyfinAPIKey {
				require.NoError(t, sw.Set(context.Background(), repository.SettingKeyJellyfinAPIKey, "dyn-key"))
			}
			if missingKey != repository.SettingKeyJellyfinUserID {
				require.NoError(t, sw.Set(context.Background(), repository.SettingKeyJellyfinUserID, "dyn-user"))
			}
			require.NoError(t, tx.Commit())
		}

		svc := service.NewJellyfinSyncService(db, service.JellyfinSyncConfig{HTTPClient: server.Client()}, testutil.NopLogger())
		summary, err := svc.SyncUnwatchedMovies(context.Background())
		require.NoError(t, err)
		assert.Equal(t, service.JellyfinMovieSyncSummary{}, summary)
	}
	assert.Equal(t, int32(0), httpCalls.Load(), "no HTTP requests when any credential is missing")

	// Dynamic reload on same service instance once settings are saved
	db := setupTestDB(t)
	svc := service.NewJellyfinSyncService(db, service.JellyfinSyncConfig{HTTPClient: server.Client()}, testutil.NopLogger())

	summary, err := svc.SyncUnwatchedMovies(context.Background())
	require.NoError(t, err)
	assert.Equal(t, service.JellyfinMovieSyncSummary{}, summary)
	assert.Equal(t, int32(0), httpCalls.Load())

	tx, err := db.Begin()
	require.NoError(t, err)
	sw := repository.NewSettingWriter(tx)
	require.NoError(t, sw.Set(context.Background(), repository.SettingKeyJellyfinURL, server.URL))
	require.NoError(t, sw.Set(context.Background(), repository.SettingKeyJellyfinAPIKey, "dyn-key"))
	require.NoError(t, sw.Set(context.Background(), repository.SettingKeyJellyfinUserID, "dyn-user"))
	require.NoError(t, tx.Commit())

	summary, err = svc.SyncUnwatchedMovies(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Created)
	assert.Equal(t, int32(1), httpCalls.Load())
}

func TestJellyfinSyncService_SyncUnwatchedMovies_DryRun(t *testing.T) {
	db := setupTestDB(t)
	testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2010, Status: model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed, TMDBID: ptr(int64(27205)),
	}, []model.TitleName{{Name: "Inception", Language: "en", IsPrimary: true}})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{"Id": "jf-1", "Name": "Inception", "Type": "Movie", "ProviderIds": map[string]string{"Tmdb": "27205"}},
				{"Id": "jf-2", "Name": "New Movie", "Type": "Movie", "ProviderIds": map[string]string{"Tmdb": "603"}},
				{"Id": "jf-3", "Name": "Watched Movie", "Type": "Movie", "ProviderIds": map[string]string{"Tmdb": "999"}, "UserData": map[string]any{"Played": true}},
				{"Id": "jf-4", "Name": "Some Series", "Type": "Series", "ProviderIds": map[string]string{"Tmdb": "888"}},
			},
		})
	}))
	defer server.Close()

	svc := service.NewJellyfinSyncService(db, service.JellyfinSyncConfig{
		BaseURL:    server.URL,
		APIKey:     "k",
		UserID:     "u",
		DryRun:     true,
		HTTPClient: server.Client(),
	}, testutil.NopLogger())

	summary, err := svc.SyncUnwatchedMovies(context.Background())
	require.NoError(t, err)
	assert.Equal(t, service.JellyfinMovieSyncSummary{
		DryRun:    true,
		Fetched:   4,
		Unwatched: 2,
		Created:   1,
		Skipped:   1,
		Errors:    0,
	}, summary)

	titles, err := repository.NewTitleRepository(db).ListAll()
	require.NoError(t, err)
	assert.Len(t, titles, 1)

	tasks, err := repository.NewTaskRepository(db).ListPending()
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestJellyfinSyncService_SyncUnwatchedMovies_NoWriteLockAndEnrichmentPipeline(t *testing.T) {
	db := setupTestDB(t)
	dataDir := t.TempDir()

	// Encode a minimal valid 1x1 PNG so CoverService.ExtractAndStoreAccent succeeds
	var pngBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 20, G: 120, B: 220, A: 255})
	require.NoError(t, png.Encode(&pngBuf, img))

	tmdbMux := http.NewServeMux()
	tmdbMux.HandleFunc("/movie/27205", func(w http.ResponseWriter, r *http.Request) {
		poster := "/poster27205.png"
		rt := 148
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":           27205,
			"title":        "Inception",
			"overview":     "A thief steals secrets through dream-sharing.",
			"release_date": "2010-07-16",
			"poster_path":  poster,
			"imdb_id":      "tt1375666",
			"runtime":      rt,
			"vote_average": 8.4,
			"genres":       []map[string]any{{"id": 28, "name": "Action"}},
		})
	})
	tmdbMux.HandleFunc("/movie/27205/translations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"translations": []map[string]any{
				{"iso_639_1": "en", "data": map[string]string{"title": "Inception"}},
				{"iso_639_1": "fr", "data": map[string]string{"title": "Inception"}},
			},
		})
	})
	tmdbMux.HandleFunc("/image/poster27205.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBuf.Bytes())
	})
	tmdbServer := httptest.NewServer(tmdbMux)
	defer tmdbServer.Close()

	tmdbClient := matching.NewTMDBClient("test-tmdb-key")
	tmdbClient.SetBaseURL(tmdbServer.URL)

	var concurrentWriteErr error
	jfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prove writeDB (MaxOpenConns=1) is NOT locked while FetchUserMovies executes
		txCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		concurrentWriteErr = database.WithTxContext(txCtx, db, func(tx *sql.Tx) error {
			return repository.NewSettingWriter(tx).Set(txCtx, "probe_key", "probe_ok")
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{
				{
					"Id":             "jf-inception",
					"Name":           "Inception",
					"Type":           "Movie",
					"ProductionYear": 2010,
					"ProviderIds":    map[string]string{"Tmdb": "27205"},
				},
			},
		})
	}))
	defer jfServer.Close()

	limiter := service.NewAPILimiter(20, 5)
	svc := service.NewJellyfinSyncService(db, service.JellyfinSyncConfig{
		BaseURL:    jfServer.URL,
		APIKey:     "k",
		UserID:     "u",
		HTTPClient: jfServer.Client(),
	}, testutil.NopLogger())
	svc.SetAPILimiter(limiter)

	summary, err := svc.SyncUnwatchedMovies(context.Background())
	require.NoError(t, err)
	require.NoError(t, concurrentWriteErr, "writeDB transaction must not block during FetchUserMovies HTTP call")
	assert.Equal(t, 1, summary.Created)

	pipeline := matching.NewPipeline(tmdbClient, nil, nil, nil, dataDir)
	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	titleSvc := service.NewTitleService(db, titleRepo, taskRepo, pipeline)
	coverSvc := service.NewCoverService(db, titleRepo, tmdbClient, nil, dataDir)
	coverSvc.SetAPILimiter(limiter)

	worker := service.NewTaskQueueWorker(taskRepo, titleRepo, pipeline, tmdbClient, nil, nil, settingRepo, dataDir, titleSvc, db)
	worker.SetCovers(coverSvc)
	worker.SetAPILimiter(limiter)

	var due []model.Task
	require.NoError(t, database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		var fetchErr error
		due, fetchErr = repository.NewTaskWriter(tx).FetchDue(context.Background(), 10)
		return fetchErr
	}))
	require.Len(t, due, 1)

	require.NoError(t, limiter.Wait(context.Background()))
	worker.ProcessTask(context.Background(), due[0])

	// Enrichment task should now be completed (deleted from task_queue)
	pendingAfter, err := taskRepo.ListPending()
	require.NoError(t, err)
	assert.Empty(t, pendingAfter)

	tmdbID := int64(27205)
	movieType := model.TitleTypeMovie
	enriched, err := titleRepo.FindByExternalID(nil, &tmdbID, nil, nil, nil, &movieType)
	require.NoError(t, err)
	require.NotNil(t, enriched.Overview)
	assert.Equal(t, "A thief steals secrets through dream-sharing.", *enriched.Overview)
	require.NotNil(t, enriched.Runtime)
	assert.Equal(t, 148, *enriched.Runtime)
	require.NotNil(t, enriched.CoverURL)
	assert.Equal(t, "poster27205.png", *enriched.CoverURL)
	assert.NotNil(t, enriched.AccentHex)
	assert.Contains(t, enriched.Genres, "Action")

	_, statErr := os.Stat(filepath.Join(dataDir, "covers", "poster27205.png"))
	assert.NoError(t, statErr)
}
