package matching

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrossRefStrategy_NilDB_NotMatched(t *testing.T) {
	s := &crossRefStrategy{p: NewPipeline(nil, nil, nil, nil, t.TempDir())}

	_, matched, err := s.Try(context.Background(), MatchInput{TVDBID: 85004})
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestCrossRefStrategy_NoLookupHit_NotMatched(t *testing.T) {
	dataDir := t.TempDir()
	crossrefPath := filepath.Join(dataDir, "crossref.json")
	require.NoError(t, os.WriteFile(crossrefPath, []byte(`{"data": []}`), 0o644))
	crossDB, err := LoadCrossRefDB(crossrefPath)
	require.NoError(t, err)

	s := &crossRefStrategy{p: NewPipeline(nil, nil, nil, crossDB, dataDir)}

	_, matched, err := s.Try(context.Background(), MatchInput{TVDBID: 999999})
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestCrossRefStrategy_LookupResolvesTMDB_Matched(t *testing.T) {
	dataDir := t.TempDir()
	crossrefJSON := `{"data": [{"sources": ["https://www.themoviedb.org/tv/46298", "https://www.thetvdb.com/series/85004"], "title": "One Piece", "type": "TV"}]}`
	crossrefPath := filepath.Join(dataDir, "crossref.json")
	require.NoError(t, os.WriteFile(crossrefPath, []byte(crossrefJSON), 0o644))
	crossDB, err := LoadCrossRefDB(crossrefPath)
	require.NoError(t, err)

	// nil TMDB → enrichFromIDs is a no-op for remote fetches but strategy still matches
	s := &crossRefStrategy{p: NewPipeline(nil, nil, nil, crossDB, dataDir)}

	result, matched, err := s.Try(context.Background(), MatchInput{
		TVDBID: 85004,
		Type:   model.TitleTypeSeries,
	})
	require.NoError(t, err)
	assert.True(t, matched)
	require.NotNil(t, result)
	assert.Equal(t, MatchSourceCrossRef, result.MatchSource)
	assert.Equal(t, model.MatchStatusConfirmed, result.MatchStatus)
	assert.Equal(t, int64(46298), result.TMDBID)
}
