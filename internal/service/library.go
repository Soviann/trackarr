package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
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

// safeRuntime safely dereferences a runtime pointer, returning 0 if nil.
func safeRuntime(r *int) int {
	if r == nil {
		return 0
	}
	return *r
}

// RatingPrompt captures the data needed to fire a rating-prompt notification
// AFTER the write transaction commits. Holding push I/O inside the tx would
// tie up the sole SQLite write connection on any push-endpoint slowness.
type RatingPrompt struct {
	TitleID int64
	Message string
}

// ToggleEpisodeWatched toggles the watched status of an episode and logs a watch event.
//
// Returns the toggled *model.Episode and a *RatingPrompt (or nils) so the
// caller can fire two post-commit actions:
//   - rating-prompt push (already-existing pattern).
//   - backfill trigger via TriggerBackfillForEpisode — backfill opens its own
//     writeDB tx, so running it inside this one deadlocks on MaxOpenConns=1.
func (s *LibraryService) ToggleEpisodeWatched(ctx context.Context, tx *sql.Tx, titleID, episodeID int64) (*model.Title, *model.Episode, *RatingPrompt, error) {
	titles := repository.NewTitleRepository(tx)
	titlesW := repository.NewTitleWriter(tx)
	episodes := repository.NewEpisodeWriter(tx)
	events := repository.NewWatchEventWriter(tx)

	ep, err := episodes.ToggleWatched(ctx, episodeID)
	if err != nil {
		return nil, nil, nil, err
	}

	if ep.Watched {
		_, _ = events.Create(ctx, &model.WatchEvent{
			TitleID:   titleID,
			EpisodeID: &episodeID,
			Source:    model.WatchEventSourceManual,
		})
	}

	title, err := titles.GetByID(titleID)
	if err != nil {
		return nil, nil, nil, err
	}

	// Update total_watch_minutes: increment if watched, decrement if unwatched.
	if title != nil {
		delta := safeRuntime(title.Runtime)
		if !ep.Watched {
			delta = -delta
		}
		newTotal := title.TotalWatchMinutes + delta
		if newTotal < 0 {
			newTotal = 0
		}
		update := repository.TitleUpdate{TotalWatchMinutes: &newTotal}
		if !ep.Watched && title.Status == model.TitleStatusCompleted {
			watching := model.TitleStatusWatching
			update.Status = &watching
		} else if ep.Watched && title.Type != model.TitleTypeMovie {
			isEndedOrCancelled := title.SeriesStatus != nil &&
				(*title.SeriesStatus == model.SeriesStatusEnded || *title.SeriesStatus == model.SeriesStatusCancelled)
			if isEndedOrCancelled {
				hasUnwatched, err := titles.HasUnwatchedEpisodes(titleID)
				if err == nil && !hasUnwatched {
					completed := model.TitleStatusCompleted
					update.Status = &completed
				} else if title.Status == model.TitleStatusPlanToWatch {
					watching := model.TitleStatusWatching
					update.Status = &watching
				}
			} else if title.Status == model.TitleStatusPlanToWatch {
				watching := model.TitleStatusWatching
				update.Status = &watching
			}
		}
		_ = titlesW.Update(ctx, titleID, update)
		title.TotalWatchMinutes = newTotal
		if update.Status != nil {
			title.Status = *update.Status
		}
	}

	var prompt *RatingPrompt
	if ep.Watched && title != nil {
		prompt = s.buildRatingPrompt(tx, title)
	}

	// Progress changed (watch or unwatch): push the new season state to AniList
	// so the derived CURRENT/COMPLETED/PLANNING transition reaches the remote.
	EnqueueAniListSeasonPush(ctx, tx, ep.SeasonID)

	return title, ep, prompt, nil
}

