package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
)

type ExternalIDs struct {
	IMDB string
	TMDB int64
	TVDB int64
}

type JellyfinService struct {
	log         *slog.Logger
	db          *sql.DB
	pipeline    *matching.Pipeline
	titleSvc    *TitleService
	libSvc      *LibraryService
	playbackHub *PlaybackHubService
	source      model.WatchEventSource
}

func NewJellyfinService(db *sql.DB, pipeline *matching.Pipeline, titleSvc *TitleService, libSvc *LibraryService, playbackHub *PlaybackHubService) *JellyfinService {
	return &JellyfinService{
		log:         slog.With("subsystem", "jellyfin"),
		db:          db,
		pipeline:    pipeline,
		titleSvc:    titleSvc,
		libSvc:      libSvc,
		playbackHub: playbackHub,
		source:      model.WatchEventSourceJellyfin,
	}
}

// ProcessJellyfinWebhook ingests a Jellyfin Webhook-plugin notification.
// PlaybackStart events register the active playback session in memory.
// PlaybackStop events clear the active session and, if played to completion, process the scrobble.
// UserDataSaved events sync user ratings; ItemAdded events track newly added movies/series.
func (s *JellyfinService) ProcessJellyfinWebhook(ctx context.Context, jf *model.JellyfinPayload, rawPayload string) error {
	if jf == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(jf.NotificationType)) {
	case "playbackstart":
		return s.processPlaybackStart(ctx, jf)
	case "playbackstop":
		if s.playbackHub != nil {
			s.playbackHub.ClearSession("jellyfin", resolveJellyfinPlayer(jf))
		}
		if !parseJellyfinBool(jf.PlayedToCompletion) {
			return nil
		}
		// proceed with completion scrobble
	case "userdatasaved":
		return s.processUserDataSaved(ctx, jf)
	case "itemadded":
		return s.processItemAdded(ctx, jf)
	default:
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

func (s *JellyfinService) processPlaybackStart(ctx context.Context, jf *model.JellyfinPayload) error {
	if s.playbackHub == nil {
		return nil
	}

	year := atoiSafe(jf.Year)
	ids := ExternalIDs{
		IMDB: strings.TrimSpace(jf.ProviderIMDB),
		TMDB: int64(atoiSafe(jf.ProviderTMDB)),
		TVDB: int64(atoiSafe(jf.ProviderTVDB)),
	}

	titles := repository.NewTitleRepository(s.db)

	switch strings.ToLower(jf.ItemType) {
	case "movie":
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
		title, err := titles.FindByExternalID(imdbID, tmdbID, &ratingKey, nil, nil, &movieType)
		if err != nil {
			if strings.TrimSpace(jf.Name) != "" {
				title, err = titles.FindByNameAndType(jf.Name, model.TitleTypeMovie, year)
			}
			if err != nil {
				if s.titleSvc == nil || strings.TrimSpace(jf.Name) == "" {
					return nil
				}
				var createdID int64
				if txErr := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
					var cErr error
					createdID, cErr = s.titleSvc.CreateFromScrobble(ctx, tx, jf.Name, year, ids, model.TitleTypeMovie, ratingKey, nil, model.TitleStatusWatching)
					return cErr
				}); txErr != nil {
					s.log.Warn("failed to auto-create movie on playback start", "title", jf.Name, "err", txErr)
					return nil
				}
				title, err = titles.GetByID(createdID)
				if err != nil || title == nil {
					return nil
				}
			}
		}

		titleName := jf.Name
		if titleName == "" {
			titleName = title.PrimaryName()
		}
		for _, n := range title.Names {
			if n.IsPrimary {
				titleName = n.Name
				break
			}
		}

		overview := strings.TrimSpace(jf.Overview)
		if overview == "" && title.Overview != nil {
			overview = *title.Overview
		}

		session := model.ActivePlaybackSession{
			Source:        "jellyfin",
			Player:        resolveJellyfinPlayer(jf),
			TitleID:       title.ID,
			TitleName:     titleName,
			TitleType:     model.TitleTypeMovie,
			Year:          title.Year,
			CoverURL:      title.CoverURL,
			AccentHex:     title.AccentHex,
			Overview:      overview,
			DurationSec:   parseJellyfinTicksToSeconds(jf.RunTimeTicks),
			ViewOffsetSec: parseJellyfinTicksToSeconds(jf.PlaybackPositionTicks),
		}
		s.playbackHub.SetSession(session)
		return nil

	case "episode":
		seriesName := jf.SeriesName
		if seriesName == "" {
			seriesName = jf.Name
		}
		seriesID := jf.SeriesID
		seasonNum := atoiSafe(jf.Season)
		episodeNum := atoiSafe(jf.Episode)

		seriesType := model.TitleTypeSeries
		title, err := titles.FindByExternalID(nil, nil, &seriesID, nil, nil, &seriesType)
		if err != nil {
			if strings.TrimSpace(seriesName) != "" {
				title, err = titles.FindByNameAndType(seriesName, model.TitleTypeSeries, year)
			}
			if err != nil {
				if s.titleSvc == nil || strings.TrimSpace(seriesName) == "" {
					return nil
				}
				epHints := matching.EpisodeHints{
					EpisodeName:   jf.Name,
					SeasonNumber:  seasonNum,
					EpisodeNumber: episodeNum,
					EpisodeIMDBID: ids.IMDB,
				}
				var createdID int64
				if txErr := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
					var cErr error
					createdID, cErr = s.titleSvc.CreateFromScrobble(ctx, tx, seriesName, year, ExternalIDs{}, model.TitleTypeSeries, seriesID, nil, model.TitleStatusWatching, epHints)
					if cErr != nil {
						return cErr
					}
					if seasonNum > 0 && episodeNum > 0 {
						seasonWriter := repository.NewSeasonWriter(tx)
						epWriter := repository.NewEpisodeWriter(tx)
						if season, sErr := seasonWriter.GetOrCreate(ctx, createdID, seasonNum); sErr == nil {
							_, _ = epWriter.GetOrCreate(ctx, season.ID, episodeNum)
						}
					}
					return nil
				}); txErr != nil {
					s.log.Warn("failed to auto-create series on playback start", "series", seriesName, "err", txErr)
					return nil
				}
				title, err = titles.GetByID(createdID)
				if err != nil || title == nil {
					return nil
				}
			}
		}

		titleName := seriesName
		if titleName == "" {
			titleName = title.PrimaryName()
		}
		for _, n := range title.Names {
			if n.IsPrimary {
				titleName = n.Name
				break
			}
		}
		epName := jf.Name
		overview := strings.TrimSpace(jf.Overview)
		if overview == "" && title.Overview != nil {
			overview = *title.Overview
		}

		session := model.ActivePlaybackSession{
			Source:        "jellyfin",
			Player:        resolveJellyfinPlayer(jf),
			TitleID:       title.ID,
			TitleName:     titleName,
			TitleType:     model.TitleTypeSeries,
			Year:          title.Year,
			CoverURL:      title.CoverURL,
			AccentHex:     title.AccentHex,
			SeasonNumber:  &seasonNum,
			EpisodeNumber: &episodeNum,
			EpisodeName:   &epName,
			Overview:      overview,
			DurationSec:   parseJellyfinTicksToSeconds(jf.RunTimeTicks),
			ViewOffsetSec: parseJellyfinTicksToSeconds(jf.PlaybackPositionTicks),
		}
		s.playbackHub.SetSession(session)
		return nil

	default:
		return nil
	}
}

