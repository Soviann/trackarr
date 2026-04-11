package matching

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupPipelineTest creates a full test environment with mocked servers.
func setupPipelineTest(t *testing.T) (*Pipeline, string) {
	t.Helper()

	dataDir := t.TempDir()

	// TMDB mock
	tmdbMux := http.NewServeMux()
	tmdbMux.HandleFunc("/search/movie", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbSearchResponse{
			Results: []TMDBSearchResult{
				{ID: 550, Title: "Fight Club", ReleaseDate: "1999-10-15"},
			},
		})
	})
	tmdbMux.HandleFunc("/search/tv", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbSearchResponse{
			Results: []TMDBSearchResult{
				{ID: 1399, Name: "Breaking Bad", FirstAirDate: "2008-01-20"},
			},
		})
	})
	tmdbMux.HandleFunc("/movie/550", func(w http.ResponseWriter, r *http.Request) {
		poster := "/poster550.jpg"
		_ = json.NewEncoder(w).Encode(TMDBMovieDetails{
			ID: 550, Title: "Fight Club", ReleaseDate: "1999-10-15",
			IMDBID: "tt0137523", PosterPath: &poster,
			ExternalIDs: &struct {
				IMDBID string `json:"imdb_id"`
				TVDBID int64  `json:"tvdb_id"`
			}{IMDBID: "tt0137523"},
		})
	})
	tmdbMux.HandleFunc("/tv/1399", func(w http.ResponseWriter, r *http.Request) {
		poster := "/poster1399.jpg"
		_ = json.NewEncoder(w).Encode(TMDBTVDetails{
			ID: 1399, Name: "Breaking Bad", FirstAirDate: "2008-01-20",
			PosterPath: &poster,
			ExternalIDs: &struct {
				IMDBID string `json:"imdb_id"`
				TVDBID int64  `json:"tvdb_id"`
			}{IMDBID: "tt0903747", TVDBID: 81189},
		})
	})
	tmdbMux.HandleFunc("/movie/550/translations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbTranslationsResponse{
			Translations: []TMDBTranslation{
				{ISO639: "en", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Fight Club"}},
				{ISO639: "fr", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Title: "Fight Club"}},
			},
		})
	})
	tmdbMux.HandleFunc("/tv/1399/translations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbTranslationsResponse{
			Translations: []TMDBTranslation{
				{ISO639: "en", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Name: "Breaking Bad"}},
				{ISO639: "fr", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Name: "Breaking Bad"}},
			},
		})
	})
	tmdbMux.HandleFunc("/image/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-cover"))
	})
	tmdbServer := httptest.NewServer(tmdbMux)
	t.Cleanup(tmdbServer.Close)

	tmdbClient := NewTMDBClient("test-key")
	tmdbClient.baseURL = tmdbServer.URL

	// Gemini mock
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(geminiOKResponse(`{"confirmed": true, "confidence": "high", "reason": "Exact match"}`))
	}))
	t.Cleanup(geminiServer.Close)

	geminiClient := NewGeminiClient([]string{"test-key"})
	geminiClient.apiURL = geminiServer.URL

	// AniList mock
	anilistServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		if contains(req.Query, "Page(perPage") {
			eps := 12
			year := 2015
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"Page": map[string]interface{}{
						"media": []interface{}{
							map[string]interface{}{
								"id":       21,
								"title":    map[string]string{"romaji": "One Punch Man", "english": "One Punch Man"},
								"episodes": eps, "format": "TV", "seasonYear": year,
							},
						},
					},
				},
			})
		} else if contains(req.Query, "Media(id") {
			eps := 12
			year := 2015
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"Media": map[string]interface{}{
						"id": 21, "title": map[string]string{"romaji": "One Punch Man", "english": "One Punch Man"},
						"episodes": eps, "format": "TV", "seasonYear": year,
					},
				},
			})
		}
	}))
	t.Cleanup(anilistServer.Close)

	anilistClient := NewAniListClient()
	anilistClient.apiURL = anilistServer.URL

	pipeline := NewPipeline(tmdbClient, anilistClient, geminiClient, nil, dataDir)
	return pipeline, dataDir
}

