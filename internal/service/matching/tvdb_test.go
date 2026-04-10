package matching

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTVDBServer(t *testing.T) (*httptest.Server, *TVDBClient) {
	t.Helper()

	mux := http.NewServeMux()

	// Login
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{"token": "test-jwt-token"},
		})
	})

	// Series extended
	mux.HandleFunc("/series/81189/extended", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-jwt-token", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":    81189,
				"name":  "Breaking Bad",
				"score": 9.5,
				"genres": []map[string]string{
					{"name": "Drama"},
					{"name": "Crime"},
				},
				"averageRuntime": 47,
				"translations": map[string]interface{}{
					"nameTranslations": []map[string]string{
						{"language": "eng", "name": "Breaking Bad"},
						{"language": "fra", "name": "Breaking Bad"},
					},
					"overviewTranslations": []map[string]string{
						{"language": "eng", "overview": "A chemistry teacher diagnosed with cancer..."},
					},
				},
			},
		})
	})

	// Movie extended
	mux.HandleFunc("/movies/999/extended", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":    999,
				"name":  "Fight Club",
				"year":  1999,
				"score": 8.8,
				"genres": []map[string]string{
					{"name": "Drama"},
					{"name": "Thriller"},
				},
				"runtime": 139,
				"remoteIds": []map[string]interface{}{
					{"id": "tt0137523", "sourceId": 2},
				},
				"translations": map[string]interface{}{
					"nameTranslations": []map[string]string{
						{"language": "eng", "name": "Fight Club"},
					},
					"overviewTranslations": []map[string]string{
						{"language": "eng", "overview": "An insomniac office worker..."},
					},
				},
			},
		})
	})

	// Series by slug
	mux.HandleFunc("/series/slug/breaking-bad", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":   81189,
				"name": "Breaking Bad",
			},
		})
	})

	// Movie by slug
	mux.HandleFunc("/movies/slug/fight-club-1999", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":   999,
				"name": "Fight Club",
			},
		})
	})

	// Search
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tvdbSearchResponse{
			Data: []tvdbSearchResult{
				{TVDBID: 81189, Name: "Breaking Bad", Type: "series"},
			},
		})
	})

	// Artwork
	mux.HandleFunc("/artwork/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-tvdb-image"))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := NewTVDBClient("test-tvdb-key")
	client.SetBaseURL(server.URL)

	return server, client
}

func TestTVDBLogin(t *testing.T) {
	_, client := newTestTVDBServer(t)
	err := client.Login(context.Background())
	require.NoError(t, err)

	client.mu.Lock()
	token := client.token
	client.mu.Unlock()
	assert.Equal(t, "test-jwt-token", token)
}

func TestTVDBGetSeriesDetails(t *testing.T) {
	_, client := newTestTVDBServer(t)
	require.NoError(t, client.Login(context.Background()))

	details, err := client.GetSeriesDetails(context.Background(), 81189)
	require.NoError(t, err)
	assert.Equal(t, int64(81189), details.ID)
	assert.Equal(t, "Breaking Bad", details.Name)
	assert.InDelta(t, 9.5, details.Score, 0.01)
	assert.Len(t, details.Genres, 2)

	genres := extractSeriesGenres(details)
	assert.Contains(t, genres, "Drama")
	assert.Contains(t, genres, "Crime")

	names := extractSeriesNames(details)
	assert.Equal(t, "Breaking Bad", names["en"])
	assert.Equal(t, "Breaking Bad", names["fr"])

	overview := extractSeriesOverview(details)
	assert.Equal(t, "A chemistry teacher diagnosed with cancer...", overview)
}

func TestTVDBGetMovieDetails(t *testing.T) {
	_, client := newTestTVDBServer(t)
	require.NoError(t, client.Login(context.Background()))

	details, err := client.GetMovieDetails(context.Background(), 999)
	require.NoError(t, err)
	assert.Equal(t, int64(999), details.ID)
	assert.Equal(t, "Fight Club", details.Name)

	imdb := extractMovieIMDB(details)
	assert.Equal(t, "tt0137523", imdb)

	genres := extractMovieGenres(details)
	assert.Contains(t, genres, "Drama")

	overview := extractMovieOverview(details)
	assert.Equal(t, "An insomniac office worker...", overview)
}

func TestTVDBGetSeriesBySlug(t *testing.T) {
	_, client := newTestTVDBServer(t)
	require.NoError(t, client.Login(context.Background()))

	details, err := client.GetSeriesBySlug(context.Background(), "breaking-bad")
	require.NoError(t, err)
	assert.Equal(t, int64(81189), details.ID)
}

func TestTVDBGetMovieBySlug(t *testing.T) {
	_, client := newTestTVDBServer(t)
	require.NoError(t, client.Login(context.Background()))

	details, err := client.GetMovieBySlug(context.Background(), "fight-club-1999")
	require.NoError(t, err)
	assert.Equal(t, int64(999), details.ID)
}

func TestTVDBSearch(t *testing.T) {
	_, client := newTestTVDBServer(t)
	require.NoError(t, client.Login(context.Background()))

	results, err := client.SearchSeries(context.Background(), "Breaking Bad", 2008)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, int64(81189), results[0].TVDBID)
}

func TestTVDBDownloadCover(t *testing.T) {
	_, client := newTestTVDBServer(t)
	require.NoError(t, client.Login(context.Background()))

	destDir := t.TempDir()
	// Use a URL that will be rewritten to the mock server
	imageURL := tvdbArtworkBaseURL + "/banners/poster.jpg"
	filename, err := client.DownloadCover(imageURL, 81189, destDir)
	require.NoError(t, err)
	assert.Equal(t, "tvdb_81189.jpg", filename)

	data, err := os.ReadFile(filepath.Join(destDir, filename))
	require.NoError(t, err)
	assert.Equal(t, "fake-tvdb-image", string(data))
}

func TestTVDBDownloadCoverEmpty(t *testing.T) {
	_, client := newTestTVDBServer(t)
	_, err := client.DownloadCover("", 1, t.TempDir())
	assert.Error(t, err)
}
