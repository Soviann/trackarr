package matching

import (
	"context"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// MatchStrategy is one step of the matching pipeline. Each strategy produces
// a fully-formed MatchResult when it matches; the first match wins.
type MatchStrategy interface {
	// Name returns the MatchSource constant this strategy sets on success.
	Name() string
	// Try returns (result, matched, err). matched=true halts the chain with result;
	// matched=false lets the next strategy run (err should be nil). A non-nil error
	// aborts the whole pipeline.
	Try(ctx context.Context, input MatchInput) (*MatchResult, bool, error)
}

// newMatchResult seeds a result with the IDs and type already known from the
// Plex input. Strategies build on this base and enrich as needed.
func newMatchResult(input MatchInput) *MatchResult {
	return &MatchResult{
		IMDBID:    input.IMDBID,
		TMDBID:    input.TMDBID,
		TVDBID:    input.TVDBID,
		AniListID: input.AniListID,
		TitleType: input.Type,
		IsAnime:   input.IsAnime,
	}
}

// unmatchedResult is the fallback returned by the pipeline when every strategy
// passes. Keeps the input title as a primary English name so UIs still show
// something meaningful.
func unmatchedResult(input MatchInput) *MatchResult {
	result := newMatchResult(input)
	result.MatchStatus = model.MatchStatusUnconfirmed
	result.MatchSource = MatchSourceNone
	if result.TitleType == "" {
		result.TitleType = model.TitleTypeMovie
	}
	if input.Title != "" {
		result.Names = []model.TitleName{{Name: input.Title, Language: "en", IsPrimary: true}}
	}
	return result
}
