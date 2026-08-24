package matching

import (
	"context"
	"testing"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlexIDStrategy_NoIDs_NotMatched(t *testing.T) {
	s := &plexIDStrategy{p: NewPipeline(nil, nil, nil, nil, t.TempDir())}

	result, matched, err := s.Try(context.Background(), MatchInput{
		Title: "No IDs",
		Type:  model.TitleTypeMovie,
	})
	require.NoError(t, err)
	assert.False(t, matched)
	assert.Nil(t, result)
}

func TestPlexIDStrategy_TMDBID_Matched(t *testing.T) {
	s := &plexIDStrategy{p: NewPipeline(nil, nil, nil, nil, t.TempDir())}

	result, matched, err := s.Try(context.Background(), MatchInput{
		Title:  "Fight Club",
		TMDBID: 550,
		Type:   model.TitleTypeMovie,
	})
	require.NoError(t, err)
	assert.True(t, matched)
	require.NotNil(t, result)
	assert.Equal(t, MatchSourcePlexIDs, result.MatchSource)
	assert.Equal(t, model.MatchStatusConfirmed, result.MatchStatus)
	assert.Equal(t, int64(550), result.TMDBID)
}

func TestPlexIDStrategy_AniListIDOnly_Matched(t *testing.T) {
	s := &plexIDStrategy{p: NewPipeline(nil, nil, nil, nil, t.TempDir())}

	result, matched, err := s.Try(context.Background(), MatchInput{
		AniListID: 21,
		Type:      model.TitleTypeSeries,
	})
	require.NoError(t, err)
	assert.True(t, matched)
	require.NotNil(t, result)
	assert.Equal(t, MatchSourcePlexIDs, result.MatchSource)
	assert.Equal(t, int64(21), result.AniListID)
}