func TestPipeline_Step1_PlexIDsConfirmed(t *testing.T) {
	pipeline, dataDir := setupPipelineTest(t)

	result, err := pipeline.Run(context.Background(), MatchInput{
		Title:  "Fight Club",
		Year:   1999,
		Type:   model.TitleTypeMovie,
		IMDBID: "tt0137523",
		TMDBID: 550,
	})
	require.NoError(t, err)
	assert.Equal(t, model.MatchStatusConfirmed, result.MatchStatus)
	assert.Equal(t, MatchSourcePlexIDs, result.MatchSource)
	assert.Equal(t, "tt0137523", result.IMDBID)
	assert.Equal(t, int64(550), result.TMDBID)
	assert.NotEmpty(t, result.Names)

	// Cover should be downloaded
	assert.NotEmpty(t, result.CoverFile)
	_, err = os.Stat(filepath.Join(dataDir, "covers", result.CoverFile))
	assert.NoError(t, err)
}

func TestPipeline_Step3_TMDBSearch(t *testing.T) {
	pipeline, _ := setupPipelineTest(t)

	// No Plex IDs — forces TMDB search (Step 3) then Gemini verification (Step 5)
	result, err := pipeline.Run(context.Background(), MatchInput{
		Title: "Fight Club",
		Year:  1999,
		Type:  model.TitleTypeMovie,
	})
	require.NoError(t, err)
	// Gemini confirms with high confidence → pending_review
	assert.Equal(t, model.MatchStatusPendingReview, result.MatchStatus)
	assert.Equal(t, MatchSourceTMDBSearch, result.MatchSource)
	assert.Equal(t, int64(550), result.TMDBID)
	assert.Equal(t, "tt0137523", result.IMDBID)
}

func TestPipeline_Step3_TVSearch(t *testing.T) {
	pipeline, _ := setupPipelineTest(t)

	result, err := pipeline.Run(context.Background(), MatchInput{
		Title: "Breaking Bad",
		Year:  2008,
		Type:  model.TitleTypeSeries,
	})
	require.NoError(t, err)
	assert.Equal(t, model.MatchStatusPendingReview, result.MatchStatus)
	assert.Equal(t, MatchSourceTMDBSearch, result.MatchSource)
	assert.Equal(t, int64(1399), result.TMDBID)
	assert.Equal(t, "tt0903747", result.IMDBID)
	assert.Equal(t, int64(81189), result.TVDBID)
}

func TestPipeline_Step4_AniListSearch(t *testing.T) {
	// Create pipeline with a TMDB that returns no results
	dataDir := t.TempDir()

	tmdbMux := http.NewServeMux()
	tmdbMux.HandleFunc("/search/movie", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbSearchResponse{})
	})
	tmdbMux.HandleFunc("/search/tv", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbSearchResponse{})
	})
	tmdbServer := httptest.NewServer(tmdbMux)
	defer tmdbServer.Close()

	tmdbClient := NewTMDBClient("key")
	tmdbClient.baseURL = tmdbServer.URL

	anilistServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if contains(req.Query, "Page(perPage") {
			eps := 12
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"Page": map[string]interface{}{
						"media": []interface{}{
							map[string]interface{}{
								"id": 21, "title": map[string]string{"romaji": "One Punch Man", "english": "One Punch Man"},
								"episodes": eps, "format": "TV",
							},
						},
					},
				},
			})
		} else {
			eps := 12
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"Media": map[string]interface{}{
						"id": 21, "title": map[string]string{"romaji": "One Punch Man", "english": "One Punch Man"},
						"episodes": eps, "format": "TV",
					},
				},
			})
		}
	}))
	defer anilistServer.Close()

	anilistClient := NewAniListClient()
	anilistClient.apiURL = anilistServer.URL

	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(geminiOKResponse(`{"confirmed": true, "confidence": "high", "reason": "match"}`))
	}))
	defer geminiServer.Close()

	geminiClient := NewGeminiClient([]string{"key"})
	geminiClient.apiURL = geminiServer.URL

	pipeline := NewPipeline(tmdbClient, anilistClient, geminiClient, nil, dataDir)

	result, err := pipeline.Run(context.Background(), MatchInput{
		Title: "One Punch Man",
		Year:  2015,
		Type:  model.TitleTypeSeries,
	})
	require.NoError(t, err)
	assert.Equal(t, MatchSourceAniListSearch, result.MatchSource)
	assert.Equal(t, int64(21), result.AniListID)
	assert.True(t, result.IsAnime)
}

