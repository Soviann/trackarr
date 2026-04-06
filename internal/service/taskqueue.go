package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

// Task payloads

type EnrichmentPayload struct {
	TitleID   int64           `json:"title_id"`
	TitleName string          `json:"title_name"`
	Year      int             `json:"year"`
	TitleType model.TitleType `json:"title_type"`
	IMDBID    string          `json:"imdb_id,omitempty"`
	TMDBID    int64           `json:"tmdb_id,omitempty"`
	TVDBID    int64           `json:"tvdb_id,omitempty"`
}

type RefreshPayload struct {
	TitleID int64 `json:"title_id"`
}

type CoverFetchPayload struct {
	TitleID   int64           `json:"title_id"`
	TMDBID    int64           `json:"tmdb_id"`
	AniListID int64           `json:"anilist_id,omitempty"`
	TitleType model.TitleType `json:"title_type"`
}

// TaskQueueWorker processes retryable tasks from the queue.
type TaskQueueWorker struct {
	tasks    *repository.TaskRepository
	titles   *repository.TitleRepository
	pipeline *matching.Pipeline
	tmdb     *matching.TMDBClient
	anilist  *matching.AniListClient
	push     PushNotifier
	settings *repository.SettingRepository
	dataDir  string
	limiter  *APILimiter
}

func NewTaskQueueWorker(
	tasks *repository.TaskRepository,
	titles *repository.TitleRepository,
	pipeline *matching.Pipeline,
	tmdb *matching.TMDBClient,
	anilist *matching.AniListClient,
	push PushNotifier,
	settings *repository.SettingRepository,
	dataDir string,
) *TaskQueueWorker {
	return &TaskQueueWorker{
		tasks:    tasks,
		titles:   titles,
		pipeline: pipeline,
		tmdb:     tmdb,
		anilist:  anilist,
		push:     push,
		settings: settings,
		dataDir:  dataDir,
		limiter:  NewAPILimiter(2, 1),
	}
}

// Start launches the worker loop. It polls for due tasks every 30 seconds.
func (w *TaskQueueWorker) Start(ctx context.Context) {
	// Log queue state at startup
	counts, err := w.tasks.CountByStatus()
	if err == nil {
		total := 0
		for _, c := range counts {
			total += c
		}
		if total > 0 {
			log.Printf("task queue: %d pending, %d sleeping, %d dead",
				counts[model.TaskStatusPending]+counts[model.TaskStatusRunning],
				counts[model.TaskStatusSleeping],
				counts[model.TaskStatusDead],
			)
		}
	}

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.processDueTasks()
			}
		}
	}()
}

func (w *TaskQueueWorker) processDueTasks() {
	tasks, err := w.tasks.FetchDue(10)
	if err != nil {
		log.Printf("task queue: fetch due: %v", err)
		return
	}

	for _, task := range tasks {
		_ = w.limiter.Wait(context.Background())
		w.processTask(task)
	}
}

func (w *TaskQueueWorker) processTask(task model.Task) {
	var err error

	switch task.TaskType {
	case model.TaskTypeEnrichment:
		err = w.handleEnrichment(task)
	case model.TaskTypeRefresh:
		err = w.handleRefresh(task)
	case model.TaskTypeCoverFetch:
		err = w.handleCoverFetch(task)
	default:
		log.Printf("task queue: unknown task type %q for task %d", task.TaskType, task.ID)
		_ = w.tasks.Delete(task.ID)
		return
	}

	if err != nil {
		retryAfter := matching.ExtractRetryAfter(err)
		nextRunAt := calculateNextRunAt(task.Attempts+1, retryAfter)
		if failErr := w.tasks.Fail(task.ID, err.Error(), nextRunAt); failErr != nil {
			log.Printf("task queue: fail task %d: %v", task.ID, failErr)
		}

		// Check if task just died (day 2 + last attempt)
		if task.Day >= 2 && task.Attempts+1 >= task.MaxAttempts {
			w.notifyDeadTask(task)
		}

		return
	}

	if err := w.tasks.Complete(task.ID); err != nil {
		log.Printf("task queue: complete task %d: %v", task.ID, err)
	}
}

