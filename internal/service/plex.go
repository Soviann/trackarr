package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	plexwebhooks "github.com/hekmon/plexwebhooks"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type PlexExternalIDs struct {
	IMDB string
	TMDB int64
	TVDB int64
}

func ParseGUIDs(guids []*url.URL) PlexExternalIDs {
	var ids PlexExternalIDs
	for _, g := range guids {
		if g == nil {
			continue
		}
		raw := g.String()
		switch {
		case strings.HasPrefix(raw, "imdb://"):
			ids.IMDB = strings.TrimPrefix(raw, "imdb://")
		case strings.HasPrefix(raw, "tmdb://"):
			_, _ = fmt.Sscanf(strings.TrimPrefix(raw, "tmdb://"), "%d", &ids.TMDB)
		case strings.HasPrefix(raw, "tvdb://"):
			_, _ = fmt.Sscanf(strings.TrimPrefix(raw, "tvdb://"), "%d", &ids.TVDB)
		}
	}
	return ids
}

type PlexService struct {
	log      *slog.Logger
	db       *sql.DB
	pipeline *matching.Pipeline // nil = skip matching, create with basic info
	titleSvc *TitleService
	libSvc   *LibraryService
}

func NewPlexService(db *sql.DB, pipeline *matching.Pipeline, titleSvc *TitleService, libSvc *LibraryService) *PlexService {
	return &PlexService{
		log:      slog.With("subsystem", "plex"),
		db:       db,
		pipeline: pipeline,
		titleSvc: titleSvc,
		libSvc:   libSvc,
	}
}

// ProcessWebhook handles all inbound Plex webhook events, routing by event type.
// ctx should carry the HTTP request deadline so the write transaction is aborted
// if the client disconnects (preventing the sole write connection from being
// held indefinitely when a downstream I/O call blocks).
func (s *PlexService) ProcessWebhook(ctx context.Context, payload *plexwebhooks.Payload, rawPayload string) error {
	meta := payload.Metadata
	logger := s.log.With("ratingKey", meta.RatingKey)

	logger.Info("webhook received",
		"event", payload.Event,
		"mediaType", meta.Type,
		"title", meta.Title,
		"season", meta.ParentIndex,
		"episode", meta.Index,
	)

	switch payload.Event {
	case plexwebhooks.EventTypeScrobble:
		return s.handleScrobble(ctx, payload, rawPayload, logger)
	case plexwebhooks.EventTypePlay:
		return s.handlePlay(ctx, payload, rawPayload, logger)
	default:
		return nil
	}
}

func (s *PlexService) handleScrobble(ctx context.Context, payload *plexwebhooks.Payload, rawPayload string, logger *slog.Logger) error {
	meta := payload.Metadata
	var ids PlexExternalIDs
	if meta.Type == plexwebhooks.MediaTypeMovie {
		ids = ParseGUIDs(meta.GUIDExternal)
	}

	switch meta.Type {
	case plexwebhooks.MediaTypeMovie:
		return s.processMovie(ctx, meta, ids, rawPayload, logger)
	case plexwebhooks.MediaTypeEpisode:
		return s.processEpisode(ctx, meta, ids, rawPayload, logger)
	default:
		return fmt.Errorf("unknown media type: %s", meta.Type)
	}
}

// handlePlay handles media.play events. Only processes already-watched episodes
// (rewatches) — first-time watches wait for the media.scrobble event.
func (s *PlexService) handlePlay(ctx context.Context, payload *plexwebhooks.Payload, rawPayload string, logger *slog.Logger) error {
	// Movies always get a media.scrobble from Plex; only episodes need rewatch tracking.
	if payload.Metadata.Type != plexwebhooks.MediaTypeEpisode {
		return nil
	}
	return database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		return s.handleEpisodePlayInTx(ctx, tx, payload.Metadata, rawPayload, logger)
	})
}