func TestPipeline_NoMatch(t *testing.T) {
	dataDir := t.TempDir()

	// TMDB returns no results, no AniList (not anime)
	tmdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbSearchResponse{})
	}))
	defer tmdbServer.Close()

	tmdbClient := NewTMDBClient("key")
	tmdbClient.baseURL = tmdbServer.URL

	// Gemini fuzzy also fails
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(geminiOKResponse(`{"candidate_title": "", "candidate_year": 0, "confidence": "low", "reason": "unknown"}`))
	}))
	defer geminiServer.Close()

	geminiClient := NewGeminiClient([]string{"key"})
	geminiClient.apiURL = geminiServer.URL

	pipeline := NewPipeline(tmdbClient, nil, geminiClient, nil, dataDir)

	result, err := pipeline.Run(context.Background(), MatchInput{
		Title: "Some Obscure Title",
		Year:  2020,
		Type:  model.TitleTypeMovie,
	})
	require.NoError(t, err)
	assert.Equal(t, model.MatchStatusUnconfirmed, result.MatchStatus)
	assert.Equal(t, MatchSourceNone, result.MatchSource)
	assert.NotEmpty(t, result.Names)
	assert.Equal(t, "Some Obscure Title", result.Names[0].Name)
}

func TestPipeline_Step2_CrossRef(t *testing.T) {
	dataDir := t.TempDir()

	// Create crossref DB
	crossrefJSON := `{"data": [{"sources": ["https://anilist.co/anime/21", "https://www.themoviedb.org/tv/46298", "https://www.thetvdb.com/series/85004", "https://www.imdb.com/title/tt0388629/"], "title": "One Piece", "type": "TV", "episodes": 1000}]}`
	crossrefPath := filepath.Join(dataDir, "crossref.json")
	require.NoError(t, os.WriteFile(crossrefPath, []byte(crossrefJSON), 0o644))

	crossDB, err := LoadCrossRefDB(crossrefPath)
	require.NoError(t, err)

	// TMDB mock for enrichment
	tmdbMux := http.NewServeMux()
	tmdbMux.HandleFunc("/tv/46298", func(w http.ResponseWriter, r *http.Request) {
		poster := "/op.jpg"
		_ = json.NewEncoder(w).Encode(TMDBTVDetails{
			ID: 46298, Name: "One Piece", PosterPath: &poster,
			ExternalIDs: &struct {
				IMDBID string `json:"imdb_id"`
				TVDBID int64  `json:"tvdb_id"`
			}{IMDBID: "tt0388629", TVDBID: 85004},
		})
	})
	tmdbMux.HandleFunc("/tv/46298/translations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbTranslationsResponse{
			Translations: []TMDBTranslation{
				{ISO639: "en", Data: struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				}{Name: "One Piece"}},
			},
		})
	})
	tmdbMux.HandleFunc("/image/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("cover"))
	})
	tmdbServer := httptest.NewServer(tmdbMux)
	defer tmdbServer.Close()

	tmdbClient := NewTMDBClient("key")
	tmdbClient.baseURL = tmdbServer.URL

	pipeline := NewPipeline(tmdbClient, nil, nil, crossDB, dataDir)

	// Input: only TVDB ID known → crossref should resolve TMDB + IMDB + AniList
	result, err := pipeline.Run(context.Background(), MatchInput{
		Title:  "One Piece",
		Year:   1999,
		Type:   model.TitleTypeSeries,
		TVDBID: 85004,
	})
	require.NoError(t, err)
	assert.Equal(t, model.MatchStatusConfirmed, result.MatchStatus)
	assert.Equal(t, MatchSourceCrossRef, result.MatchSource)
	assert.Equal(t, int64(46298), result.TMDBID)
	assert.Equal(t, "tt0388629", result.IMDBID)
	assert.Equal(t, int64(21), result.AniListID)
	// Should be detected as anime due to AniList ID
	assert.True(t, result.IsAnime)
}

