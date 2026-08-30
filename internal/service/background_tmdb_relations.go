package service

import (
	"context"
	"database/sql"
	"log"
	"math"
	"sort"
	"strconv"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
)

func (s *BackgroundService) refreshTMDBMovieCollection(
	ctx context.Context,
	title *repository.TitleLite,
	movieDetails *matching.TMDBMovieDetails,
	result *RefreshResult,
) {
	if s.tmdb == nil || title == nil || movieDetails == nil || movieDetails.BelongsToCollection == nil {
		return
	}

	collectionID := movieDetails.BelongsToCollection.ID
	if collectionID <= 0 {
		return
	}

	collection, err := s.tmdb.GetMovieCollection(ctx, collectionID, "")
	if err != nil {
		log.Printf("background: tmdb get movie collection %d for title %d: %v", collectionID, title.ID, err)
		return
	}
	if collection == nil || len(collection.Parts) == 0 {
		return
	}

	// Sort parts chronologically by release_date
	sort.Slice(collection.Parts, func(i, j int) bool {
		return collection.Parts[i].ReleaseDate < collection.Parts[j].ReleaseDate
	})

	var currentMovieYear *int
	if len(movieDetails.ReleaseDate) >= 4 {
		if y, err := strconv.Atoi(movieDetails.ReleaseDate[:4]); err == nil && y > 0 {
			currentMovieYear = &y
		}
	}

	var relations []model.TitleRelation
	for idx, part := range collection.Parts {
		if part.ID == movieDetails.ID {
			continue
		}

		var year *int
		if len(part.ReleaseDate) >= 4 {
			if y, err := strconv.Atoi(part.ReleaseDate[:4]); err == nil && y > 0 {
				year = &y
			}
		}

		var coverURL *string
		if part.PosterPath != nil && *part.PosterPath != "" {
			u := "https://image.tmdb.org/t/p/w500" + *part.PosterPath
			coverURL = &u
		}

		var score *int
		if part.VoteAverage > 0 {
			sc := int(math.Round(part.VoteAverage * 10))
			score = &sc
		}

		var overview *string
		if part.Overview != "" {
			overview = &part.Overview
		}

		relType := model.RelationCollection
		if year != nil && currentMovieYear != nil {
			if *year < *currentMovieYear {
				relType = model.RelationPrequel
			} else if *year > *currentMovieYear {
				relType = model.RelationSequel
			}
		}

		relations = append(relations, model.TitleRelation{
			TitleID:      title.ID,
			Provider:     "tmdb",
			ExternalID:   part.ID,
			RelationType: relType,
			Format:       "MOVIE",
			Title:        part.Title,
			CoverURL:     coverURL,
			Year:         year,
			Score:        score,
			Overview:     overview,
			SortOrder:    idx,
		})
	}

	if len(relations) == 0 {
		return
	}

	err = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewTitleRelationWriter(tx).UpsertBatch(ctx, title.ID, relations)
	})
	if err != nil {
		log.Printf("background: upsert tmdb relations for title %d: %v", title.ID, err)
		return
	}

	result.Refreshed = true
}
