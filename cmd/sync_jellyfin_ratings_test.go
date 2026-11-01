package cmd

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncJellyfinRatings_HelpFlag(t *testing.T) {
	var buf bytes.Buffer
	err := runSyncJellyfinRatings([]string{"-h"}, &buf, nil)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Usage: trackarr sync-jellyfin-ratings")
}

func TestSyncJellyfinRatings_MissingRequiredFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		env           map[string]string
		wantErrSubstr string
	}{
		{
			name:          "all missing",
			args:          []string{},
			wantErrSubstr: "missing required configuration",
		},
		{
			name:          "missing api key and user id",
			args:          []string{"-jellyfin-url", "http://localhost:8096"},
			wantErrSubstr: "-api-key",
		},
		{
			name:          "missing user id",
			args:          []string{"-jellyfin-url", "http://localhost:8096", "-api-key", "my-key"},
			wantErrSubstr: "-user-id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear env vars to ensure isolation
			origURL := os.Getenv("JELLYFIN_URL")
			origKey := os.Getenv("JELLYFIN_API_KEY")
			origUser := os.Getenv("JELLYFIN_USER_ID")
			_ = os.Unsetenv("JELLYFIN_URL")
			_ = os.Unsetenv("JELLYFIN_API_KEY")
			_ = os.Unsetenv("JELLYFIN_USER_ID")
			defer func() {
				if origURL != "" {
					_ = os.Setenv("JELLYFIN_URL", origURL)
				}
				if origKey != "" {
					_ = os.Setenv("JELLYFIN_API_KEY", origKey)
				}
				if origUser != "" {
					_ = os.Setenv("JELLYFIN_USER_ID", origUser)
				}
			}()

			var buf bytes.Buffer
			err := runSyncJellyfinRatings(tc.args, &buf, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErrSubstr)
			assert.Contains(t, buf.String(), "Error: missing required parameter(s)")
		})
	}
}

func TestSyncJellyfinRatings_FallbackToEnv(t *testing.T) {
	db := testutil.NewTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "env-key", r.Header.Get("X-Emby-Token"))
		resp := map[string]any{
			"Items":            []any{},
			"TotalRecordCount": 0,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("JELLYFIN_URL", server.URL)
	t.Setenv("JELLYFIN_API_KEY", "env-key")
	t.Setenv("JELLYFIN_USER_ID", "env-user")

	var buf bytes.Buffer
	err := runSyncJellyfinRatings([]string{}, &buf, db)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Jellyfin Rating Sync Summary")
	assert.Contains(t, buf.String(), "Matched: 0")
}

func TestSyncJellyfinRatings_LiveAndDryRun(t *testing.T) {
	setupWithRatedMovie := func(t *testing.T) *sql.DB {
		db := testutil.NewTestDB(t)
		tmdbID := int64(12345)
		rating := 9
		title := &model.Title{
			Type:        model.TitleTypeMovie,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
			TMDBID:      &tmdbID,
			MyRating:    &rating,
		}
		testutil.CreateTitle(t, db, title, []model.TitleName{
			{Name: "Interstellar", Language: "en", IsPrimary: true},
		})
		return db
	}

	t.Run("dry run mode - no rating pushed", func(t *testing.T) {
		db := setupWithRatedMovie(t)

		pushed := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				resp := map[string]any{
					"Items": []map[string]any{
						{
							"Id":   "jf-item-1",
							"Name": "Interstellar",
							"Type": "Movie",
							"ProviderIds": map[string]string{
								"Tmdb": "12345",
							},
						},
					},
					"TotalRecordCount": 1,
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			if r.Method == http.MethodPost {
				pushed = true
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		args := []string{
			"-jellyfin-url", server.URL,
			"-api-key", "test-api-key",
			"-user-id", "test-user-id",
			"-dry-run",
		}

		err := runSyncJellyfinRatings(args, &buf, db)
		require.NoError(t, err)
		assert.False(t, pushed, "dry-run must not execute POST /Rating")
		out := buf.String()
		assert.Contains(t, out, "Mode: DRY RUN")
		assert.Contains(t, out, "Matched: 1")
		assert.Contains(t, out, "Updated: 0")
		assert.Contains(t, out, "Skipped: 1")
		assert.Contains(t, out, "Missing: 0")
		assert.Contains(t, out, "Errors:  0")
	})

	t.Run("live mode - rating pushed", func(t *testing.T) {
		db := setupWithRatedMovie(t)

		pushedRating := ""
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				resp := map[string]any{
					"Items": []map[string]any{
						{
							"Id":   "jf-item-1",
							"Name": "Interstellar",
							"Type": "Movie",
							"ProviderIds": map[string]string{
								"Tmdb": "12345",
							},
						},
					},
					"TotalRecordCount": 1,
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			if r.Method == http.MethodPost {
				pushedRating = r.URL.Query().Get("rating")
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		var buf bytes.Buffer
		args := []string{
			"-jellyfin-url", server.URL,
			"-api-key", "test-api-key",
			"-user-id", "test-user-id",
		}

		err := runSyncJellyfinRatings(args, &buf, db)
		require.NoError(t, err)
		assert.Equal(t, "9", pushedRating)
		out := buf.String()
		assert.Contains(t, out, "Mode:    LIVE")
		assert.Contains(t, out, "Matched: 1")
		assert.Contains(t, out, "Updated: 1")
		assert.Contains(t, out, "Skipped: 0")
		assert.Contains(t, out, "Missing: 0")
		assert.Contains(t, out, "Errors:  0")
	})
}

func TestSyncJellyfinRatings_DatabaseNotFound(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentDir := filepath.Join(tempDir, "does-not-exist")
	t.Setenv("DATA_DIR", nonExistentDir)

	var buf bytes.Buffer
	args := []string{
		"-jellyfin-url", "http://localhost:8096",
		"-api-key", "key",
		"-user-id", "user",
	}

	err := runSyncJellyfinRatings(args, &buf, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database file not found")
}

func TestSyncJellyfinRatings_Entrypoint(t *testing.T) {
	// Calling SyncJellyfinRatings with -h prints usage and returns nil without panic
	err := SyncJellyfinRatings([]string{"-h"})
	assert.NoError(t, err)
}

func TestIsRegularFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "testfile-*")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_ = tmpFile.Close()

	assert.True(t, isRegularFile(tmpFile.Name()))
	assert.False(t, isRegularFile(os.TempDir())) // Directory, not regular file
	assert.False(t, isRegularFile(fmt.Sprintf("%s/nonexistent-%d", os.TempDir(), os.Getpid())))
}
