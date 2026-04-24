package matching

import "context"

// aniListSearchStrategy searches AniList (anime-only) and runs Gemini
// verification before accepting the first hit.
type aniListSearchStrategy struct{ p *Pipeline }

func (s *aniListSearchStrategy) Name() string { return MatchSourceAniListSearch }

func (s *aniListSearchStrategy) Try(ctx context.Context, input MatchInput) (*MatchResult, bool, error) {
	if s.p.anilist == nil || input.Title == "" {
		return nil, false, nil
	}
	result := newMatchResult(input)
	if !s.p.searchAniList(ctx, input, result) {
		return nil, false, nil
	}
	result.MatchSource = MatchSourceAniListSearch
	final, err := s.p.verifyAndEnrich(ctx, input, result)
	if err != nil {
		return nil, false, err
	}
	return final, true, nil
}
