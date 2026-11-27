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

func newTestTMDBServer(t *testing.T) (*httptest.Server, *TMDBClient) {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/search/movie", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.URL.Query().Get("api_key"))
		_ = json.NewEncoder(w).Encode(tmdbSearchResponse{
			Results: []TMDBSearchResult{
				{ID: 550, Title: "Fight Club", ReleaseDate: "1999-10-15", Overview: "An insomniac office worker..."},
				{ID: 551, Title: "Fight Club 2", ReleaseDate: "2025-01-01"},
			},
		})
	})

	mux.HandleFunc("/search/tv", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbSearchResponse{
			Results: []TMDBSearchResult{
				{ID: 1399, Name: "Breaking Bad", FirstAirDate: "2008-01-20"},
			},
		})
	})

	mux.HandleFunc("/movie/550", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(TMDBMovieDetails{
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
		_ = json.NewEncoder(w).Encode(TMDBTVDetails{
			ID:           1399,
			Name:         "Breaking Bad",
			FirstAirDate: "2008-01-20",
			CreatedBy: []TMDBCreator{
				{Name: "Vince Gilligan"},
			},
			Seasons: []struct {
				SeasonNumber int    `json:"season_number"`
				EpisodeCount int    `json:"episode_count"`
				AirDate      string `json:"air_date"`
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
		_ = json.NewEncoder(w).Encode(tmdbSeasonResponse{
			Episodes: []TMDBEpisode{
				{ID: 1, Name: "Pilot", EpisodeNumber: 1, SeasonNumber: 1, AirDate: "2008-01-20"},
				{ID: 2, Name: "Cat's in the Bag...", EpisodeNumber: 2, SeasonNumber: 1, AirDate: "2008-01-27"},
			},
		})
	})

	mux.HandleFunc("/movie/550/translations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbTranslationsResponse{
			Translations: []TMDBTranslation{
				{ISO639: "en", ISO3166: "US", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Fight Club"}},
				{ISO639: "fr", ISO3166: "FR", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Fight Club"}},
				{ISO639: "de", ISO3166: "DE", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Fight Club"}},
			},
		})
	})

	// Movie 680 (Pulp Fiction): a real-world case where TMDB returns BOTH
	// fr-FR and fr-CA with different titles — the canonical bug we want to
	// guard against.
	mux.HandleFunc("/movie/680/translations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbTranslationsResponse{
			Translations: []TMDBTranslation{
				{ISO639: "en", ISO3166: "US", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Pulp Fiction"}},
				{ISO639: "fr", ISO3166: "CA", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Fiction Pulpeuse"}},
				{ISO639: "fr", ISO3166: "FR", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Pulp Fiction"}},
			},
		})
	})

	// Movie 681: only non-FR French variants present — the "fr" key must
	// be absent so the display layer falls back to English.
	mux.HandleFunc("/movie/681/translations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbTranslationsResponse{
			Translations: []TMDBTranslation{
				{ISO639: "en", ISO3166: "US", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Some Movie"}},
				{ISO639: "fr", ISO3166: "CA", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Quelque Film"}},
				{ISO639: "fr", ISO3166: "BE", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Un Film Quelconque"}},
			},
		})
	})

	mux.HandleFunc("/image/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-image-data"))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := NewTMDBClient("test-key")
	client.baseURL = server.URL

	return server, client
}

func TestTMDBSearchMovie(t *testing.T) {
	_, client := newTestTMDBServer(t)

	results, err := client.SearchMovie(context.Background(), "Fight Club", 1999)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, int64(550), results[0].ID)
	assert.Equal(t, "Fight Club", results[0].DisplayTitle())
	assert.Equal(t, 1999, results[0].Year())
}

func TestTMDBSearchTV(t *testing.T) {
	_, client := newTestTMDBServer(t)

	results, err := client.SearchTV(context.Background(), "Breaking Bad", 2008)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Breaking Bad", results[0].DisplayTitle())
	assert.Equal(t, 2008, results[0].Year())
}

func TestTMDBGetMovieDetails(t *testing.T) {
	_, client := newTestTMDBServer(t)

	details, err := client.GetMovieDetails(context.Background(), 550)
	require.NoError(t, err)
	assert.Equal(t, "Fight Club", details.Title)
	assert.Equal(t, "tt0137523", details.IMDBID)
	assert.Equal(t, "tt0137523", details.ExternalIDs.IMDBID)
}

func TestTMDBGetTVDetails(t *testing.T) {
	_, client := newTestTMDBServer(t)

	details, err := client.GetTVDetails(context.Background(), 1399)
	require.NoError(t, err)
	assert.Equal(t, "Breaking Bad", details.Name)
	assert.Equal(t, "tt0903747", details.ExternalIDs.IMDBID)
	assert.Equal(t, int64(81189), details.ExternalIDs.TVDBID)
	assert.Len(t, details.Seasons, 2)
	require.Len(t, details.CreatedBy, 1)
	assert.Equal(t, "Vince Gilligan", details.CreatedBy[0].Name)
}

func TestTMDBGetTVSeasonEpisodes(t *testing.T) {
	_, client := newTestTMDBServer(t)

	episodes, err := client.GetTVSeasonEpisodes(context.Background(), 1399, 1)
	require.NoError(t, err)
	assert.Len(t, episodes, 2)
	assert.Equal(t, "Pilot", episodes[0].Name)
	assert.Equal(t, 1, episodes[0].EpisodeNumber)
}

func TestTMDBGetTitleNames(t *testing.T) {
	_, client := newTestTMDBServer(t)

	names, err := client.GetTitleNames(context.Background(), 550, "movie")
	require.NoError(t, err)
	assert.Equal(t, "Fight Club", names["en"])
	assert.Equal(t, "Fight Club", names["fr"])
	assert.Equal(t, "Fight Club", names["de"])
}

func TestTMDBGetTitleNamesPrefersFrFRoverFrCA(t *testing.T) {
	_, client := newTestTMDBServer(t)

	names, err := client.GetTitleNames(context.Background(), 680, "movie")
	require.NoError(t, err)
	assert.Equal(t, "Pulp Fiction", names["en"])
	// fr-FR must win over fr-CA, regardless of TMDB array ordering.
	assert.Equal(t, "Pulp Fiction", names["fr"])
}

func TestTMDBGetTitleNamesNoFrFRSkipsFrenchEntirely(t *testing.T) {
	_, client := newTestTMDBServer(t)

	names, err := client.GetTitleNames(context.Background(), 681, "movie")
	require.NoError(t, err)
	assert.Equal(t, "Some Movie", names["en"])
	// No fr-FR available → no "fr" key at all, so the display layer
	// will fall back to English instead of surfacing fr-CA or fr-BE.
	_, hasFR := names["fr"]
	assert.False(t, hasFR, "non-FR French variants must not be stored as fr")
}

func TestTMDBDownloadCover(t *testing.T) {
	_, client := newTestTMDBServer(t)

	destDir := t.TempDir()
	filename, err := client.DownloadCover(context.Background(), "/abc123.jpg", destDir)
	require.NoError(t, err)
	assert.Equal(t, "abc123.jpg", filename)

	data, err := os.ReadFile(filepath.Join(destDir, filename))
	require.NoError(t, err)
	assert.Equal(t, "fake-image-data", string(data))
}

func TestTMDBDownloadCoverEmptyPath(t *testing.T) {
	_, client := newTestTMDBServer(t)

	_, err := client.DownloadCover(context.Background(), "", t.TempDir())
	assert.Error(t, err)
}

func TestTMDBSearchResultYear(t *testing.T) {
	r := TMDBSearchResult{ReleaseDate: ""}
	assert.Equal(t, 0, r.Year())

	r = TMDBSearchResult{FirstAirDate: "2020-05-01"}
	assert.Equal(t, 2020, r.Year())
}

func TestExtractFlatrateProvidersFR(t *testing.T) {
	wp := &tmdbWatchProviders{Results: map[string]tmdbWatchProviderCountry{
		"FR": {Flatrate: []TMDBWatchProvider{
			{ProviderID: 119, ProviderName: "Amazon Prime Video"},
			{ProviderID: 8, ProviderName: "Netflix"},
		}},
		"US": {Flatrate: []TMDBWatchProvider{{ProviderID: 9, ProviderName: "Amazon Prime Video"}}},
	}}
	got := ExtractFlatrateProvidersFR(wp)
	assert.JSONEq(t, `[{"id":119,"name":"Amazon Prime Video"},{"id":8,"name":"Netflix"}]`, got)

	// No FR entry → empty array, never null.
	assert.Equal(t, "[]", ExtractFlatrateProvidersFR(&tmdbWatchProviders{Results: map[string]tmdbWatchProviderCountry{
		"US": {Flatrate: []TMDBWatchProvider{{ProviderID: 9, ProviderName: "Amazon Prime Video"}}},
	}}))

	// Nil providers → empty array.
	assert.Equal(t, "[]", ExtractFlatrateProvidersFR(nil))
}

func TestExtractOriginCountry(t *testing.T) {
	assert.Nil(t, ExtractOriginCountry(nil))
	assert.Nil(t, ExtractOriginCountry([]string{}))
	assert.Nil(t, ExtractOriginCountry([]string{""}))

	got := ExtractOriginCountry([]string{"KR", "US"})
	assert.NotNil(t, got)
	assert.Equal(t, "KR", *got)

	got = ExtractOriginCountry([]string{"jp"})
	assert.NotNil(t, got)
	assert.Equal(t, "JP", *got) // uppercased
}

func TestMarshalCredits(t *testing.T) {
	assert.Equal(t, "[]", marshalCredits(nil))
	assert.Equal(t, "[]", marshalCredits(&TMDBCredits{}))

	credits := &TMDBCredits{
		Crew: []TMDBCrewMember{
			{Name: "Christopher Nolan", Job: "Director"},
			{Name: "Christopher Nolan", Job: "Screenplay", Department: "Writing"},
			{Name: "Kai Bird", Job: "Author", Department: "Writing"},
			{Name: "Ludwig Göransson", Job: "Original Music Composer", Department: "Sound"},
			{Name: "John Williams", Job: "Music Score", Department: "Sound"},
			{Name: "Random Crew", Job: "Camera Operator", Department: "Camera"},
		},
		Cast: []TMDBCastMember{
			{Name: "Cillian Murphy", Character: "J. Robert Oppenheimer"},
			{Name: "Emily Blunt", Character: "Kitty Oppenheimer"},
			{Name: "Matt Damon", Character: ""}, // Empty character -> "Actor"
		},
	}

	got := marshalCredits(credits)
	expected := `[
		{"name":"Christopher Nolan","role":"Director"},
		{"name":"Christopher Nolan","role":"Writer"},
		{"name":"Kai Bird","role":"Writer"},
		{"name":"Ludwig Göransson","role":"Composer"},
		{"name":"John Williams","role":"Composer"},
		{"name":"Cillian Murphy","role":"J. Robert Oppenheimer"},
		{"name":"Emily Blunt","role":"Kitty Oppenheimer"},
		{"name":"Matt Damon","role":"Actor"}
	]`
	assert.JSONEq(t, expected, got)

	// Test cap at 20 actors
	manyCast := make([]TMDBCastMember, 25)
	for i := 0; i < 25; i++ {
		manyCast[i] = TMDBCastMember{Name: "Actor " + string(rune('A'+i)), Character: "Character"}
	}
	capped := marshalCredits(&TMDBCredits{Cast: manyCast})
	var parsed []map[string]string
	err := json.Unmarshal([]byte(capped), &parsed)
	require.NoError(t, err)
	assert.Len(t, parsed, 20)
	assert.Equal(t, "Actor A", parsed[0]["name"])
	assert.Equal(t, "Actor T", parsed[19]["name"])
}

func TestExtractTVMetadata_CreatedBy(t *testing.T) {
	t.Run("nominal combines created_by and crew writers without duplicates", func(t *testing.T) {
		details := &TMDBTVDetails{
			ID:        1396,
			Name:      "Breaking Bad",
			CreatedBy: []TMDBCreator{{Name: "Vince Gilligan"}, {Name: "  "}},
			Credits: &TMDBCredits{
				Crew: []TMDBCrewMember{
					{Name: "Michelle MacLaren", Job: "Director", Department: "Directing"},
					{Name: "Vince Gilligan", Job: "Director", Department: "Directing"},
					{Name: "Vince Gilligan", Job: "Writer", Department: "Writing"},
					{Name: "Peter Gould", Job: "Screenplay", Department: "Writing"},
					{Name: "Dave Porter", Job: "Original Music Composer", Department: "Sound"},
				},
				Cast: []TMDBCastMember{
					{Name: "Bryan Cranston", Character: "Walter White"},
				},
			},
		}

		_, credits, _, _ := ExtractTVMetadata(details)
		expected := `[
			{"name":"Michelle MacLaren","role":"Director"},
			{"name":"Vince Gilligan","role":"Director"},
			{"name":"Vince Gilligan","role":"Writer"},
			{"name":"Peter Gould","role":"Writer"},
			{"name":"Dave Porter","role":"Composer"},
			{"name":"Bryan Cranston","role":"Walter White"}
		]`
		assert.JSONEq(t, expected, credits)
	})

	t.Run("boundary extracts created_by even when credits block is nil", func(t *testing.T) {
		details := &TMDBTVDetails{
			ID:        100,
			Name:      "Creator Only Show",
			CreatedBy: []TMDBCreator{{Name: "Craig Mazin"}},
			Credits:   nil,
		}
		_, credits, _, _ := ExtractTVMetadata(details)
		assert.JSONEq(t, `[{"name":"Craig Mazin","role":"Writer"}]`, credits)
	})

	t.Run("boundary blank creator names with nil credits yields empty array", func(t *testing.T) {
		details := &TMDBTVDetails{
			ID:        101,
			Name:      "Empty Creators",
			CreatedBy: []TMDBCreator{{Name: "   "}},
			Credits:   nil,
		}
		_, credits, _, _ := ExtractTVMetadata(details)
		assert.Equal(t, "[]", credits)
	})
}
