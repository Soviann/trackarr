package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type ExternalIDs struct {
	IMDB string
	TMDB int64
	TVDB int64
}

type JellyfinService struct {
	log      *slog.Logger
	db       *sql.DB
	pipeline *matching.Pipeline
	titleSvc *TitleService
	libSvc   *LibraryService
	source   model.WatchEventSource
}

func NewJellyfinService(db *sql.DB, pipeline *matching.Pipeline, titleSvc *TitleService, libSvc *LibraryService) *JellyfinService {
	return &JellyfinService{
		log:      slog.With("subsystem", "jellyfin"),
		db:       db,
		pipeline: pipeline,
		titleSvc: titleSvc,
		libSvc:   libSvc,
		source:   model.WatchEventSourceJellyfin,
	}
}

// ProcessJellyfinWebhook ingests a Jellyfin Webhook-plugin notification.
// It normalises the payload and, when the event represents a completed playback,
// processes the scrobble into the database.
func (s *JellyfinService) ProcessJellyfinWebhook(ctx context.Context, jf *model.JellyfinPayload, rawPayload string) error {
	if !strings.EqualFold(jf.NotificationType, "PlaybackStop") || !parseJellyfinBool(jf.PlayedToCompletion) {
		return nil
	}

	year := atoiSafe(jf.Year)
	ids := ExternalIDs{
		IMDB: strings.TrimSpace(jf.ProviderIMDB),
		TMDB: int64(atoiSafe(jf.ProviderTMDB)),
		TVDB: int64(atoiSafe(jf.ProviderTVDB)),
	}

	switch strings.ToLower(jf.ItemType) {
	case "movie":
		return s.processMovie(ctx, jf, year, ids, rawPayload)
	case "episode":
		return s.processEpisode(ctx, jf, year, ids, rawPayload)
	default:
		return nil
	}
}

func (s *JellyfinService) processMovie(ctx context.Context, jf *model.JellyfinPayload, year int, ids ExternalIDs, rawPayload string) error {
	logger := s.log.With("itemID", jf.ItemID, "title", jf.Name)
	var ratingPrompt *RatingPrompt

	if err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		prompt, err := s.processMovieInTx(ctx, tx, jf, year, ids, rawPayload, logger)
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

func (s *JellyfinService) processMovieInTx(ctx context.Context, tx *sql.Tx, jf *model.JellyfinPayload, year int, ids ExternalIDs, rawPayload string, logger *slog.Logger) (*RatingPrompt, error) {
	titles := repository.NewTitleRepository(tx)
	titlesW := repository.NewTitleWriter(tx)
	var imdbID *string
	var tmdbID *int64
	ratingKey := jf.ItemID

	if ids.IMDB != "" {
		imdbID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		tmdbID = &ids.TMDB
	}

	movieType := model.TitleTypeMovie
	title, err := titles.FindByExternalID(imdbID, tmdbID, &ratingKey, nil, &movieType)
	if err != nil {
		titleID, err := s.titleSvc.CreateFromScrobble(ctx, tx, jf.Name, year, ids, model.TitleTypeMovie, ratingKey, nil, model.TitleStatusCompleted)
		if err != nil {
			return nil, fmt.Errorf("create movie: %w", err)
		}
		logger = logger.With("titleID", titleID)
		logger.Info("created title from Jellyfin movie")

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
		s.enqueueEnrichmentTx(ctx, tx, title.ID, jf.Name, year, title.Type, ids, logger)
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

func (s *JellyfinService) processEpisode(ctx context.Context, jf *model.JellyfinPayload, year int, ids ExternalIDs, rawPayload string) error {
	seriesName := jf.SeriesName
	if seriesName == "" {
		seriesName = jf.Name
	}
	logger := s.log.With("seriesID", jf.SeriesID, "series", seriesName, "season", jf.Season, "episode", jf.Episode)

	var autoCompleteCheck *autoCompleteRequest
	var ratingPrompt *RatingPrompt

	if err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		req, prompt, err := s.processEpisodeInTx(ctx, tx, jf, seriesName, year, ids, rawPayload, logger)
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

type autoCompleteRequest struct {
	titleID    int64
	tmdbID     int64
	seasonNum  int
	episodeNum int
}

func (s *JellyfinService) processEpisodeInTx(ctx context.Context, tx *sql.Tx, jf *model.JellyfinPayload, seriesName string, year int, ids ExternalIDs, rawPayload string, logger *slog.Logger) (*autoCompleteRequest, *RatingPrompt, error) {
	titles := repository.NewTitleRepository(tx)
	titlesW := repository.NewTitleWriter(tx)
	seasons := repository.NewSeasonWriter(tx)
	episodes := repository.NewEpisodeWriter(tx)

	grandparentKey := jf.SeriesID
	seasonNum := atoiSafe(jf.Season)
	episodeNum := atoiSafe(jf.Episode)

	seriesType := model.TitleTypeSeries
	title, err := titles.FindByExternalID(nil, nil, &grandparentKey, nil, &seriesType)
	if err != nil {
		// For an episode scrobble, ids contains episode-level provider IDs.
		// Pass empty ExternalIDs so CreateFromScrobble uses seriesName + year in the
		// matching pipeline to resolve true series-level external IDs.
		titleID, createErr := s.titleSvc.CreateFromScrobble(ctx, tx, seriesName, year, ExternalIDs{}, model.TitleTypeSeries, grandparentKey, nil, model.TitleStatusWatching)
		if createErr != nil {
			return nil, nil, fmt.Errorf("create series: %w", createErr)
		}
		logger = logger.With("titleID", titleID)
		logger.Info("created title from Jellyfin episode")
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
			s.enqueueEnrichmentTx(ctx, tx, title.ID, seriesName, year, title.Type, ExternalIDs{}, logger)
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

func needsEnrichment(title *model.Title) bool {
	return title.TMDBID == nil && title.AniListID == nil
}

func (s *JellyfinService) enqueueEnrichmentTx(ctx context.Context, tx *sql.Tx, titleID int64, titleName string, year int, titleType model.TitleType, ids ExternalIDs, logger *slog.Logger) {
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

func parseJellyfinBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1":
		return true
	default:
		return false
	}
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
