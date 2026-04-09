package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type LibraryService struct {
	db       *sql.DB
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
	settings *repository.SettingRepository
	push     PushNotifier
	backfill *BackfillService
	pipeline *matching.Pipeline // For TMDB access during auto-complete
}

func NewLibraryService(
	db *sql.DB,
	titles *repository.TitleRepository,
	seasons *repository.SeasonRepository,
	episodes *repository.EpisodeRepository,
	events *repository.WatchEventRepository,
	settings *repository.SettingRepository,
	push PushNotifier,
	backfill *BackfillService,
	pipeline *matching.Pipeline,
) *LibraryService {
	return &LibraryService{
		db:       db,
		titles:   titles,
		seasons:  seasons,
		episodes: episodes,
		events:   events,
		settings: settings,
		push:     push,
		backfill: backfill,
		pipeline: pipeline,
	}
}

// ToggleEpisodeWatched toggles the watched status of an episode and logs a watch event.
func (s *LibraryService) ToggleEpisodeWatched(db database.DBTX, titleID, episodeID int64) (*model.Title, error) {
	titles := repository.NewTitleRepository(db)
	episodes := repository.NewEpisodeRepository(db)
	events := repository.NewWatchEventRepository(db)

	ep, err := episodes.ToggleWatched(episodeID)
	if err != nil {
		return nil, err
	}

	if ep.Watched {
		_, _ = events.Create(&model.WatchEvent{
			TitleID:   titleID,
			EpisodeID: &episodeID,
			Source:    model.WatchEventSourceManual,
		})

		if s.backfill != nil {
			s.backfill.BackfillForEpisode(titleID, ep)
		}
	}

	title, err := titles.GetByID(titleID)
	if err != nil {
		return nil, err
	}
	if ep.Watched && title != nil {
		s.maybePromptRating(db, title)
	}

	return title, nil
}

// MarkEpisodesWatched marks multiple episodes as watched and logs watch events.
func (s *LibraryService) MarkEpisodesWatched(db database.DBTX, titleID int64, episodeIDs []int64, source model.WatchEventSource, rawPayload *string) (*model.Title, error) {
	titles := repository.NewTitleRepository(db)
	episodes := repository.NewEpisodeRepository(db)
	events := repository.NewWatchEventRepository(db)

	now := time.Now().UTC()
	if err := episodes.BatchMarkWatched(episodeIDs, now); err != nil {
		return nil, err
	}

	watchEvents := make([]model.WatchEvent, len(episodeIDs))
	for i, epID := range episodeIDs {
		id := epID
		watchEvents[i] = model.WatchEvent{
			TitleID:     titleID,
			EpisodeID:   &id,
			Source:      source,
			PlexPayload: rawPayload,
		}
	}
	if err := events.BatchCreate(watchEvents); err != nil {
		log.Printf("library: batch create watch events for title %d: %v", titleID, err)
	}

	title, err := titles.GetByID(titleID)
	if err != nil {
		return nil, err
	}
	if title != nil {
		s.maybePromptRating(db, title)
	}

	return title, nil
}

// MarkMovieWatched marks a movie title as completed and logs a watch event.
func (s *LibraryService) MarkMovieWatched(db database.DBTX, titleID int64, source model.WatchEventSource, rawPayload *string) error {
	titles := repository.NewTitleRepository(db)
	events := repository.NewWatchEventRepository(db)

	completedStatus := model.TitleStatusCompleted
	if err := titles.Update(titleID, repository.TitleUpdate{Status: &completedStatus}); err != nil {
		return err
	}

	_, _ = events.Create(&model.WatchEvent{
		TitleID:     titleID,
		Source:      source,
		PlexPayload: rawPayload,
	})

	title, err := titles.GetByID(titleID)
	if err != nil {
		return err
	}
	if title != nil {
		s.maybePromptRating(db, title)
	}

	return nil
}

// maybePromptRating sends a push notification if a title is finished and has no rating.
func (s *LibraryService) maybePromptRating(db database.DBTX, title *model.Title) {
	if title.MyRating != nil {
		return
	}

	settings := repository.NewSettingRepository(db)
	if !IsNotificationEnabled(settings, NotifRatingPrompt) {
		return
	}

	shouldPrompt := false
	msg := ""

	if title.Type == model.TitleTypeMovie {
		shouldPrompt = title.Status == model.TitleStatusCompleted
		msg = fmt.Sprintf("Rate %s? You just watched this movie", title.PrimaryName())
	} else {
		for _, season := range title.Seasons {
			if len(season.Episodes) == 0 {
				continue
			}
			allWatched := true
			for _, ep := range season.Episodes {
				if !ep.Watched {
					allWatched = false
					break
				}
			}
			if allWatched {
				shouldPrompt = true
				msg = fmt.Sprintf("Rate %s? You finished season %d", title.PrimaryName(), season.SeasonNumber)
				break
			}
		}
	}

	if shouldPrompt && s.push != nil {
		_ = s.push.SendNotification("PlexTracker", msg, fmt.Sprintf("/title/%d", title.ID))
	}
}

// CheckAutoComplete checks if a series should be marked as completed based on last watched episode.
func (s *LibraryService) CheckAutoComplete(ctx context.Context, db database.DBTX, titleID int64, tmdbID int64, seasonNum, episodeNum int) error {
	if s.pipeline == nil {
		return nil
	}
	tmdbClient := s.pipeline.TMDB()
	if tmdbClient == nil {
		return nil
	}

	if completed, seriesStatus := checkSeriesCompleted(ctx, tmdbClient, tmdbID, seasonNum, episodeNum); completed {
		titles := repository.NewTitleRepository(db)
		completedStatus := model.TitleStatusCompleted
		update := repository.TitleUpdate{Status: &completedStatus}
		if seriesStatus != nil {
			update.SeriesStatus = seriesStatus
		}
		return titles.Update(titleID, update)
	}

	return nil
}
