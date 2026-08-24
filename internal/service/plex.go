package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
)

type PlexService struct {
	log      *slog.Logger
	db       *sql.DB
	pipeline *matching.Pipeline
	titleSvc *TitleService
	libSvc   *LibraryService
	source   model.WatchEventSource
}

func NewPlexService(db *sql.DB, pipeline *matching.Pipeline, titleSvc *TitleService, libSvc *LibraryService) *PlexService {
	return &PlexService{
		log:      slog.With("subsystem", "plex"),
		db:       db,
		pipeline: pipeline,
		titleSvc: titleSvc,
		libSvc:   libSvc,
		source:   model.WatchEventSourcePlex,
	}
}

// ProcessPlexWebhook ingests a native Plex webhook payload.
// Only media.scrobble events (triggered at 90%+ playback completion) are processed.
func (s *PlexService) ProcessPlexWebhook(ctx context.Context, p *model.PlexPayload, rawPayload string) error {
	if !strings.EqualFold(p.Event, "media.scrobble") {
		return nil
	}

	imdb, tmdb, tvdb := p.Metadata.ExtractExternalIDs()
	ids := ExternalIDs{
		IMDB: imdb,
		TMDB: tmdb,
		TVDB: tvdb,
	}

	switch strings.ToLower(p.Metadata.Type) {
	case "movie":
		return s.processMovie(ctx, p, ids, rawPayload)
	case "episode":
		return s.processEpisode(ctx, p, ids, rawPayload)
	default:
		return nil
	}
}

func (s *PlexService) processMovie(ctx context.Context, p *model.PlexPayload, ids ExternalIDs, rawPayload string) error {
	logger := s.log.With("ratingKey", p.Metadata.RatingKey, "title", p.Metadata.Title)
	var ratingPrompt *RatingPrompt

	if err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		prompt, err := s.processMovieInTx(ctx, tx, p, ids, rawPayload, logger)
		if err != nil {
			return err
		}
		ratingPrompt = prompt
		return nil
	}); err != nil {
		return err
	}

	s.libSvc.SendRatingPrompt(ctx, ratingPrompt)
	return nil
}

func (s *PlexService) processMovieInTx(ctx context.Context, tx *sql.Tx, p *model.PlexPayload, ids ExternalIDs, rawPayload string, logger *slog.Logger) (*RatingPrompt, error) {
	titles := repository.NewTitleRepository(tx)
	titlesW := repository.NewTitleWriter(tx)
	var imdbID *string
	var tmdbID *int64
	ratingKey := p.Metadata.RatingKey

	if ids.IMDB != "" {
		imdbID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		tmdbID = &ids.TMDB
	}

	movieType := model.TitleTypeMovie
	title, err := titles.FindByExternalID(imdbID, tmdbID, &ratingKey, nil, nil, &movieType)
	if err != nil {
		titleID, err := s.titleSvc.CreateFromScrobble(ctx, tx, p.Metadata.Title, p.Metadata.Year, ids, model.TitleTypeMovie, ratingKey, nil, model.TitleStatusCompleted)
		if err != nil {
			return nil, fmt.Errorf("create movie: %w", err)
		}
		logger = logger.With("titleID", titleID)
		logger.Info("created title from Plex movie")

		prompt, err := s.libSvc.MarkMovieWatched(ctx, tx, titleID, s.source, &rawPayload)
		if err != nil {
			return nil, err
		}
		if err := titlesW.UpdateLastWatchedAt(ctx, titleID, time.Now().UTC()); err != nil {
			return nil, err
		}
		return prompt, nil
	}
	logger = logger.With("titleID", title.ID)

	if needsEnrichment(title) {
		s.enqueueEnrichmentTx(ctx, tx, title.ID, p.Metadata.Title, p.Metadata.Year, title.Type, ids, logger)
	}

	prompt, err := s.libSvc.MarkMovieWatched(ctx, tx, title.ID, s.source, &rawPayload)
	if err != nil {
		return nil, err
	}
	if err := titlesW.UpdateLastWatchedAt(ctx, title.ID, time.Now().UTC()); err != nil {
		return nil, err
	}
	return prompt, nil
}

func (s *PlexService) processEpisode(ctx context.Context, p *model.PlexPayload, ids ExternalIDs, rawPayload string) error {
	seriesName := p.Metadata.GrandparentTitle
	if seriesName == "" {
		seriesName = p.Metadata.Title
	}
	logger := s.log.With("grandparentKey", p.Metadata.GrandparentRatingKey, "series", seriesName, "season", p.Metadata.ParentIndex, "episode", p.Metadata.Index)

	var autoCompleteCheck *autoCompleteRequest
	var ratingPrompt *RatingPrompt

	if err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		req, prompt, err := s.processEpisodeInTx(ctx, tx, p, seriesName, ids, rawPayload, logger)
		if err != nil {
			return err
		}
		autoCompleteCheck = req
		ratingPrompt = prompt
		return nil
	}); err != nil {
		return err
	}

	s.libSvc.SendRatingPrompt(ctx, ratingPrompt)

	if autoCompleteCheck != nil {
		if err := s.libSvc.CheckAutoComplete(ctx, s.db, autoCompleteCheck.titleID, autoCompleteCheck.tmdbID, autoCompleteCheck.seasonNum, autoCompleteCheck.episodeNum); err != nil {
			logger.Warn("auto-complete check", "titleID", autoCompleteCheck.titleID, "err", err)
		}
	}
	return nil
}

