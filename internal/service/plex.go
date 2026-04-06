package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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
	db       *sql.DB
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
	tasks    *repository.TaskRepository
	settings *repository.SettingRepository
	pipeline *matching.Pipeline // nil = skip matching, create with basic info
	push     PushNotifier
}

func NewPlexService(db *sql.DB, titles *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, tasks *repository.TaskRepository, settings *repository.SettingRepository, pipeline *matching.Pipeline, push PushNotifier) *PlexService {
	return &PlexService{db: db, titles: titles, seasons: seasons, episodes: episodes, events: events, tasks: tasks, settings: settings, pipeline: pipeline, push: push}
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
	events := repository.NewWatchEventRepository(tx)
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
	title, err := titles.FindByExternalID(imdbID, tmdbID, &ratingKey, &movieType)
	if err != nil {
		newTitle, names := s.buildNewTitle(meta.Title, meta.Year, ids, model.TitleTypeMovie, ratingKey, meta.GUIDExternal)
		newTitle.Status = model.TitleStatusCompleted

		titleID, err := titles.Create(newTitle, names)
		if err != nil {
			return fmt.Errorf("create movie: %w", err)
		}

		_, _ = events.Create(&model.WatchEvent{
			TitleID:     titleID,
			Source:      model.WatchEventSourcePlex,
			PlexPayload: &rawPayload,
		})

		if ratingNotifEnabled && s.push != nil {
			_ = s.push.SendNotification(
				"PlexTracker",
				fmt.Sprintf("Rate %s? You just watched this movie", meta.Title),
				fmt.Sprintf("/title/%d", titleID),
			)
		}
		return nil
	}

	if needsEnrichment(title) {
		s.triggerAsyncEnrichment(title.ID, meta.Title, meta.Year, title.Type, ids)
	}

	_, _ = events.Create(&model.WatchEvent{
		TitleID:     title.ID,
		Source:      model.WatchEventSourcePlex,
		PlexPayload: &rawPayload,
	})

	if title.MyRating == nil && ratingNotifEnabled && s.push != nil {
		_ = s.push.SendNotification(
			"PlexTracker",
			fmt.Sprintf("Rate %s? You just watched this movie", meta.Title),
			fmt.Sprintf("/title/%d", title.ID),
		)
	}
	return nil
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
	events := repository.NewWatchEventRepository(tx)

	grandparentKey := meta.GrandparentRatingKey
	var imdbID *string
	var tmdbID *int64

	if ids.IMDB != "" {
		imdbID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		tmdbID = &ids.TMDB
	}

	title, err := titles.FindByExternalID(imdbID, tmdbID, &grandparentKey, nil)
	if err != nil {
		seriesName := meta.GrandparentTitle
		if seriesName == "" {
			seriesName = meta.Title
		}

		newTitle, names := s.buildNewTitle(seriesName, meta.Year, ids, model.TitleTypeSeries, grandparentKey, meta.GUIDExternal)
		newTitle.Status = model.TitleStatusWatching

		titleID, createErr := titles.Create(newTitle, names)
		if createErr != nil {
			return fmt.Errorf("create series: %w", createErr)
		}
		title = &model.Title{ID: titleID}
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

	now := time.Now().UTC()
	if !ep.Watched {
		_ = episodes.MarkWatched(ep.ID, now)
	}

	_, _ = events.Create(&model.WatchEvent{
		TitleID:     title.ID,
		EpisodeID:   &ep.ID,
		Source:      model.WatchEventSourcePlex,
		PlexPayload: &rawPayload,
	})

	// Backfill previous episodes — prefer title's TMDB ID (from DB) over Plex GUIDs
	backfillTMDBID := title.TMDBID
	if backfillTMDBID == nil {
		backfillTMDBID = tmdbID
	}
	var tmdbClient *matching.TMDBClient
	if s.pipeline != nil {
		tmdbClient = s.pipeline.TMDB()
	}
	if err := BackfillPreviousEpisodes(tx, tmdbClient, title.ID, backfillTMDBID, meta.ParentIndex, meta.Index, now); err != nil {
		log.Printf("backfill warning: %v", err)
	}

	// Auto-complete if this is the last episode of the last season of an ended/cancelled series
	if tmdbClient != nil && backfillTMDBID != nil {
		if completed, seriesStatus := checkSeriesCompleted(tmdbClient, *backfillTMDBID, meta.ParentIndex, meta.Index); completed {
			completedStatus := model.TitleStatusCompleted
			update := repository.TitleUpdate{Status: &completedStatus}
			if seriesStatus != nil {
				update.SeriesStatus = seriesStatus
			}
			if err := titles.Update(title.ID, update); err != nil {
				log.Printf("auto-complete warning: %v", err)
			} else {
				log.Printf("auto-completed series (title %d) on last episode", title.ID)
			}
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
				s.enqueueEnrichment(titleID, titleName, year, titleType, ids)
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

		if err := s.titles.Update(titleID, update); err != nil {
			log.Printf("async enrichment update failed for title %d: %v", titleID, err)
		} else {
			log.Printf("async enrichment completed for title %d", titleID)
		}
	}()
}

func (s *PlexService) enqueueEnrichment(titleID int64, titleName string, year int, titleType model.TitleType, ids PlexExternalIDs) {
	if s.tasks == nil {
		return
	}
	payload, _ := json.Marshal(EnrichmentPayload{
		TitleID:   titleID,
		TitleName: titleName,
		Year:      year,
		TitleType: titleType,
		IMDBID:    ids.IMDB,
		TMDBID:    ids.TMDB,
		TVDBID:    ids.TVDB,
	})
	dedupKey := fmt.Sprintf("enrichment:%d", titleID)
	if _, err := s.tasks.Enqueue(model.TaskTypeEnrichment, string(payload), &dedupKey); err != nil {
		log.Printf("enqueue enrichment for title %d: %v", titleID, err)
	}
}

// buildNewTitle runs the matching pipeline (if available) and constructs a Title + names.
func (s *PlexService) buildNewTitle(title string, year int, ids PlexExternalIDs, titleType model.TitleType, ratingKey string, guids []*url.URL) (*model.Title, []model.TitleName) {
	t := &model.Title{
		Type:          titleType,
		Year:          year,
		PlexRatingKey: &ratingKey,
	}

	if s.pipeline != nil {
		result, err := s.pipeline.Run(matching.MatchInput{
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
			return t, result.Names
		}
	}

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
	names := []model.TitleName{{Name: title, Language: "en", IsPrimary: true}}
	return t, names
}
