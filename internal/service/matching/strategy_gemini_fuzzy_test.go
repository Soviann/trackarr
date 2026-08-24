package matching

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiFuzzyStrategy_NilGemini_NotMatched(t *testing.T) {
	tmdb := NewTMDBClient("key")
	s := &geminiFuzzyStrategy{p: NewPipeline(tmdb, nil, nil, nil, t.TempDir())}

	_, matched, err := s.Try(context.Background(), MatchInput{Title: "X", Type: model.TitleTypeMovie})
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestGeminiFuzzyStrategy_NilTMDB_NotMatched(t *testing.T) {
	gemini := NewGeminiClient([]string{"key"})
	s := &geminiFuzzyStrategy{p: NewPipeline(nil, nil, gemini, nil, t.TempDir())}

	_, matched, err := s.Try(context.Background(), MatchInput{Title: "X", Type: model.TitleTypeMovie})
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestGeminiFuzzyStrategy_EmptyCandidate_NotMatched(t *testing.T) {
	tmdb := NewTMDBClient("key")
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(geminiOKResponse(`{"candidate_title": "", "candidate_year": 0, "confidence": "low", "reason": "unknown"}`))
	}))
	defer geminiServer.Close()

	gemini := NewGeminiClient([]string{"key"})
	gemini.apiURL = geminiServer.URL

	s := &geminiFuzzyStrategy{p: NewPipeline(tmdb, nil, gemini, nil, t.TempDir())}

	_, matched, err := s.Try(context.Background(), MatchInput{Title: "Garbage", Type: model.TitleTypeMovie})
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestGeminiFuzzyStrategy_FuzzyErrorSwallowed_NotMatched(t *testing.T) {
	tmdb := NewTMDBClient("key")
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer geminiServer.Close()

	gemini := NewGeminiClient([]string{"key"})
	gemini.apiURL = geminiServer.URL

	s := &geminiFuzzyStrategy{p: NewPipeline(tmdb, nil, gemini, nil, t.TempDir())}

	// Gemini error must not abort the pipeline — strategy logs and yields to next step.
	_, matched, err := s.Try(context.Background(), MatchInput{Title: "X", Type: model.TitleTypeMovie})
	require.NoError(t, err)
	assert.False(t, matched)
}