func (s *JellyfinService) processUserDataSaved(ctx context.Context, jf *model.JellyfinPayload) error {
	var titleType model.TitleType
	switch strings.ToLower(strings.TrimSpace(jf.ItemType)) {
	case "movie":
		titleType = model.TitleTypeMovie
	case "series":
		titleType = model.TitleTypeSeries
	default:
		return nil
	}

	rating, ok := parseJellyfinRating(jf.Rating)
	if !ok {
		return nil
	}

	ids := ExternalIDs{
		IMDB: strings.TrimSpace(jf.ProviderIMDB),
		TMDB: int64(atoiSafe(jf.ProviderTMDB)),
		TVDB: int64(atoiSafe(jf.ProviderTVDB)),
	}
	if ids.IMDB == "" && ids.TMDB == 0 && ids.TVDB == 0 {
		return nil
	}

	var imdbPtr *string
	if ids.IMDB != "" {
		imdbPtr = &ids.IMDB
	}
	var tmdbPtr *int64
	if ids.TMDB != 0 {
		tmdbPtr = &ids.TMDB
	}
	var tvdbPtr *int64
	if ids.TVDB != 0 {
		tvdbPtr = &ids.TVDB
	}

	titles := repository.NewTitleRepository(s.db)
	title, err := titles.FindByExternalID(imdbPtr, tmdbPtr, nil, nil, tvdbPtr, &titleType)
	if err != nil {
		s.log.Debug("title not found for Jellyfin rating webhook",
			"itemType", jf.ItemType,
			"name", jf.Name,
			"imdb", ids.IMDB,
			"tmdb", ids.TMDB,
			"tvdb", ids.TVDB,
			"err", err,
		)
		return nil
	}

	if title.MyRating != nil && *title.MyRating == rating {
		s.log.Debug("skipping duplicate Jellyfin rating update", "titleID", title.ID, "rating", rating)
		return nil
	}

	if s.titleSvc == nil {
		return nil
	}

	if err := s.titleSvc.Update(ctx, title.ID, repository.TitleUpdate{MyRating: &rating}, title, nil, &rating, false); err != nil {
		return fmt.Errorf("update title rating from Jellyfin: %w", err)
	}

	s.log.Info("updated rating from Jellyfin webhook", "titleID", title.ID, "title", title.PrimaryName(), "rating", rating)
	return nil
}