// ToggleEpisodeWatchedTx wraps ToggleEpisodeWatched in a managed database transaction.
func (s *LibraryService) ToggleEpisodeWatchedTx(ctx context.Context, titleID, episodeID int64) (*model.Title, *model.Episode, *RatingPrompt, error) {
	var (
		title  *model.Title
		ep     *model.Episode
		prompt *RatingPrompt
	)
	err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		title, ep, prompt, err = s.ToggleEpisodeWatched(ctx, tx, titleID, episodeID)
		return err
	})
	return title, ep, prompt, err
}

// TriggerBackfillForEpisode is the post-commit half of ToggleEpisodeWatched:
// fires previous-episode backfill only when watching forward (never on
// unwatch). Safe to call with nil ep.
//
// Callers MUST invoke this after the write transaction has committed —
// BackfillForEpisode opens its own writeDB tx and will deadlock if called
// inside another one.
func (s *LibraryService) TriggerBackfillForEpisode(ctx context.Context, titleID int64, ep *model.Episode) {
	if s.backfill == nil || ep == nil || !ep.Watched {
		return
	}
	s.backfill.BackfillForEpisode(ctx, titleID, ep)
}

// MarkEpisodesWatched marks multiple episodes as watched and logs watch events.
// Returns a *RatingPrompt (or nil) so the caller can fire the push AFTER commit.
//
// seasonIDs is an optional hint: when non-nil, it bypasses the
// distinctSeasonIDs lookup inside the write transaction. The Plex scrobble
// hot path always operates on one episode whose season is already loaded —
// passing the hint avoids a SELECT inside the only writeDB connection.
// Pass nil when the caller does not already know the season IDs (manual
// batch-watch flow).
func (s *LibraryService) MarkEpisodesWatched(ctx context.Context, tx *sql.Tx, titleID int64, episodeIDs []int64, seasonIDs []int64, source model.WatchEventSource, rawPayload *string) (*model.Title, *RatingPrompt, error) {
	titles := repository.NewTitleRepository(tx)
	titlesW := repository.NewTitleWriter(tx)
	episodes := repository.NewEpisodeWriter(tx)
	events := repository.NewWatchEventWriter(tx)

	now := time.Now().UTC()
	if err := episodes.BatchMarkWatched(ctx, episodeIDs, now); err != nil {
		return nil, nil, err
	}

	watchEvents := make([]model.WatchEvent, len(episodeIDs))
	for i, epID := range episodeIDs {
		watchEvents[i] = model.WatchEvent{
			TitleID:    titleID,
			EpisodeID:  &epID,
			Source:     source,
			RawPayload: rawPayload,
		}
	}
	if err := events.BatchCreate(ctx, watchEvents); err != nil {
		log.Printf("library: batch create watch events for title %d: %v", titleID, err)
	}

	title, err := titles.GetByID(titleID)
	if err != nil {
		return nil, nil, err
	}
	var prompt *RatingPrompt
	if title != nil {
		// Increment total_watch_minutes by runtime × number of newly watched episodes.
		newTotal := title.TotalWatchMinutes + safeRuntime(title.Runtime)*len(episodeIDs)
		update := repository.TitleUpdate{TotalWatchMinutes: &newTotal}
		if title.Type != model.TitleTypeMovie {
			isEndedOrCancelled := title.SeriesStatus != nil &&
				(*title.SeriesStatus == model.SeriesStatusEnded || *title.SeriesStatus == model.SeriesStatusCancelled)
			if isEndedOrCancelled {
				hasUnwatched, err := titles.HasUnwatchedEpisodes(titleID)
				if err == nil && !hasUnwatched {
					completed := model.TitleStatusCompleted
					update.Status = &completed
				} else if title.Status == model.TitleStatusPlanToWatch {
					watching := model.TitleStatusWatching
					update.Status = &watching
				}
			} else if title.Status == model.TitleStatusPlanToWatch {
				watching := model.TitleStatusWatching
				update.Status = &watching
			}
		}
		if err := titlesW.Update(ctx, titleID, update); err != nil {
			log.Printf("library: update title for %d: %v", titleID, err)
		}
		title.TotalWatchMinutes = newTotal
		if update.Status != nil {
			title.Status = *update.Status
		}
		prompt = s.buildRatingPrompt(tx, title)
	}

	// Push one task per distinct season in the batch. Plex scrobbles pass a
	// single episode at a time, but manual batch-watch can span multiple
	// seasons when the user catches up on backlogs.
	pushSeasonIDs := seasonIDs
	if pushSeasonIDs == nil {
		pushSeasonIDs = distinctSeasonIDs(ctx, tx, episodeIDs)
	}
	for _, seasonID := range pushSeasonIDs {
		EnqueueAniListSeasonPush(ctx, tx, seasonID)
	}

	return title, prompt, nil
}