func (s *PlexService) handleEpisodePlayInTx(ctx context.Context, tx *sql.Tx, meta plexwebhooks.Metadata, rawPayload string, logger *slog.Logger) error {
	titles := repository.NewTitleRepository(tx)
	titlesW := repository.NewTitleWriter(tx)
	seasons := repository.NewSeasonWriter(tx)
	episodes := repository.NewEpisodeWriter(tx)
	events := repository.NewWatchEventWriter(tx)

	grandparentKey := meta.GrandparentRatingKey
	title, err := titles.FindByExternalID(nil, nil, &grandparentKey, nil, nil)
	if err != nil {
		// Not a tracked title — nothing to do.
		return nil
	}
	logger = logger.With("titleID", title.ID)

	season, err := seasons.GetOrCreate(ctx, title.ID, meta.ParentIndex)
	if err != nil {
		return fmt.Errorf("get/create season: %w", err)
	}

	ep, err := episodes.GetOrCreate(ctx, season.ID, meta.Index)
	if err != nil {
		return fmt.Errorf("get/create episode: %w", err)
	}

	now := time.Now().UTC()

	// media.play on unwatched episode — catch-up (media.scrobble missed).
	if !ep.Watched {
		if err := episodes.MarkWatched(ctx, ep.ID, now); err != nil {
			return fmt.Errorf("mark episode watched: %w", err)
		}
	}

	if _, err := events.Create(ctx, &model.WatchEvent{
		TitleID:     title.ID,
		EpisodeID:   &ep.ID,
		Source:      model.WatchEventSourcePlex,
		PlexPayload: &rawPayload,
	}); err != nil {
		logger.Warn("create watch event", "episodeID", ep.ID, "err", err)
	}

	if err := episodes.UpdateLastWatchedAt(ctx, ep.ID, now); err != nil {
		logger.Warn("update episode last_watched_at", "episodeID", ep.ID, "err", err)
	}

	if err := titlesW.UpdateLastWatchedAt(ctx, title.ID, now); err != nil {
		logger.Warn("update title last_watched_at", "err", err)
	}

	return nil
}

