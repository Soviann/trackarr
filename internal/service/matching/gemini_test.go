package matching

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func geminiOKResponse(jsonStr string) []byte {
	resp := geminiResponse{
		Candidates: []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		}{
			{Content: struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			}{Parts: []struct {
				Text string `json:"text"`
			}{{Text: jsonStr}}}},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestGeminiVerifyMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Query().Get("key"), "test-key")
		_, _ = w.Write(geminiOKResponse(`{"confirmed": true, "confidence": "high", "reason": "Exact title and year match"}`))
	}))
	defer server.Close()

	client := NewGeminiClient([]string{"test-key-1"})
	client.apiURL = server.URL

	result, err := client.VerifyMatch(
		context.Background(),
		PlexInfo{Title: "Fight Club", Year: 1999, Type: "movie"},
		MatchCandidate{Title: "Fight Club", Year: 1999, TMDBID: 550, IMDBID: "tt0137523"},
	)
	require.NoError(t, err)
	assert.True(t, result.Confirmed)
	assert.Equal(t, "high", result.Confidence)
	assert.NotEmpty(t, result.Reason)
}

func TestGeminiFuzzyResolve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(geminiOKResponse(`{"candidate_title": "Fight Club", "candidate_year": 1999, "confidence": "high", "reason": "Well-known movie"}`))
	}))
	defer server.Close()

	client := NewGeminiClient([]string{"test-key-1"})
	client.apiURL = server.URL

	result, err := client.FuzzyResolve(context.Background(), PlexInfo{Title: "FightClub", Year: 1999, Type: "movie"})
	require.NoError(t, err)
	assert.Equal(t, "Fight Club", result.CandidateTitle)
	assert.Equal(t, 1999, result.CandidateYear)
}

func TestGeminiKeyRotationOn429(t *testing.T) {
	var callCount atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			// First key: rate limited
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limited"))
			return
		}
		// Second key: success
		assert.Contains(t, r.URL.Query().Get("key"), "key-2")
		_, _ = w.Write(geminiOKResponse(`{"confirmed": true, "confidence": "high", "reason": "match"}`))
	}))
	defer server.Close()

	client := NewGeminiClient([]string{"key-1", "key-2"})
	client.apiURL = server.URL

	result, err := client.VerifyMatch(
		context.Background(),
		PlexInfo{Title: "Test", Year: 2020, Type: "movie"},
		MatchCandidate{Title: "Test", Year: 2020},
	)
	require.NoError(t, err)
	assert.True(t, result.Confirmed)
	assert.Equal(t, int64(2), callCount.Load())
}

func TestGeminiKeyRotationOn503(t *testing.T) {
	var callCount atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			// First key: 503 Service Unavailable
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("service unavailable"))
			return
		}
		// Second key: success
		assert.Contains(t, r.URL.Query().Get("key"), "key-2")
		_, _ = w.Write(geminiOKResponse(`{"confirmed": true, "confidence": "high", "reason": "match"}`))
	}))
	defer server.Close()

	client := NewGeminiClient([]string{"key-1", "key-2"})
	client.apiURL = server.URL

	result, err := client.VerifyMatch(
		context.Background(),
		PlexInfo{Title: "Test", Year: 2020, Type: "movie"},
		MatchCandidate{Title: "Test", Year: 2020},
	)
	require.NoError(t, err)
	assert.True(t, result.Confirmed)
	assert.Equal(t, int64(2), callCount.Load())
}

func TestGeminiAllKeysRateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewGeminiClient([]string{"key-1", "key-2"})
	client.apiURL = server.URL

	_, err := client.VerifyMatch(
		context.Background(),
		PlexInfo{Title: "Test", Year: 2020, Type: "movie"},
		MatchCandidate{Title: "Test", Year: 2020},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all API keys failed")
}

func TestGeminiResponseWithMarkdownFences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Response wrapped in markdown code fences
		_, _ = w.Write(geminiOKResponse("```json\n{\"confirmed\": true, \"confidence\": \"high\", \"reason\": \"match\"}\n```"))
	}))
	defer server.Close()

	client := NewGeminiClient([]string{"key-1"})
	client.apiURL = server.URL

	result, err := client.VerifyMatch(
		context.Background(),
		PlexInfo{Title: "Test", Year: 2020, Type: "movie"},
		MatchCandidate{Title: "Test", Year: 2020},
	)
	require.NoError(t, err)
	assert.True(t, result.Confirmed)
}