func (w *TaskQueueWorker) handleEnrichment(task model.Task) error {
	var payload EnrichmentPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode enrichment payload: %w", err)
	}

	if w.pipeline == nil {
		return fmt.Errorf("pipeline not configured")
	}

	result, err := w.pipeline.Run(matching.MatchInput{
		Title:  payload.TitleName,
		Year:   payload.Year,
		Type:   payload.TitleType,
		IMDBID: payload.IMDBID,
		TMDBID: payload.TMDBID,
		TVDBID: payload.TVDBID,
	})
	if err != nil {
		return err
	}

	update := repository.TitleUpdate{
		MatchStatus:   &result.MatchStatus,
		MatchSource:   &result.MatchSource,
		OriginalTitle: &payload.TitleName,
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
		update.CoverURL = &result.CoverFile
	}
	if result.TitleType != payload.TitleType {
		update.Type = &result.TitleType
	}
	if result.Overview != "" {
		update.Overview = &result.Overview
	}
	if result.Genres != "" {
		update.Genres = &result.Genres
	}
	if result.Runtime != nil {
		update.Runtime = result.Runtime
	}
	if result.TMDBRating != nil {
		update.TMDBRating = result.TMDBRating
	}
	if result.Credits != "" {
		update.Credits = &result.Credits
	}
	if result.AniListRating != nil {
		update.AniListRating = result.AniListRating
	}
	if result.ReleaseDate != "" {
		update.ReleaseDate = &result.ReleaseDate
	}

	if err := w.titles.Update(payload.TitleID, update); err != nil {
		return err
	}

	if len(result.Names) > 0 {
		if err := w.titles.ReplaceNames(payload.TitleID, result.Names); err != nil {
			log.Printf("enrichment: replace names for title %d: %v", payload.TitleID, err)
		}
	}

	return nil
}

func (w *TaskQueueWorker) handleRefresh(task model.Task) error {
	var payload RefreshPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode refresh payload: %w", err)
	}

	if w.tmdb == nil {
		return fmt.Errorf("TMDB client not configured")
	}

	title, err := w.titles.GetByID(payload.TitleID)
	if err != nil {
		return fmt.Errorf("get title %d: %w", payload.TitleID, err)
	}

	// Try TMDB cover
	if title.TMDBID != nil && title.CoverURL == nil {
		if title.Type == model.TitleTypeMovie {
			details, err := w.tmdb.GetMovieDetails(*title.TMDBID)
			if err != nil {
				return err
			}
			if details.PosterPath != nil {
				coverPath, err := w.tmdb.DownloadCover(*details.PosterPath, fmt.Sprintf("%s/covers", w.dataDir))
				if err == nil {
					_ = w.titles.Update(title.ID, repository.TitleUpdate{CoverURL: &coverPath})
					title.CoverURL = &coverPath
				}
			}
		} else {
			details, err := w.tmdb.GetTVDetails(*title.TMDBID)
			if err != nil {
				return err
			}
			if details.PosterPath != nil {
				coverPath, err := w.tmdb.DownloadCover(*details.PosterPath, fmt.Sprintf("%s/covers", w.dataDir))
				if err == nil {
					_ = w.titles.Update(title.ID, repository.TitleUpdate{CoverURL: &coverPath})
					title.CoverURL = &coverPath
				}
			}
		}
	}

	// Fallback: AniList cover
	if title.CoverURL == nil && title.AniListID != nil {
		w.downloadAniListCover(title)
	}

	return nil
}

