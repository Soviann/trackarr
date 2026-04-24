package matching

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTMDBStrategy_NilClient_NotMatched(t *testing.T) {
	s := &tmdbSearchStrategy{p: NewPipeline(nil, nil, nil, nil, t.TempDir())}

	_, matched, err := s.Try(context.Background(), MatchInput{Title: "X", Type: model.TitleTypeMovie})
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestTMDBStrategy_EmptyTitle_NotMatched(t *testing.T) {
	tmdbClient := NewTMDBClient("key")
	s := &tmdbSearchStrategy{p: NewPipeline(tmdbClient, nil, nil, nil, t.TempDir())}

	_, matched, err := s.Try(context.Background(), MatchInput{Type: model.TitleTypeMovie})
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestTMDBStrategy_SearchEmpty_NotMatched(t *testing.T) {
	tmdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tmdbSearchResponse{})
	}))
	defer tmdbServer.Close()

	tmdbClient := NewTMDBClient("key")
	tmdbClient.baseURL = tmdbServer.URL

	s := &tmdbSearchStrategy{p: NewPipeline(tmdbClient, nil, nil, nil, t.TempDir())}

	_, matched, err := s.Try(context.Background(), MatchInput{Title: "Unknown", Type: model.TitleTypeMovie})
	require.NoError(t, err)
	assert.False(t, matched)
}
