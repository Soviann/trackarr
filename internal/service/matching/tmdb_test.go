package matching

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTMDBServer(t *testing.T) (*httptest.Server, *TMDBClient) {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/search/movie", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.URL.Query().Get("api_key"))
		json.NewEncoder(w).Encode(tmdbSearchResponse{
			Results: []TMDBSearchResult{
				{ID: 550, Title: "Fight Club", ReleaseDate: "1999-10-15", Overview: "An insomniac office worker..."},
				{ID: 551, Title: "Fight Club 2", ReleaseDate: "2025-01-01"},
			},
		})
	})

	mux.HandleFunc("/search/tv", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbSearchResponse{
			Results: []TMDBSearchResult{
				{ID: 1399, Name: "Breaking Bad", FirstAirDate: "2008-01-20"},
			},
		})
	})

	mux.HandleFunc("/movie/550", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(TMDBMovieDetails{
			ID:          550,
			Title:       "Fight Club",
			ReleaseDate: "1999-10-15",
			IMDBID:      "tt0137523",
			ExternalIDs: &struct {
				IMDBID string `json:"imdb_id"`
				TVDBID int64  `json:"tvdb_id"`
			}{IMDBID: "tt0137523"},
		})
	})

	mux.HandleFunc("/tv/1399", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(TMDBTVDetails{
			ID:           1399,
			Name:         "Breaking Bad",
			FirstAirDate: "2008-01-20",
			Seasons: []struct {
				SeasonNumber int `json:"season_number"`
				EpisodeCount int `json:"episode_count"`
			}{
				{SeasonNumber: 1, EpisodeCount: 7},
				{SeasonNumber: 2, EpisodeCount: 13},
			},
			ExternalIDs: &struct {
				IMDBID string `json:"imdb_id"`
				TVDBID int64  `json:"tvdb_id"`
			}{IMDBID: "tt0903747", TVDBID: 81189},
		})
	})

	mux.HandleFunc("/tv/1399/season/1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbSeasonResponse{
			Episodes: []TMDBEpisode{
				{ID: 1, Name: "Pilot", EpisodeNumber: 1, SeasonNumber: 1, AirDate: "2008-01-20"},
				{ID: 2, Name: "Cat's in the Bag...", EpisodeNumber: 2, SeasonNumber: 1, AirDate: "2008-01-27"},
			},
		})
	})

	mux.HandleFunc("/movie/550/translations", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tmdbTranslationsResponse{
			Translations: []TMDBTranslation{
				{ISO639: "en", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Fight Club"}},
				{ISO639: "fr", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Fight Club"}},
				{ISO639: "de", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Fight Club"}},
			},
		})
	})

	mux.HandleFunc("/image/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake-image-data"))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := NewTMDBClient("test-key")
	client.baseURL = server.URL

	return server, client
}

func TestTMDBSearchMovie(t *testing.T) {
	_, client := newTestTMDBServer(t)

	results, err := client.SearchMovie("Fight Club", 1999)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, int64(550), results[0].ID)
	assert.Equal(t, "Fight Club", results[0].DisplayTitle())
	assert.Equal(t, 1999, results[0].Year())
}

func TestTMDBSearchTV(t *testing.T) {
	_, client := newTestTMDBServer(t)

	results, err := client.SearchTV("Breaking Bad", 2008)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Breaking Bad", results[0].DisplayTitle())
	assert.Equal(t, 2008, results[0].Year())
}

func TestTMDBGetMovieDetails(t *testing.T) {
	_, client := newTestTMDBServer(t)

	details, err := client.GetMovieDetails(550)
	require.NoError(t, err)
	assert.Equal(t, "Fight Club", details.Title)
	assert.Equal(t, "tt0137523", details.IMDBID)
	assert.Equal(t, "tt0137523", details.ExternalIDs.IMDBID)
}

func TestTMDBGetTVDetails(t *testing.T) {
	_, client := newTestTMDBServer(t)

	details, err := client.GetTVDetails(1399)
	require.NoError(t, err)
	assert.Equal(t, "Breaking Bad", details.Name)
	assert.Equal(t, "tt0903747", details.ExternalIDs.IMDBID)
	assert.Equal(t, int64(81189), details.ExternalIDs.TVDBID)
	assert.Len(t, details.Seasons, 2)
}

func TestTMDBGetTVSeasonEpisodes(t *testing.T) {
	_, client := newTestTMDBServer(t)

	episodes, err := client.GetTVSeasonEpisodes(1399, 1)
	require.NoError(t, err)
	assert.Len(t, episodes, 2)
	assert.Equal(t, "Pilot", episodes[0].Name)
	assert.Equal(t, 1, episodes[0].EpisodeNumber)
}

func TestTMDBGetTitleNames(t *testing.T) {
	_, client := newTestTMDBServer(t)

	names, err := client.GetTitleNames(550, "movie")
	require.NoError(t, err)
	assert.Equal(t, "Fight Club", names["en"])
	assert.Equal(t, "Fight Club", names["fr"])
	// German should be filtered out (only en/fr)
	_, hasDE := names["de"]
	assert.False(t, hasDE)
}

func TestTMDBDownloadCover(t *testing.T) {
	_, client := newTestTMDBServer(t)

	destDir := t.TempDir()
	filename, err := client.DownloadCover("/abc123.jpg", destDir)
	require.NoError(t, err)
	assert.Equal(t, "abc123.jpg", filename)

	data, err := os.ReadFile(filepath.Join(destDir, filename))
	require.NoError(t, err)
	assert.Equal(t, "fake-image-data", string(data))
}

func TestTMDBDownloadCoverEmptyPath(t *testing.T) {
	_, client := newTestTMDBServer(t)

	_, err := client.DownloadCover("", t.TempDir())
	assert.Error(t, err)
}

func TestTMDBSearchResultYear(t *testing.T) {
	r := TMDBSearchResult{ReleaseDate: ""}
	assert.Equal(t, 0, r.Year())

	r = TMDBSearchResult{FirstAirDate: "2020-05-01"}
	assert.Equal(t, 2020, r.Year())
}
