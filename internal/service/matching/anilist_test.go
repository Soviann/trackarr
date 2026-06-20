package matching

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAniListServer(t *testing.T) (*httptest.Server, *AniListClient) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		switch {
		case contains(req.Query, "Page(perPage"):
			// Search query
			malID := int64(21)
			eps := 148
			year := 2011
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"Page": map[string]any{
						"media": []any{
							map[string]any{
								"id":    21,
								"idMal": malID,
								"title": map[string]string{
									"romaji":  "One Punch Man",
									"english": "One Punch Man",
								},
								"episodes":   eps,
								"format":     "TV",
								"seasonYear": year,
							},
						},
					},
				},
			})

		case contains(req.Query, "Media(id"):
			// Details query
			malID := int64(21)
			eps := 12
			year := 2015
			startYear, startMonth, startDay := 2023, 3, 4
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"Media": map[string]any{
						"id":    21,
						"idMal": malID,
						"title": map[string]string{
							"romaji":  "One Punch Man",
							"english": "One Punch Man",
						},
						"episodes":   eps,
						"format":     "TV",
						"seasonYear": year,
						"coverImage": map[string]string{
							"extraLarge": "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx21-YCDoj1EkAxFn.jpg",
							"large":      "https://s4.anilist.co/file/anilistcdn/media/anime/cover/medium/bx21-YCDoj1EkAxFn.jpg",
						},
						"startDate": map[string]any{
							"year":  startYear,
							"month": startMonth,
							"day":   startDay,
						},
					},
				},
			})

		case contains(req.Query, "SaveMediaListEntry"):
			// Sync rating mutation
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"SaveMediaListEntry": map[string]any{
						"id":    1,
						"score": 85,
					},
				},
			})
		}
	}))
	t.Cleanup(server.Close)

	client := NewAniListClient()
	client.apiURL = server.URL

	return server, client
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAniListSearchAnime(t *testing.T) {
	_, client := newTestAniListServer(t)

	results, err := client.SearchAnime(context.Background(), "One Punch Man")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, int64(21), results[0].ID)
	assert.Equal(t, "One Punch Man", results[0].RomajiTitle)
	assert.Equal(t, "One Punch Man", results[0].DisplayTitle())
	assert.NotNil(t, results[0].MALID)
	assert.Equal(t, int64(21), *results[0].MALID)
}

func TestAniListGetAnimeDetails(t *testing.T) {
	_, client := newTestAniListServer(t)

	details, err := client.GetAnimeDetails(context.Background(), 21)
	require.NoError(t, err)
	assert.Equal(t, int64(21), details.ID)
	assert.Equal(t, "TV", details.Format)
	assert.NotNil(t, details.Episodes)
	assert.Equal(t, 12, *details.Episodes)
	assert.Equal(t, "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx21-YCDoj1EkAxFn.jpg", details.CoverURL)
	assert.NotNil(t, details.StartDate)
	assert.Equal(t, "2023-03-04", *details.StartDate)
}

func TestAniListSyncRating(t *testing.T) {
	_, client := newTestAniListServer(t)

	err := client.SyncRating(context.Background(), 21, 85, "test-token")
	require.NoError(t, err)
}

func TestAniListGetNames(t *testing.T) {
	_, client := newTestAniListServer(t)

	names, err := client.GetNames(context.Background(), 21)
	require.NoError(t, err)
	assert.Equal(t, "One Punch Man", names.Romaji)
	assert.Equal(t, "One Punch Man", names.English)
}

func TestAniListDownloadCover(t *testing.T) {
	// Create a test HTTP server that serves a fake image
	imgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-image-data"))
	}))
	t.Cleanup(imgServer.Close)

	client := NewAniListClient()
	destDir := t.TempDir()
	filename, err := client.DownloadCover(context.Background(), imgServer.URL+"/cover/large/bx21-YCDoj1EkAxFn.jpg", destDir)
	require.NoError(t, err)
	assert.Equal(t, "al-bx21-YCDoj1EkAxFn.jpg", filename)

	data, err := os.ReadFile(filepath.Join(destDir, filename))
	require.NoError(t, err)
	assert.Equal(t, "fake-image-data", string(data))
}

func TestAniListDownloadCoverEmptyURL(t *testing.T) {
	client := NewAniListClient()
	_, err := client.DownloadCover(context.Background(), "", t.TempDir())
	assert.Error(t, err)
}

func TestFilenameFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{
			"https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx21459-nYh85uj2Fuwr.jpg",
			"al-bx21459-nYh85uj2Fuwr.jpg",
		},
		{
			"https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx21-YCDoj1EkAxFn.png",
			"al-bx21-YCDoj1EkAxFn.png",
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, filenameFromURL(tt.url))
	}
}

func TestAniListSearchResultDisplayTitle(t *testing.T) {
	r := AniListSearchResult{RomajiTitle: "Shingeki no Kyojin", EnglishTitle: ""}
	assert.Equal(t, "Shingeki no Kyojin", r.DisplayTitle())

	r.EnglishTitle = "Attack on Titan"
	assert.Equal(t, "Attack on Titan", r.DisplayTitle())
}

func TestAniListSaveMediaListEntry_WithScoreAndProgress(t *testing.T) {
	var captured string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = string(body)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":{"SaveMediaListEntry":{"id":42}}}`))
	}))
	defer server.Close()

	client := NewAniListClientWithURL(server.URL)
	score := 9
	err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryInput{
		MediaID:  166240,
		Status:   "COMPLETED",
		Progress: 12,
		Score:    &score,
	}, "test-token")
	require.NoError(t, err)
	assert.Contains(t, captured, `"mediaId":166240`)
	assert.Contains(t, captured, `"status":"COMPLETED"`)
	assert.Contains(t, captured, `"progress":12`)
	assert.Contains(t, captured, `"scoreRaw":90`)
}

func TestAniListSaveMediaListEntry_OmitsScoreWhenNil(t *testing.T) {
	var captured string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = string(body)
		_, _ = w.Write([]byte(`{"data":{"SaveMediaListEntry":{"id":42}}}`))
	}))
	defer server.Close()

	client := NewAniListClientWithURL(server.URL)
	err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryInput{
		MediaID:  166240,
		Status:   "CURRENT",
		Progress: 5,
		Score:    nil,
	}, "test-token")
	require.NoError(t, err)
	assert.NotContains(t, captured, `"scoreRaw"`)
}

func TestAniListSaveMediaListEntry_Returns401AsTokenInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"message":"invalid token"}]}`))
	}))
	defer server.Close()

	client := NewAniListClientWithURL(server.URL)
	err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryInput{
		MediaID: 1, Status: "CURRENT",
	}, "bad-token")
	var tokenInvalid TokenInvalidError
	assert.ErrorAs(t, err, &tokenInvalid)
}