func (w *TaskQueueWorker) handleCoverFetch(task model.Task) error {
	var payload CoverFetchPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode cover_fetch payload: %w", err)
	}

	coversDir := fmt.Sprintf("%s/covers", w.dataDir)

	// Try TMDB
	if w.tmdb != nil && payload.TMDBID != 0 {
		var posterPath *string
		if payload.TitleType == model.TitleTypeMovie {
			details, err := w.tmdb.GetMovieDetails(payload.TMDBID)
			if err != nil {
				return err
			}
			posterPath = details.PosterPath
		} else {
			details, err := w.tmdb.GetTVDetails(payload.TMDBID)
			if err != nil {
				return err
			}
			posterPath = details.PosterPath
		}

		if posterPath != nil && *posterPath != "" {
			coverPath, err := w.tmdb.DownloadCover(*posterPath, coversDir)
			if err != nil {
				return err
			}
			_ = w.titles.Update(payload.TitleID, repository.TitleUpdate{CoverURL: &coverPath})
			return nil
		}
	}

	// Fallback: AniList
	if w.anilist != nil && payload.AniListID != 0 {
		details, err := w.anilist.GetAnimeDetails(payload.AniListID)
		if err != nil {
			return fmt.Errorf("anilist cover fetch: %w", err)
		}
		if details.CoverURL != "" {
			coverPath, err := w.anilist.DownloadCover(details.CoverURL, coversDir)
			if err != nil {
				return fmt.Errorf("download anilist cover: %w", err)
			}
			_ = w.titles.Update(payload.TitleID, repository.TitleUpdate{CoverURL: &coverPath})
		}
	}

	return nil
}

// notifyDeadTask sends a push notification when a task dies (if enabled).
func (w *TaskQueueWorker) downloadAniListCover(title *model.Title) {
	if w.anilist == nil || title.AniListID == nil {
		return
	}

	details, err := w.anilist.GetAnimeDetails(*title.AniListID)
	if err != nil || details.CoverURL == "" {
		return
	}

	coverPath, err := w.anilist.DownloadCover(details.CoverURL, fmt.Sprintf("%s/covers", w.dataDir))
	if err != nil {
		return
	}

	_ = w.titles.Update(title.ID, repository.TitleUpdate{CoverURL: &coverPath})
	title.CoverURL = &coverPath
}

func (w *TaskQueueWorker) notifyDeadTask(task model.Task) {
	if !IsNotificationEnabled(w.settings, NotifDeadTask) {
		return
	}

	// Extract title name from payload for the notification
	titleName := "unknown"
	var ep EnrichmentPayload
	if json.Unmarshal([]byte(task.Payload), &ep) == nil && ep.TitleName != "" {
		titleName = ep.TitleName
	}

	_ = w.push.SendNotification(
		"PlexTracker",
		fmt.Sprintf("Task failed — Unable to process: %s", titleName),
		"/admin/tasks",
	)
}

// calculateNextRunAt computes the next retry time with exponential backoff + jitter.
// Base delay: 30s, max: 1 hour. Jitter: 0-25% of calculated delay.
func calculateNextRunAt(attempts int, retryAfter time.Duration) time.Time {
	base := 30 * time.Second
	delay := time.Duration(float64(base) * math.Pow(2, float64(attempts-1)))

	maxDelay := time.Hour
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter: 0-25%
	jitter := time.Duration(float64(delay) * 0.25 * rand.Float64())
	delay += jitter

	// If Retry-After header is present, use whichever is larger
	if retryAfter > delay {
		delay = retryAfter
	}

	return time.Now().Add(delay)
}

// Notification preference helpers

const (
	NotifRatingPrompt = "notif_rating_prompt"
	NotifDeadTask     = "notif_dead_task"
	NotifSeriesEnded  = "notif_series_ended"
)

// IsNotificationEnabled checks whether a notification type is enabled.
// Default is enabled (when key is absent from settings).
func IsNotificationEnabled(settings *repository.SettingRepository, notifType string) bool {
	if settings == nil {
		return true // default: enabled
	}
	val, err := settings.Get(notifType)
	if err != nil {
		return true // default: enabled
	}
	return val != "false"
}