func (s *JellyfinService) processItemAdded(ctx context.Context, jf *model.JellyfinPayload) error {
	var titleType model.TitleType
	switch strings.ToLower(strings.TrimSpace(jf.ItemType)) {
	case "movie":
		titleType = model.TitleTypeMovie
	case "series":
		titleType = model.TitleTypeSeries
	default:
		return nil
	}

	if s.db == nil {
		return fmt.Errorf("jellyfin item added: database is not initialized")
	}

	name := strings.TrimSpace(jf.Name)
	if name == "" && titleType == model.TitleTypeSeries {
		name = strings.TrimSpace(jf.SeriesName)
	}

	ids := parseJellyfinWebhookExternalIDs(jf)
	titleName := resolveJellyfinItemAddedName(name, ids)
	if titleName == "" {
		return nil
	}

	year := clampJellyfinYear(atoiSafe(jf.Year))
	itemID := strings.TrimSpace(jf.ItemID)

	created, err := createJellyfinTitleIfMissing(ctx, s.db, jellyfinTitleImport{
		TitleType:        titleType,
		TitleName:        titleName,
		OriginalName:     name,
		Year:             year,
		IDs:              ids,
		ExternalSourceID: itemID,
		MatchSource:      "jellyfin_webhook",
		LookupTVDB:       true,
	})
	if err != nil {
		return fmt.Errorf("jellyfin item added: %w", err)
	}
	if created {
		s.log.Info("created title from Jellyfin ItemAdded webhook",
			"type", titleType,
			"title", titleName,
			"year", year,
			"itemID", itemID,
		)
	}
	return nil
}

type jellyfinTitleImport struct {
	TitleType        model.TitleType
	TitleName        string
	OriginalName     string
	Year             int
	IDs              ExternalIDs
	ExternalSourceID string
	MatchSource      string
	LookupTVDB       bool
}

func parseJellyfinWebhookExternalIDs(jf *model.JellyfinPayload) ExternalIDs {
	var ids ExternalIDs
	if imdb := strings.ToLower(strings.TrimSpace(jf.ProviderIMDB)); imdb != "" {
		ids.IMDB = imdb
	}
	if v, err := strconv.ParseInt(strings.TrimSpace(jf.ProviderTMDB), 10, 64); err == nil && v > 0 {
		ids.TMDB = v
	}
	if v, err := strconv.ParseInt(strings.TrimSpace(jf.ProviderTVDB), 10, 64); err == nil && v > 0 {
		ids.TVDB = v
	}
	return ids
}

func resolveJellyfinItemAddedName(name string, ids ExternalIDs) string {
	if name != "" {
		return name
	}
	switch {
	case ids.TMDB > 0:
		return fmt.Sprintf("TMDB %d", ids.TMDB)
	case ids.TVDB > 0:
		return fmt.Sprintf("TVDB %d", ids.TVDB)
	case ids.IMDB != "":
		return ids.IMDB
	default:
		return ""
	}
}

func optionalJellyfinIDPointers(ids ExternalIDs, externalSourceID string) (imdbPtr *string, tmdbPtr *int64, tvdbPtr *int64, extIDPtr *string) {
	if ids.IMDB != "" {
		imdb := ids.IMDB
		imdbPtr = &imdb
	}
	if ids.TMDB > 0 {
		tmdb := ids.TMDB
		tmdbPtr = &tmdb
	}
	if ids.TVDB > 0 {
		tvdb := ids.TVDB
		tvdbPtr = &tvdb
	}
	if externalSourceID != "" {
		ext := externalSourceID
		extIDPtr = &ext
	}
	return imdbPtr, tmdbPtr, tvdbPtr, extIDPtr
}

func findExistingJellyfinTitle(
	titles *repository.TitleRepository,
	titleType model.TitleType,
	imdbPtr *string,
	tmdbPtr *int64,
	tvdbPtr *int64,
	extIDPtr *string,
	name string,
	year int,
) (*model.Title, error) {
	if imdbPtr != nil || tmdbPtr != nil || tvdbPtr != nil || extIDPtr != nil {
		existing, err := titles.FindByExternalID(imdbPtr, tmdbPtr, extIDPtr, nil, tvdbPtr, &titleType)
		if err == nil && existing != nil {
			return existing, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("lookup %s by external id: %w", titleType, err)
		}
	}
	if name != "" {
		existing, err := titles.FindByNameAndType(name, titleType, year)
		if err == nil && existing != nil {
			return existing, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("lookup %s by name and type: %w", titleType, err)
		}
	}
	return nil, nil
}

