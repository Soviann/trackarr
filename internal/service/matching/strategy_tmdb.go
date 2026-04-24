package matching

import "context"

// tmdbSearchStrategy runs a TMDB title search then routes the match through
// Gemini verification (via verifyAndEnrich) to avoid blindly trusting the first
// search hit.
type tmdbSearchStrategy struct{ p *Pipeline }

func (s *tmdbSearchStrategy) Name() string { return MatchSourceTMDBSearch }

func (s *tmdbSearchStrategy) Try(ctx context.Context, input MatchInput) (*MatchResult, bool, error) {
	if s.p.tmdb == nil || input.Title == "" {
		return nil, false, nil
	}
	result := newMatchResult(input)
	if !s.p.searchTMDB(ctx, input, result) {
		return nil, false, nil
	}
	result.MatchSource = MatchSourceTMDBSearch
	final, err := s.p.verifyAndEnrich(ctx, input, result)
	if err != nil {
		return nil, false, err
	}
	return final, true, nil
}