// MarkEpisodesWatchedTx wraps MarkEpisodesWatched in a managed database transaction.
func (s *LibraryService) MarkEpisodesWatchedTx(ctx context.Context, titleID int64, episodeIDs []int64, seasonIDs []int64, source model.WatchEventSource, rawPayload *string) (*model.Title, *RatingPrompt, error) {
	var (
		title  *model.Title
		prompt *RatingPrompt
	)
	err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		title, prompt, err = s.MarkEpisodesWatched(ctx, tx, titleID, episodeIDs, seasonIDs, source, rawPayload)
		return err
	})
	return title, prompt, err
}

// MarkMovieWatched marks a movie title as completed and logs a watch event.
// Returns a *RatingPrompt (or nil) so the caller can fire the push AFTER commit.
func (s *LibraryService) MarkMovieWatched(ctx context.Context, tx *sql.Tx, titleID int64, source model.WatchEventSource, rawPayload *string) (*RatingPrompt, error) {
	titles := repository.NewTitleRepository(tx)
	titlesW := repository.NewTitleWriter(tx)
	events := repository.NewWatchEventWriter(tx)

	// Fetch title first to get runtime for watchtime calculation.
	title, err := titles.GetByID(titleID)
	if err != nil {
		return nil, err
	}

	completedStatus := model.TitleStatusCompleted
	newTotal := title.TotalWatchMinutes + safeRuntime(title.Runtime)
	if err := titlesW.Update(ctx, titleID, repository.TitleUpdate{Status: &completedStatus, TotalWatchMinutes: &newTotal}); err != nil {
		return nil, err
	}

	_, _ = events.Create(ctx, &model.WatchEvent{
		TitleID:    titleID,
		Source:     source,
		RawPayload: rawPayload,
	})

	title.TotalWatchMinutes = newTotal
	// Webhook-driven movie scrobbles never hit the PATCH handler, so we enqueue
	// the AniList push here. PushMovieState short-circuits on non-AniList or
	// non-anime titles, but pre-filtering avoids creating throwaway tasks.
	if title.IsAnime && title.AniListID != nil && *title.AniListID != 0 {
		EnqueueAniListMoviePush(ctx, tx, titleID)
	}
	return s.buildRatingPrompt(tx, title), nil
}

// SendRatingPrompt fires a rating-prompt push notification. Must be called
// AFTER any enclosing write transaction has committed so a slow push endpoint
// cannot hold the sole SQLite write connection. Safe to call with nil. The
// ctx is only used to scope the Unsubscribe that runs when the push endpoint
// reports the subscription as gone (410); push delivery itself is always
// bounded by pushHTTPClient's 5s timeout.
func (s *LibraryService) SendRatingPrompt(ctx context.Context, p *RatingPrompt) {
	if p == nil || s.push == nil {
		return
	}
	if err := s.push.SendNotification(ctx, "Trackarr", p.Message, fmt.Sprintf("/title/%d", p.TitleID)); err != nil {
		log.Printf("rating prompt push failed: %v", err)
	}
}