func TestPipeline_IMDBConflict_TMDBWins(t *testing.T) {
	dataDir := t.TempDir()

	// TMDB mock: title 550 returns IMDB "tt0137523"
	tmdbMux := http.NewServeMux()
	tmdbMux.HandleFunc("/movie/550", func(w http.ResponseWriter, r *http.Request) {
		poster := "/poster550.jpg"
		_ = json.NewEncoder(w).Encode(TMDBMovieDetails{
			ID: 550, Title: "Fight Club", ReleaseDate: "1999-10-15",
			IMDBID: "tt0137523", PosterPath: &poster,
			ExternalIDs: &struct {
				IMDBID string `json:"imdb_id"`
				TVDBID int64  `json:"tvdb_id"`
			}{IMDBID: "tt0137523"},
		})
	})
	tmdbMux.HandleFunc("/movie/550/translations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbTranslationsResponse{})
	})
	tmdbMux.HandleFunc("/image/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-cover"))
	})
	tmdbServer := httptest.NewServer(tmdbMux)
	defer tmdbServer.Close()
	tmdbClient := NewTMDBClient("test-key")
	tmdbClient.baseURL = tmdbServer.URL

	// TVDB mock: TVDB ID 999 returns a CONFLICTING IMDB "tt9999999"
	tvdbMux := http.NewServeMux()
	tvdbMux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{"token": "test-token"},
		})
	})
	tvdbMux.HandleFunc("/movies/999/extended", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":   999,
				"name": "Fight Club",
				"remoteIds": []map[string]interface{}{
					{"id": "tt9999999", "sourceId": 2}, // IMDB, conflicts with TMDB
				},
			},
		})
	})
	tvdbServer := httptest.NewServer(tvdbMux)
	defer tvdbServer.Close()
	tvdbClient := NewTVDBClient("test-key")
	tvdbClient.SetBaseURL(tvdbServer.URL)

	pipeline := NewPipeline(tmdbClient, nil, nil, nil, dataDir)
	pipeline.SetTVDB(tvdbClient)

	result, err := pipeline.Run(context.Background(), MatchInput{
		Title:  "Fight Club",
		Year:   1999,
		Type:   model.TitleTypeMovie,
		TMDBID: 550,
		TVDBID: 999,
	})
	require.NoError(t, err)
	assert.Equal(t, model.MatchStatusPendingReview, result.MatchStatus)
	assert.Equal(t, MatchSourcePlexIDs, result.MatchSource)
	assert.Equal(t, "tt0137523", result.IMDBID, "TMDB IMDB ID should win over TVDB's conflicting value")
}

func TestPipeline_NilClients(t *testing.T) {
	// Pipeline should work gracefully with nil optional clients
	pipeline := NewPipeline(nil, nil, nil, nil, t.TempDir())

	result, err := pipeline.Run(context.Background(), MatchInput{
		Title: "Test",
		Year:  2020,
		Type:  model.TitleTypeMovie,
	})
	require.NoError(t, err)
	assert.Equal(t, model.MatchStatusUnconfirmed, result.MatchStatus)
	assert.Equal(t, MatchSourceNone, result.MatchSource)
	assert.Equal(t, "Test", result.Names[0].Name)
}
