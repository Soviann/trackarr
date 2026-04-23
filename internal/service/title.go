package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type TitleService struct {
	db       *sql.DB
	titles   *repository.TitleRepository
	tasks    *repository.TaskRepository
	pipeline *matching.Pipeline
}

func NewTitleService(db *sql.DB, titles *repository.TitleRepository, tasks *repository.TaskRepository, pipeline *matching.Pipeline) *TitleService {
	return &TitleService{db: db, titles: titles, tasks: tasks, pipeline: pipeline}
}

// CreateFromPlex constructs a new title from Plex metadata and starts matching.
// Caller owns the transaction so the create (or the "existing title found"
// update-in-place branch) shares atomicity with the surrounding webhook work.
func (s *TitleService) CreateFromPlex(ctx context.Context, tx *sql.Tx, title string, year int, ids PlexExternalIDs, titleType model.TitleType, ratingKey string, guids []*url.URL, status model.TitleStatus) (int64, error) {
	titles := repository.NewTitleRepository(tx)
	writer := repository.NewTitleWriter(tx)
	t := &model.Title{
		Type:          titleType,
		Year:          year,
		PlexRatingKey: &ratingKey,
		Status:        status,
	}

	var names []model.TitleName

	if s.pipeline != nil {
		result, err := s.pipeline.Run(ctx, matching.MatchInput{
			Title:  title,
			Year:   year,
			Type:   titleType,
			IMDBID: ids.IMDB,
			TMDBID: ids.TMDB,
			TVDBID: ids.TVDB,
		})
		if err == nil {
			t.MatchStatus = result.MatchStatus
			t.MatchSource = &result.MatchSource
			t.OriginalTitle = &title
			t.Type = result.TitleType
			t.IsAnime = result.IsAnime
			if result.IMDBID != "" {
				t.IMDBID = &result.IMDBID
			}
			if result.TMDBID != 0 {
				t.TMDBID = &result.TMDBID
			}
			if result.TVDBID != 0 {
				t.TVDBID = &result.TVDBID
			}
			if result.AniListID != 0 {
				t.AniListID = &result.AniListID
			}
			if result.CoverFile != "" {
				coverURL := "/covers/" + result.CoverFile
				t.CoverURL = &coverURL
			}
			names = result.Names

			// Title already exists under these external IDs: update the Plex key
			// and return the existing ID so scrobbles converge on one record.
			existing, err := titles.FindByExternalID(t.IMDBID, t.TMDBID, nil, t.AniListID, &t.Type)
			if err == nil && existing != nil {
				update := repository.TitleUpdate{
					PlexRatingKey: t.PlexRatingKey,
					Type:          &t.Type,
					IsAnime:       &t.IsAnime,
				}
				if err := writer.Update(ctx, existing.ID, update); err != nil {
					return 0, fmt.Errorf("update existing title with plex key: %w", err)
				}
				return existing.ID, nil
			}
		}
	}

	if len(names) == 0 {
		// Fallback: no pipeline or pipeline error
		t.MatchStatus = model.MatchStatusConfirmed
		t.OriginalTitle = &title
		fallbackSource := matching.MatchSourcePlexIDs
		t.MatchSource = &fallbackSource
		if ids.IMDB != "" {
			t.IMDBID = &ids.IMDB
		}
		if ids.TMDB != 0 {
			t.TMDBID = &ids.TMDB
		}
		if ids.TVDB != 0 {
			t.TVDBID = &ids.TVDB
		}
		names = []model.TitleName{{Name: title, Language: "en", IsPrimary: true}}
	}

	return writer.Create(ctx, t, names)
}

// Rematch updates a title's external IDs and enqueues an enrichment task. The
// service owns the transaction because handlers call it with the pool handle.
func (s *TitleService) Rematch(ctx context.Context, db *sql.DB, id int64, imdbID *string, tmdbID *int64, anilistID *int64, tvdbID *int64) error {
	title, err := s.titles.GetByID(id)
	if err != nil {
		return err
	}

	matchStatus := model.MatchStatusConfirmed
	matchSource := matching.MatchSourceManual
	update := repository.TitleUpdate{
		MatchStatus: &matchStatus,
		MatchSource: &matchSource,
	}
	if tmdbID != nil {
		update.TMDBID = tmdbID
	}
	if imdbID != nil {
		update.IMDBID = imdbID
	}
	if anilistID != nil {
		update.AniListID = anilistID
	}
	if tvdbID != nil {
		update.TVDBID = tvdbID
	}

	if err := database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(ctx, id, update)
	}); err != nil {
		return err
	}

	// Enqueue enrichment task
	payloadTMDB := int64(0)
	if tmdbID != nil {
		payloadTMDB = *tmdbID
	} else if title.TMDBID != nil {
		payloadTMDB = *title.TMDBID
	}
	payloadIMDB := ""
	if imdbID != nil {
		payloadIMDB = *imdbID
	} else if title.IMDBID != nil {
		payloadIMDB = *title.IMDBID
	}
	payloadTVDB := int64(0)
	if tvdbID != nil {
		payloadTVDB = *tvdbID
	} else if title.TVDBID != nil {
		payloadTVDB = *title.TVDBID
	}

	payload, err := json.Marshal(EnrichmentPayload{
		TitleID:   id,
		TitleName: title.PrimaryName(),
		Year:      title.Year,
		TitleType: title.Type,
		IMDBID:    payloadIMDB,
		TMDBID:    payloadTMDB,
		TVDBID:    payloadTVDB,
	})
	if err != nil {
		return fmt.Errorf("marshal enrichment payload: %w", err)
	}
	dedupKey := fmt.Sprintf("enrichment:%d", id)
	_, err = s.tasks.Enqueue(model.TaskTypeEnrichment, string(payload), &dedupKey)
	return err
}

// ResolveURL identifies a title from an external URL.
func (s *TitleService) ResolveURL(ctx context.Context, url string) (*matching.MatchResult, error) {
	if s.pipeline == nil {
		return nil, fmt.Errorf("matching pipeline not available")
	}
	return s.pipeline.ResolveURL(ctx, url)
}

// Merge consolidates sourceID into destID. db must be the pool handle (*sql.DB),
// never a *sql.Tx — the method opens its own transaction via
// database.WithTxContext so a cancelled ctx aborts the write immediately.
func (s *TitleService) Merge(ctx context.Context, db *sql.DB, destID, sourceID int64, explicitOffset *int) error {
	source, err := s.titles.GetByID(sourceID)
	if err != nil {
		return err
	}

	seasonOffset := 0
	if explicitOffset != nil {
		seasonOffset = *explicitOffset
	} else if source.IsAnime && s.pipeline != nil {
		name := source.PrimaryName()
		if ident, err := s.pipeline.IdentifyAnimeSeason(ctx, name, source.Year); err == nil && ident.IsSeason {
			log.Printf("fusion: Gemini identified sequel season %d for %q", ident.SeasonNumber, name)
			seasonOffset = ident.SeasonNumber - 1
		} else if err != nil {
			log.Printf("fusion: Gemini season identification failed for %q: %v", name, err)
		}
	}

	return database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Merge(ctx, destID, sourceID, seasonOffset)
	})
}
