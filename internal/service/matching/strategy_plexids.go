package matching

import (
	"context"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// plexIDStrategy confirms a title as soon as any strong external ID is already
// present in the Plex payload. Enrichment pulls metadata from TMDB/TVDB/AniList.
type plexIDStrategy struct{ p *Pipeline }

func (s *plexIDStrategy) Name() string { return MatchSourcePlexIDs }

func (s *plexIDStrategy) Try(ctx context.Context, input MatchInput) (*MatchResult, bool, error) {
	if input.TMDBID == 0 && input.IMDBID == "" && input.AniListID == 0 {
		return nil, false, nil
	}
	result := newMatchResult(input)
	result.MatchStatus = model.MatchStatusConfirmed
	result.MatchSource = MatchSourcePlexIDs
	s.p.enrichFromIDs(ctx, result, input)
	return result, true, nil
}