func TestParseJSONFromResponse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"plain JSON", `{"confirmed": true, "confidence": "high", "reason": "ok"}`, true},
		{"with json fence", "```json\n{\"confirmed\": true, \"confidence\": \"high\", \"reason\": \"ok\"}\n```", true},
		{"with plain fence", "```\n{\"confirmed\": false, \"confidence\": \"low\", \"reason\": \"no\"}\n```", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v MatchVerification
			err := parseJSONFromResponse(tt.input, &v)
			require.NoError(t, err)
			assert.Equal(t, tt.want, v.Confirmed)
		})
	}
}

func TestDisambiguateCandidates_SelectsCorrectCandidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req geminiRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.NotEmpty(t, req.Contents)
		require.NotEmpty(t, req.Contents[0].Parts)
		prompt := req.Contents[0].Parts[0].Text

		assert.Contains(t, prompt, "Sherlock Holmes")
		assert.Contains(t, prompt, "Candidate 1")
		assert.Contains(t, prompt, "Candidate 2")
		assert.Contains(t, prompt, "100")
		assert.Contains(t, prompt, "200")

		_, _ = w.Write(geminiOKResponse(`{"matched_tmdb_id": 200, "confidence": "high", "reason": "Jeremy Brett 1984 series is the exact match"}`))
	}))
	defer server.Close()

	client := NewGeminiClient([]string{"test-key-1"})
	client.apiURL = server.URL

	candidates := []DisambiguationCandidate{
		{TMDBID: 100, Title: "Sherlock Holmes", Year: 1965, Overview: "1965 BBC series"},
		{TMDBID: 200, Title: "The Adventures of Sherlock Holmes", Year: 1984, Overview: "1984 Granada series"},
	}

	result, err := client.DisambiguateCandidates(
		context.Background(),
		PlexInfo{Title: "Sherlock Holmes", Year: 1984, Type: "series"},
		candidates,
		EpisodeHints{},
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(200), result.MatchedTMDBID)
	assert.Equal(t, "high", result.Confidence)
	assert.Contains(t, result.Reason, "1984")
}

func TestDisambiguateCandidates_WithEpisodeHints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req geminiRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.NotEmpty(t, req.Contents)
		prompt := req.Contents[0].Parts[0].Text

		assert.Contains(t, prompt, "Un scandale en Bohême")
		assert.Contains(t, prompt, "Season: 1, Episode: 1")
		assert.Contains(t, prompt, "tt0506445")

		_, _ = w.Write(geminiOKResponse(`{"matched_tmdb_id": 200, "confidence": "high", "reason": "S01E01 Un scandale en Bohême belongs to Granada 1984"}`))
	}))
	defer server.Close()

	client := NewGeminiClient([]string{"test-key-1"})
	client.apiURL = server.URL

	candidates := []DisambiguationCandidate{
		{TMDBID: 100, Title: "Sherlock Holmes", Year: 1965, Overview: "1965 BBC series"},
		{TMDBID: 200, Title: "The Adventures of Sherlock Holmes", Year: 1984, Overview: "1984 Granada series"},
	}
	hints := EpisodeHints{
		EpisodeName:   "Un scandale en Bohême",
		SeasonNumber:  1,
		EpisodeNumber: 1,
		EpisodeIMDBID: "tt0506445",
	}

	result, err := client.DisambiguateCandidates(
		context.Background(),
		PlexInfo{Title: "Sherlock Holmes", Year: 0, Type: "series"},
		candidates,
		hints,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(200), result.MatchedTMDBID)
	assert.Equal(t, "high", result.Confidence)
}

func TestVerifyMatch_WithEpisodeHints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req geminiRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.NotEmpty(t, req.Contents)
		prompt := req.Contents[0].Parts[0].Text

		assert.Contains(t, prompt, "A Scandal in Bohemia")
		assert.Contains(t, prompt, "Season: 1, Episode: 1")
		assert.Contains(t, prompt, "tt0506445")

		_, _ = w.Write(geminiOKResponse(`{"confirmed": true, "confidence": "high", "reason": "Episode confirms Granada series"}`))
	}))
	defer server.Close()

	client := NewGeminiClient([]string{"test-key-1"})
	client.apiURL = server.URL

	hints := EpisodeHints{
		EpisodeName:   "A Scandal in Bohemia",
		SeasonNumber:  1,
		EpisodeNumber: 1,
		EpisodeIMDBID: "tt0506445",
	}

	result, err := client.VerifyMatch(
		context.Background(),
		PlexInfo{Title: "Sherlock Holmes", Year: 1984, Type: "series"},
		MatchCandidate{Title: "The Adventures of Sherlock Holmes", Year: 1984, TMDBID: 200},
		hints,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Confirmed)
	assert.Equal(t, "high", result.Confidence)
}
