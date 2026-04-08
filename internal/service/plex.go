package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"

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
	db       *sql.DB
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
	tasks    *repository.TaskRepository
	settings *repository.SettingRepository
	pipeline *matching.Pipeline // nil = skip matching, create with basic info
	push     PushNotifier
	titleSvc *TitleService
	libSvc   *LibraryService
}

func NewPlexService(db *sql.DB, titles *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, tasks *repository.TaskRepository, settings *repository.SettingRepository, pipeline *matching.Pipeline, push PushNotifier, titleSvc *TitleService, libSvc *LibraryService) *PlexService {
	return &PlexService{
		db:       db,
		titles:   titles,
		seasons:  seasons,
		episodes: episodes,
		events:   events,
		tasks:    tasks,
		settings: settings,
		pipeline: pipeline,
		push:     push,
		titleSvc: titleSvc,
		libSvc:   libSvc,
	}
}

func (s *PlexService) ProcessScrobble(payload *plexwebhooks.Payload, rawPayload string) error {
	if payload.Event != plexwebhooks.EventTypeScrobble {
		return nil
	}

	meta := payload.Metadata
	ids := ParseGUIDs(meta.GUIDExternal)

	switch meta.Type {
	case plexwebhooks.MediaTypeMovie:
		return s.processMovie(meta, ids, rawPayload)
	case plexwebhooks.MediaTypeEpisode:
		return s.processEpisode(meta, ids, rawPayload)
	default:
		return fmt.Errorf("unknown media type: %s", meta.Type)
	}
}

func (s *PlexService) processMovie(meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string) error {
	// Check notification preference before the transaction to avoid SQLite deadlock (MaxOpenConns=1).
	ratingNotifEnabled := IsNotificationEnabled(s.settings, NotifRatingPrompt)
	return database.WithTx(s.db, func(tx *sql.Tx) error {
		return s.processMovieInTx(tx, meta, ids, rawPayload, ratingNotifEnabled)
	})
}

func (s *PlexService) processMovieInTx(tx *sql.Tx, meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string, ratingNotifEnabled bool) error {
	titles := repository.NewTitleRepository(tx)
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
		titleID, err := s.titleSvc.CreateFromPlex(tx, meta.Title, meta.Year, ids, model.TitleTypeMovie, ratingKey, meta.GUIDExternal, model.TitleStatusCompleted)
		if err != nil {
			return fmt.Errorf("create movie: %w", err)
		}

		// Use LibraryService to mark as watched + notify
		return s.libSvc.MarkMovieWatched(tx, titleID, model.WatchEventSourcePlex, &rawPayload)
	}

	if needsEnrichment(title) {
		s.triggerAsyncEnrichment(title.ID, meta.Title, meta.Year, title.Type, ids)
	}

	// Use LibraryService to mark as watched + notify
	return s.libSvc.MarkMovieWatched(tx, title.ID, model.WatchEventSourcePlex, &rawPayload)
}

func (s *PlexService) processEpisode(meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string) error {
	return database.WithTx(s.db, func(tx *sql.Tx) error {
		return s.processEpisodeInTx(tx, meta, ids, rawPayload)
	})
}