// buildRatingPrompt inspects a title and returns a RatingPrompt if a rating
// notification should fire. Read-only DB access; performs no I/O.
func (s *LibraryService) buildRatingPrompt(db database.DBTX, title *model.Title) *RatingPrompt {
	if title.MyRating != nil {
		return nil
	}

	settings := repository.NewSettingRepository(db)
	if !IsNotificationEnabled(settings, NotifRatingPrompt) {
		return nil
	}

	if title.Type == model.TitleTypeMovie {
		if title.Status != model.TitleStatusCompleted {
			return nil
		}
		return &RatingPrompt{
			TitleID: title.ID,
			Message: fmt.Sprintf("Rate %s? You just watched this movie", title.PrimaryName()),
		}
	}

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
			return &RatingPrompt{
				TitleID: title.ID,
				Message: fmt.Sprintf("Rate %s? You finished season %d", title.PrimaryName(), season.SeasonNumber),
			}
		}
	}

	return nil
}

// EnqueueAniListSeasonPush schedules a per-season AniList push within the
// caller's transaction. Errors are logged and swallowed — the task queue is
// a best-effort propagation layer, never the source of truth.
func EnqueueAniListSeasonPush(ctx context.Context, tx *sql.Tx, seasonID int64) {
	payload, err := json.Marshal(AniListPushSeasonPayload{SeasonID: seasonID})
	if err != nil {
		log.Printf("library: marshal anilist push payload for season %d: %v", seasonID, err)
		return
	}
	if _, err := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeAniListPushSeason, string(payload), nil); err != nil {
		log.Printf("library: enqueue anilist push for season %d: %v", seasonID, err)
	}
}

// EnqueueAniListMoviePush schedules a per-movie AniList push (anime movies
// only — guard at the call site).
func EnqueueAniListMoviePush(ctx context.Context, tx *sql.Tx, titleID int64) {
	payload, err := json.Marshal(AniListPushMoviePayload{TitleID: titleID})
	if err != nil {
		log.Printf("library: marshal anilist push payload for movie %d: %v", titleID, err)
		return
	}
	if _, err := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeAniListPushMovie, string(payload), nil); err != nil {
		log.Printf("library: enqueue anilist push for movie %d: %v", titleID, err)
	}
}

// distinctSeasonIDs returns the distinct season_id values of the given
// episodes. Batch watches can span multiple seasons (catch-up scenarios), so
// we must emit one push task per season — enqueueing duplicates would be
// wasted work.
func distinctSeasonIDs(ctx context.Context, tx *sql.Tx, episodeIDs []int64) []int64 {
	if len(episodeIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(episodeIDs))
	args := make([]any, len(episodeIDs))
	for i, id := range episodeIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT DISTINCT season_id FROM episodes WHERE id IN (%s)`, strings.Join(placeholders, ","))
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("library: query distinct season ids: %v", err)
		return nil
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			log.Printf("library: scan distinct season id: %v", err)
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// CheckAutoComplete checks if a series should be marked completed based on the
// last watched episode. Runs AFTER the scrobble transaction commits (TMDB HTTP
// out of the write path), so it takes the pool handle and opens its own tx.
func (s *LibraryService) CheckAutoComplete(ctx context.Context, db *sql.DB, titleID int64, tmdbID int64, seasonNum, episodeNum int) error {
	if s.pipeline == nil {
		return nil
	}
	tmdbClient := s.pipeline.TMDB()
	if tmdbClient == nil {
		return nil
	}

	if completed, seriesStatus := checkSeriesCompleted(ctx, tmdbClient, tmdbID, seasonNum, episodeNum); completed {
		completedStatus := model.TitleStatusCompleted
		update := repository.TitleUpdate{Status: &completedStatus}
		if seriesStatus != nil {
			update.SeriesStatus = seriesStatus
		}
		return database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
			return repository.NewTitleWriter(tx).Update(ctx, titleID, update)
		})
	}

	return nil
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
