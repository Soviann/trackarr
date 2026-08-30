package service

import (
	"context"
	"database/sql"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

func (s *BackgroundService) refreshTVDBRelations(
	ctx context.Context,
	title *repository.TitleLite,
	result *RefreshResult,
) {
	if s.tvdb == nil || title == nil || title.TVDBID == nil {
		return
	}

	extended, err := s.tvdb.GetSeriesDetails(ctx, *title.TVDBID)
	if err != nil {
		log.Printf("background: tvdb get series extended %d for title %d: %v", *title.TVDBID, title.ID, err)
		return
	}
	if extended == nil || len(extended.Lists) == 0 {
		return
	}

	var relations []model.TitleRelation
	seenSeries := make(map[int64]bool)
	seenMovies := make(map[int64]bool)

	for _, listBrief := range extended.Lists {
		// Only process lists that explicitly represent a franchise or universe
		nameLower := strings.ToLower(listBrief.Name)
		isFranchiseList := strings.Contains(nameLower, "franchise") ||
			strings.Contains(nameLower, "universe") ||
			strings.Contains(nameLower, "univers") ||
			strings.Contains(nameLower, "saga")

		if !isFranchiseList {
			continue
		}

		listDetail, err := s.tvdb.GetListExtended(ctx, listBrief.ID)
		if err != nil {
			log.Printf("background: tvdb get list %d for title %d: %v", listBrief.ID, title.ID, err)
			continue
		}
		if listDetail == nil || len(listDetail.Entities) == 0 {
			continue
		}

		for _, entity := range listDetail.Entities {
			if entity.SeriesID != nil {
				sID := *entity.SeriesID
				if sID == *title.TVDBID || seenSeries[sID] {
					continue
				}
				seenSeries[sID] = true

				sDetails, err := s.tvdb.GetSeriesDetails(ctx, sID)
				if err != nil || sDetails == nil {
					continue
				}

				var year *int
				if len(sDetails.Year) >= 4 {
					if y, err := strconv.Atoi(sDetails.Year[:4]); err == nil && y > 0 {
						year = &y
					}
				}

				var coverURL *string
				if sDetails.Image != "" {
					coverURL = &sDetails.Image
				}

				var score *int
				if sDetails.Score > 0 {
					sc := int(math.Round(sDetails.Score * 10))
					score = &sc
				}

				var overview *string
				if sDetails.Overview != "" {
					overview = &sDetails.Overview
				}

				relations = append(relations, model.TitleRelation{
					TitleID:      title.ID,
					Provider:     "tvdb",
					ExternalID:   sID,
					RelationType: model.RelationSpinOff,
					Format:       "TV",
					Title:        sDetails.Name,
					CoverURL:     coverURL,
					Year:         year,
					Score:        score,
					Overview:     overview,
					SortOrder:    entity.Order,
				})
			} else if entity.MovieID != nil {
				mID := *entity.MovieID
				if seenMovies[mID] {
					continue
				}
				seenMovies[mID] = true

				mDetails, err := s.tvdb.GetMovieDetails(ctx, mID)
				if err != nil || mDetails == nil {
					continue
				}

				var year *int
				if len(mDetails.Year) >= 4 {
					if y, err := strconv.Atoi(mDetails.Year[:4]); err == nil && y > 0 {
						year = &y
					}
				}

				var coverURL *string
				if mDetails.Image != "" {
					coverURL = &mDetails.Image
				}

				var score *int
				if mDetails.Score > 0 {
					sc := int(math.Round(mDetails.Score * 10))
					score = &sc
				}

				var overview *string
				if mDetails.Overview != "" {
					overview = &mDetails.Overview
				}

				relations = append(relations, model.TitleRelation{
					TitleID:      title.ID,
					Provider:     "tvdb",
					ExternalID:   mID,
					RelationType: model.RelationSideStory,
					Format:       "MOVIE",
					Title:        mDetails.Name,
					CoverURL:     coverURL,
					Year:         year,
					Score:        score,
					Overview:     overview,
					SortOrder:    entity.Order,
				})
			}
		}
	}

	if len(relations) == 0 {
		return
	}

	err = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewTitleRelationWriter(tx).UpsertBatch(ctx, title.ID, relations)
	})
	if err != nil {
		log.Printf("background: upsert tvdb relations for title %d: %v", title.ID, err)
		return
	}

	result.Refreshed = true
}
