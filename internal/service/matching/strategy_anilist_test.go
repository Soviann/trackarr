package matching

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAniListStrategy_NilClient_NotMatched(t *testing.T) {
	s := &aniListSearchStrategy{p: NewPipeline(nil, nil, nil, nil, t.TempDir())}

	_, matched, err := s.Try(context.Background(), MatchInput{Title: "X", Type: model.TitleTypeSeries})
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestAniListStrategy_EmptyTitle_NotMatched(t *testing.T) {
	anilist := NewAniListClient()
	s := &aniListSearchStrategy{p: NewPipeline(nil, anilist, nil, nil, t.TempDir())}

	_, matched, err := s.Try(context.Background(), MatchInput{Type: model.TitleTypeSeries})
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestAniListStrategy_SearchEmpty_NotMatched(t *testing.T) {
	anilistServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"Page": map[string]any{"media": []any{}},
			},
		})
	}))
	defer anilistServer.Close()

	anilist := NewAniListClient()
	anilist.apiURL = anilistServer.URL

	s := &aniListSearchStrategy{p: NewPipeline(nil, anilist, nil, nil, t.TempDir())}

	_, matched, err := s.Try(context.Background(), MatchInput{Title: "Unknown", Type: model.TitleTypeSeries})
	require.NoError(t, err)
	assert.False(t, matched)
}