func createJellyfinTitleIfMissing(ctx context.Context, db *sql.DB, item jellyfinTitleImport) (bool, error) {
	imdbPtr, tmdbPtr, tvdbPtr, extIDPtr := optionalJellyfinIDPointers(item.IDs, item.ExternalSourceID)
	var lookupTVDBPtr *int64
	if item.LookupTVDB {
		lookupTVDBPtr = tvdbPtr
	}

	var created bool
	now := time.Now().UTC()
	err := database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		txTitles := repository.NewTitleRepository(tx)
		dup, err := findExistingJellyfinTitle(txTitles, item.TitleType, imdbPtr, tmdbPtr, lookupTVDBPtr, extIDPtr, item.OriginalName, item.Year)
		if err != nil {
			return err
		}
		if dup != nil {
			return nil
		}

		matchStatus := model.MatchStatusPendingReview
		if tmdbPtr != nil && *tmdbPtr > 0 {
			matchStatus = model.MatchStatusConfirmed
		}
		matchSource := item.MatchSource
		var origTitle *string
		if item.OriginalName != "" {
			orig := item.OriginalName
			origTitle = &orig
		}

		title := &model.Title{
			Type:             item.TitleType,
			Year:             item.Year,
			Status:           model.TitleStatusPlanToWatch,
			ExternalSourceID: extIDPtr,
			IMDBID:           imdbPtr,
			TMDBID:           tmdbPtr,
			TVDBID:           tvdbPtr,
			MatchStatus:      matchStatus,
			MatchSource:      &matchSource,
			OriginalTitle:    origTitle,
			ArrIgnored:       true,
		}
		names := []model.TitleName{{
			Name:      item.TitleName,
			Language:  "en",
			IsPrimary: true,
		}}

		titleWriter := repository.NewTitleWriter(tx)
		titleID, err := titleWriter.Create(ctx, title, names)
		if err != nil {
			return fmt.Errorf("create title: %w", err)
		}
		if item.TitleType == model.TitleTypeMovie {
			if err := titleWriter.MarkDownloaded(ctx, titleID, now); err != nil {
				return fmt.Errorf("mark title downloaded: %w", err)
			}
		}

		payloadJSON, err := json.Marshal(EnrichmentPayload{
			TitleID:       titleID,
			TitleName:     item.TitleName,
			Year:          item.Year,
			TitleType:     item.TitleType,
			IMDBID:        item.IDs.IMDB,
			TMDBID:        item.IDs.TMDB,
			TVDBID:        item.IDs.TVDB,
			PreserveMatch: false,
		})
		if err != nil {
			return fmt.Errorf("marshal enrichment payload: %w", err)
		}
		dedupKey := fmt.Sprintf("enrichment:%d", titleID)
		if _, err := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeEnrichment, string(payloadJSON), &dedupKey); err != nil {
			return fmt.Errorf("enqueue enrichment: %w", err)
		}

		created = true
		return nil
	})
	return created, err
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
	title, err := titles.FindByExternalID(imdbID, tmdbID, &ratingKey, nil, nil, &movieType)
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
	title, err := titles.FindByExternalID(nil, nil, &grandparentKey, nil, nil, &seriesType)
	if err != nil {
		// For an episode scrobble, ids contains episode-level provider IDs.
		// Pass empty ExternalIDs so CreateFromScrobble uses seriesName + year in the
		// matching pipeline to resolve true series-level external IDs.
		epHints := matching.EpisodeHints{
			EpisodeName:   jf.Name,
			SeasonNumber:  seasonNum,
			EpisodeNumber: episodeNum,
			EpisodeIMDBID: ids.IMDB,
		}
		titleID, createErr := s.titleSvc.CreateFromScrobble(ctx, tx, seriesName, year, ExternalIDs{}, model.TitleTypeSeries, grandparentKey, nil, model.TitleStatusWatching, epHints)
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

func resolveJellyfinPlayer(jf *model.JellyfinPayload) string {
	if name := strings.TrimSpace(jf.DeviceName); name != "" {
		return name
	}
	if client := strings.TrimSpace(jf.ClientName); client != "" {
		return client
	}
	return "Jellyfin"
}

func parseJellyfinRating(raw string) (int, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	val, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(val) || math.IsInf(val, 0) || val <= 0 {
		return 0, false
	}
	rounded := int(math.Round(val))
	if rounded < 1 {
		return 0, false
	}
	if rounded > 10 {
		rounded = 10
	}
	return rounded, true
}

// ParseJellyfinRating parses a Jellyfin rating string into a 1–10 integer rating.
func ParseJellyfinRating(raw string) (int, bool) {
	return parseJellyfinRating(raw)
}

func parseJellyfinTicksToSeconds(raw string) int64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	ticks, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || ticks <= 0 {
		return 0
	}
	return ticks / 10_000_000
}