func (s *PlexService) processMovie(ctx context.Context, meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string, logger *slog.Logger) error {
	// ratingPrompt is populated inside the transaction and, if non-nil, is fired
	// AFTER commit. Keeps the webpush HTTP request out of the write transaction
	// so a slow push endpoint cannot tie up the sole writeDB connection.
	var ratingPrompt *RatingPrompt

	if err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		prompt, err := s.processMovieInTx(ctx, tx, meta, ids, rawPayload, logger)
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

func (s *PlexService) processMovieInTx(ctx context.Context, tx *sql.Tx, meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string, logger *slog.Logger) (*RatingPrompt, error) {
	titles := repository.NewTitleRepository(tx)
	titlesW := repository.NewTitleWriter(tx)
	var imdbID *string
	var tmdbID *int64
	ratingKey := meta.RatingKey

	if ids.IMDB != "" {
		imdbID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		tmdbID = &ids.TMDB
	}

	movieType := model.TitleTypeMovie
	title, err := titles.FindByExternalID(imdbID, tmdbID, &ratingKey, nil, &movieType)
	if err != nil {
		titleID, err := s.titleSvc.CreateFromPlex(ctx, tx, meta.Title, meta.Year, ids, model.TitleTypeMovie, ratingKey, meta.GUIDExternal, model.TitleStatusCompleted)
		if err != nil {
			return nil, fmt.Errorf("create movie: %w", err)
		}
		logger = logger.With("titleID", titleID)
		logger.Info("created title from Plex movie")

		prompt, err := s.libSvc.MarkMovieWatched(ctx, tx, titleID, model.WatchEventSourcePlex, &rawPayload)
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
		s.enqueueEnrichmentTx(ctx, tx, title.ID, meta.Title, meta.Year, title.Type, ids, logger)
	}

	prompt, err := s.libSvc.MarkMovieWatched(ctx, tx, title.ID, model.WatchEventSourcePlex, &rawPayload)
	if err != nil {
		return nil, err
	}
	if err := titlesW.UpdateLastWatchedAt(ctx, title.ID, time.Now().UTC()); err != nil {
		return nil, err
	}
	return prompt, nil
}

func (s *PlexService) processEpisode(ctx context.Context, meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string, logger *slog.Logger) error {
	// Both autoCompleteCheck and ratingPrompt are populated inside the
	// transaction and, if non-nil, executed AFTER commit. This keeps the TMDB
	// HTTP request (CheckAutoComplete) and the webpush HTTP request out of the
	// write transaction so slow external I/O cannot tie up the sole writeDB
	// connection.
	var autoCompleteCheck *autoCompleteRequest
	var ratingPrompt *RatingPrompt

	if err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		req, prompt, err := s.processEpisodeInTx(ctx, tx, meta, ids, rawPayload, logger)
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

// autoCompleteRequest captures the inputs needed to run CheckAutoComplete
// outside the write transaction.
type autoCompleteRequest struct {
	titleID    int64
	tmdbID     int64
	seasonNum  int
	episodeNum int
}

func (s *PlexService) processEpisodeInTx(ctx context.Context, tx *sql.Tx, meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string, logger *slog.Logger) (*autoCompleteRequest, *RatingPrompt, error) {
	titles := repository.NewTitleRepository(tx)
	titlesW := repository.NewTitleWriter(tx)
	seasons := repository.NewSeasonWriter(tx)
	episodes := repository.NewEpisodeWriter(tx)

	grandparentKey := meta.GrandparentRatingKey
	var imdbID *string
	var tmdbID *int64

	if ids.IMDB != "" {
		imdbID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		tmdbID = &ids.TMDB
	}

	title, err := titles.FindByExternalID(imdbID, tmdbID, &grandparentKey, nil, nil)
	if err != nil {
		seriesName := meta.GrandparentTitle
		if seriesName == "" {
			seriesName = meta.Title
		}

		titleID, createErr := s.titleSvc.CreateFromPlex(ctx, tx, seriesName, meta.Year, ids, model.TitleTypeSeries, grandparentKey, meta.GUIDExternal, model.TitleStatusWatching)
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
			seriesName := meta.GrandparentTitle
			if seriesName == "" {
				seriesName = meta.Title
			}
			s.enqueueEnrichmentTx(ctx, tx, title.ID, seriesName, meta.Year, title.Type, ids, logger)
		}
	}

	season, err := seasons.GetOrCreate(ctx, title.ID, meta.ParentIndex)
	if err != nil {
		return nil, nil, fmt.Errorf("get/create season: %w", err)
	}

	ep, err := episodes.GetOrCreate(ctx, season.ID, meta.Index)
	if err != nil {
		return nil, nil, fmt.Errorf("get/create episode: %w", err)
	}

	_, prompt, err := s.libSvc.MarkEpisodesWatched(ctx, tx, title.ID, []int64{ep.ID}, []int64{ep.SeasonID}, model.WatchEventSourcePlex, &rawPayload)
	if err != nil {
		return nil, nil, err
	}

	if err := titlesW.UpdateLastWatchedAt(ctx, title.ID, time.Now().UTC()); err != nil {
		logger.Warn("update title last_watched_at", "err", err)
	}

	// Auto-complete check runs AFTER the transaction commits (see processEpisode).
	// Keeping the TMDB HTTP call out of the transaction prevents a hung network
	// request from holding the sole write connection.
	backfillTMDBID := title.TMDBID
	if backfillTMDBID == nil {
		backfillTMDBID = &ids.TMDB
	}
	if backfillTMDBID != nil && *backfillTMDBID != 0 {
		return &autoCompleteRequest{
			titleID:    title.ID,
			tmdbID:     *backfillTMDBID,
			seasonNum:  meta.ParentIndex,
			episodeNum: meta.Index,
		}, prompt, nil
	}

	return nil, prompt, nil
}

// checkSeriesCompleted checks if the given season/episode is the last episode
// of the last season of an ended or cancelled series (via TMDB).
func checkSeriesCompleted(ctx context.Context, tmdb *matching.TMDBClient, tmdbID int64, seasonNum, episodeNum int) (bool, *model.SeriesStatus) {
	details, err := tmdb.GetTVDetails(ctx, tmdbID)
	if err != nil {
		return false, nil
	}

	// Check series status — only auto-complete for ended/cancelled
	seriesStatus := mapTMDBSeriesStatus(details)
	if seriesStatus == nil {
		return false, nil
	}
	if *seriesStatus != model.SeriesStatusEnded && *seriesStatus != model.SeriesStatusCancelled {
		return false, nil
	}

	// Find the last season (highest number, excluding specials S00)
	lastSeasonNum := 0
	lastSeasonEpisodeCount := 0
	for _, s := range details.Seasons {
		if s.SeasonNumber == 0 {
			continue
		}
		if s.SeasonNumber > lastSeasonNum {
			lastSeasonNum = s.SeasonNumber
			lastSeasonEpisodeCount = s.EpisodeCount
		}
	}

	if lastSeasonNum == 0 || lastSeasonEpisodeCount == 0 {
		return false, nil
	}

	return seasonNum == lastSeasonNum && episodeNum == lastSeasonEpisodeCount, seriesStatus
}

// needsEnrichment returns true if a title lacks enrichment data.
func needsEnrichment(title *model.Title) bool {
	return title.TMDBID == nil && title.AniListID == nil
}

// enqueueEnrichmentTx inserts an enrichment task into the queue on the given
// transaction. Replaces the previous goroutine-per-webhook "trigger" pattern
// which spawned unbounded goroutines during large Plex scans: the task queue
// already enforces a shared rate limiter, exponential retry and panic recovery,
// and SQLite's single-writer model means the rafale of webhooks becomes a
// backlog rather than hundreds of concurrent TMDB/AniList/Gemini calls.
func (s *PlexService) enqueueEnrichmentTx(ctx context.Context, tx *sql.Tx, titleID int64, titleName string, year int, titleType model.TitleType, ids PlexExternalIDs, logger *slog.Logger) {
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
