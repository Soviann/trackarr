package matching

import (
	"context"

	"github.com/Soviann/trackarr/internal/model"
)

// crossRefStrategy resolves missing IDs via the local cross-reference database
// (AniList/TMDB/TVDB/IMDB mapping) before falling back to remote searches.
type crossRefStrategy struct{ p *Pipeline }

func (s *crossRefStrategy) Name() string { return MatchSourceCrossRef }

func (s *crossRefStrategy) Try(ctx context.Context, input MatchInput) (*MatchResult, bool, error) {
	if s.p.crossDB == nil {
		return nil, false, nil
	}
	result := newMatchResult(input)
	crossIDs := s.p.crossDB.Lookup(ExternalIDs{
		IMDB:      result.IMDBID,
		TMDBMovie: result.TMDBID,
		TMDBTV:    result.TMDBID,
		TVDB:      result.TVDBID,
	})
	if crossIDs == nil {
		return nil, false, nil
	}
	mergeIDs(result, crossIDs)
	if result.TMDBID == 0 && result.IMDBID == "" {
		return nil, false, nil
	}
	result.MatchStatus = model.MatchStatusConfirmed
	result.MatchSource = MatchSourceCrossRef
	s.p.enrichFromIDs(ctx, result, input)
	return result, true, nil
}
