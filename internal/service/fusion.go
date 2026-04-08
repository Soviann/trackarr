package service

import (
	"context"
	"log"

	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

// MergeTitles consolidates sourceID into destID.
// It also handles season offset calculation for anime if a pipeline is provided.
func MergeTitles(ctx context.Context, titles *repository.TitleRepository, pipeline *matching.Pipeline, destID, sourceID int64) error {
	source, err := titles.GetByID(sourceID)
	if err != nil {
		return err
	}

	seasonOffset := 0
	if source.IsAnime && pipeline != nil {
		// Try to identify if the source is actually a sequel season of the destination
		name := ""
		if source.OriginalTitle != nil && *source.OriginalTitle != "" {
			name = *source.OriginalTitle
		} else if len(source.Names) > 0 {
			for _, n := range source.Names {
				if n.IsPrimary {
					name = n.Name
					break
				}
			}
			if name == "" {
				name = source.Names[0].Name
			}
		}

		if ident, err := pipeline.IdentifyAnimeSeason(name, source.Year); err == nil && ident.IsSeason {
			log.Printf("fusion: Gemini identified sequel season %d for %q", ident.SeasonNumber, name)
			// If Gemini says this is Season 2, we want its "Season 1" to become Season 2 of the destination
			seasonOffset = ident.SeasonNumber - 1
		} else if err != nil {
			log.Printf("fusion: Gemini season identification failed for %q: %v", name, err)
		}
	}

	return titles.Merge(destID, sourceID, seasonOffset)
}