func (s *PlexService) processEpisodeInTx(ctx context.Context, tx *sql.Tx, p *model.PlexPayload, seriesName string, ids ExternalIDs, rawPayload string, logger *slog.Logger) (*autoCompleteRequest, *RatingPrompt, error) {
	titles := repository.NewTitleRepository(tx)
	titlesW := repository.NewTitleWriter(tx)
	seasons := repository.NewSeasonWriter(tx)
	episodes := repository.NewEpisodeWriter(tx)

	grandparentKey := p.Metadata.GrandparentRatingKey
	seasonNum := p.Metadata.ParentIndex
	episodeNum := p.Metadata.Index

	seriesType := model.TitleTypeSeries
	title, err := titles.FindByExternalID(nil, nil, &grandparentKey, nil, nil, &seriesType)
	if err != nil {
		titleID, createErr := s.titleSvc.CreateFromScrobble(ctx, tx, seriesName, p.Metadata.Year, ExternalIDs{}, model.TitleTypeSeries, grandparentKey, nil, model.TitleStatusWatching)
		if createErr != nil {
			return nil, nil, fmt.Errorf("create series: %w", createErr)
		}
		logger = logger.With("titleID", titleID)
		logger.Info("created title from Plex episode")
		title = &model.Title{ID: titleID, Status: model.TitleStatusWatching}
	} else {
		logger = logger.With("titleID", title.ID)
		if title.Status != model.TitleStatusCompleted && title.Status != model.TitleStatusWatching {
			watchingStatus := model.TitleStatusWatching
			if updateErr := titlesW.Update(ctx, title.ID, repository.TitleUpdate{Status: &watchingStatus}); updateErr != nil {
				logger.Warn("update status to watching", "err", updateErr)
			}
		}
		if needsEnrichment(title) {
			s.enqueueEnrichmentTx(ctx, tx, title.ID, seriesName, p.Metadata.Year, title.Type, ExternalIDs{}, logger)
		}
	}

	season, err := seasons.GetOrCreate(ctx, title.ID, seasonNum)
	if err != nil {
		return nil, nil, fmt.Errorf("get/create season: %w", err)
	}

	ep, err := episodes.GetOrCreate(ctx, season.ID, episodeNum)
	if err != nil {
		return nil, nil, fmt.Errorf("get/create episode: %w", err)
	}

	_, prompt, err := s.libSvc.MarkEpisodesWatched(ctx, tx, title.ID, []int64{ep.ID}, []int64{ep.SeasonID}, s.source, &rawPayload)
	if err != nil {
		return nil, nil, err
	}

	if err := titlesW.UpdateLastWatchedAt(ctx, title.ID, time.Now().UTC()); err != nil {
		logger.Warn("update title last_watched_at", "err", err)
	}

	backfillTMDBID := title.TMDBID
	if backfillTMDBID == nil {
		backfillTMDBID = &ids.TMDB
	}
	if backfillTMDBID != nil && *backfillTMDBID != 0 {
		return &autoCompleteRequest{
			titleID:    title.ID,
			tmdbID:     *backfillTMDBID,
			seasonNum:  seasonNum,
			episodeNum: episodeNum,
		}, prompt, nil
	}

	return nil, prompt, nil
}

func (s *PlexService) enqueueEnrichmentTx(ctx context.Context, tx *sql.Tx, titleID int64, titleName string, year int, titleType model.TitleType, ids ExternalIDs, logger *slog.Logger) {
	if s.pipeline == nil {
		return
	}
	payload, err := json.Marshal(EnrichmentPayload{
		TitleID:   titleID,
		TitleName: titleName,
		Year:      year,
		TitleType: titleType,
		IMDBID:    ids.IMDB,
		TMDBID:    ids.TMDB,
		TVDBID:    ids.TVDB,
	})
	if err != nil {
		logger.Error("marshal enrichment payload", "err", err)
		return
	}
	dedupKey := fmt.Sprintf("enrichment:%d", titleID)
	if _, err := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeEnrichment, string(payload), &dedupKey); err != nil {
		logger.Error("enqueue enrichment", "err", err)
	}
	logger.Info("enrichment enqueued")
}
