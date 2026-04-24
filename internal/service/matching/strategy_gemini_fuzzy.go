package matching

import (
	"context"
	"log"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// geminiFuzzyStrategy is the last-resort fallback: Gemini rewrites the Plex
// title into a canonical form which is then fed back into TMDB search. Matches
// are marked unconfirmed because the resolved title has not been independently
// verified.
type geminiFuzzyStrategy struct{ p *Pipeline }

func (s *geminiFuzzyStrategy) Name() string { return MatchSourceGeminiFuzzy }

func (s *geminiFuzzyStrategy) Try(ctx context.Context, input MatchInput) (*MatchResult, bool, error) {
	if s.p.gemini == nil || s.p.tmdb == nil || input.Title == "" {
		return nil, false, nil
	}
	resolution, err := s.p.gemini.FuzzyResolve(ctx, PlexInfo{
		Title: input.Title,
		Year:  input.Year,
		Type:  string(input.Type),
	})
	if err != nil {
		log.Printf("gemini fuzzy resolve failed: %v", err)
		return nil, false, nil
	}
	if resolution.CandidateTitle == "" {
		return nil, false, nil
	}

	result := newMatchResult(input)
	resolvedInput := MatchInput{
		Title: resolution.CandidateTitle,
		Year:  resolution.CandidateYear,
		Type:  input.Type,
	}
	if !s.p.searchTMDB(ctx, resolvedInput, result) {
		return nil, false, nil
	}
	result.MatchStatus = model.MatchStatusUnconfirmed
	result.MatchSource = MatchSourceGeminiFuzzy
	s.p.enrichFromIDs(ctx, result, input)
	return result, true, nil
}