func (s *PlexService) processEpisodeInTx(tx *sql.Tx, meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string) error {
	titles := repository.NewTitleRepository(tx)
	seasons := repository.NewSeasonRepository(tx)
	episodes := repository.NewEpisodeRepository(tx)

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

		titleID, createErr := s.titleSvc.CreateFromPlex(tx, seriesName, meta.Year, ids, model.TitleTypeSeries, grandparentKey, meta.GUIDExternal, model.TitleStatusWatching)
		if createErr != nil {
			return fmt.Errorf("create series: %w", createErr)
		}
		title = &model.Title{ID: titleID, Status: model.TitleStatusWatching}
	} else {
		if title.Status != model.TitleStatusCompleted && title.Status != model.TitleStatusWatching {
			watchingStatus := model.TitleStatusWatching
			if updateErr := titles.Update(title.ID, repository.TitleUpdate{Status: &watchingStatus}); updateErr != nil {
				log.Printf("update status to watching: %v", updateErr)
			}
		}
		if needsEnrichment(title) {
			seriesName := meta.GrandparentTitle
			if seriesName == "" {
				seriesName = meta.Title
			}
			s.triggerAsyncEnrichment(title.ID, seriesName, meta.Year, title.Type, ids)
		}
	}

	season, err := seasons.GetOrCreate(title.ID, meta.ParentIndex)
	if err != nil {
		return fmt.Errorf("get/create season: %w", err)
	}

	ep, err := episodes.GetOrCreate(season.ID, meta.Index)
	if err != nil {
		return fmt.Errorf("get/create episode: %w", err)
	}

	// Use LibraryService to mark as watched + notify
	if _, err := s.libSvc.MarkEpisodesWatched(tx, title.ID, []int64{ep.ID}, model.WatchEventSourcePlex, &rawPayload); err != nil {
		return err
	}

	// Auto-complete check
	backfillTMDBID := title.TMDBID
	if backfillTMDBID == nil {
		backfillTMDBID = &ids.TMDB
	}
	if backfillTMDBID != nil && *backfillTMDBID != 0 {
		if err := s.libSvc.CheckAutoComplete(tx, title.ID, *backfillTMDBID, meta.ParentIndex, meta.Index); err != nil {
			log.Printf("auto-complete warning: %v", err)
		}
	}

	return nil
}

// checkSeriesCompleted checks if the given season/episode is the last episode
// of the last season of an ended or cancelled series (via TMDB).
func checkSeriesCompleted(tmdb *matching.TMDBClient, tmdbID int64, seasonNum, episodeNum int) (bool, *model.SeriesStatus) {
	details, err := tmdb.GetTVDetails(tmdbID)
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

// triggerAsyncEnrichment runs the matching pipeline in a goroutine and updates the title.
func (s *PlexService) triggerAsyncEnrichment(titleID int64, titleName string, year int, titleType model.TitleType, ids PlexExternalIDs) {
	if s.pipeline == nil {
		return
	}

	go func() {
		result, err := s.pipeline.Run(matching.MatchInput{
			Title:  titleName,
			Year:   year,
			Type:   titleType,
			IMDBID: ids.IMDB,
			TMDBID: ids.TMDB,
			TVDBID: ids.TVDB,
		})
		if err != nil {
			log.Printf("async enrichment failed for title %d: %v", titleID, err)
			if matching.IsRetryableError(err) {
				s.enqueueEnrichment(titleID, titleName, year, titleType, false, ids)
			}
			return
		}

		update := repository.TitleUpdate{
			MatchStatus:   &result.MatchStatus,
			MatchSource:   &result.MatchSource,
			OriginalTitle: &titleName,
		}
		if result.IMDBID != "" {
			update.IMDBID = &result.IMDBID
		}
		if result.TMDBID != 0 {
			update.TMDBID = &result.TMDBID
		}
		if result.TVDBID != 0 {
			update.TVDBID = &result.TVDBID
		}
		if result.AniListID != 0 {
			update.AniListID = &result.AniListID
		}
		if result.CoverFile != "" {
			coverURL := "/covers/" + result.CoverFile
			update.CoverURL = &coverURL
		}
		if result.TitleType != titleType {
			update.Type = &result.TitleType
		}
		if result.IsAnime {
			update.IsAnime = &result.IsAnime
		}

		if err := s.titles.Update(titleID, update); err != nil {
			log.Printf("async enrichment update failed for title %d: %v", titleID, err)
		} else {
			log.Printf("async enrichment completed for title %d", titleID)
		}
	}()
}

func (s *PlexService) enqueueEnrichment(titleID int64, titleName string, year int, titleType model.TitleType, isAnime bool, ids PlexExternalIDs) {
	if s.tasks == nil {
		return
	}
	payload, _ := json.Marshal(EnrichmentPayload{
		TitleID:   titleID,
		TitleName: titleName,
		Year:      year,
		TitleType: titleType,
		IsAnime:   isAnime,
		IMDBID:    ids.IMDB,
		TMDBID:    ids.TMDB,
		TVDBID:    ids.TVDB,
	})
	dedupKey := fmt.Sprintf("enrichment:%d", titleID)
	if _, err := s.tasks.Enqueue(model.TaskTypeEnrichment, string(payload), &dedupKey); err != nil {
		log.Printf("enqueue enrichment for title %d: %v", titleID, err)
	}
}
