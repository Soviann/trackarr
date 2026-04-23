package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"path/filepath"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
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
	IsAnime   bool            `json:"is_anime"`
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
	tasks       *repository.TaskRepository
	titles      *repository.TitleRepository
	events      *repository.WatchEventRepository
	genres      *repository.GenreRepository
	pipeline    *matching.Pipeline
	tmdb        *matching.TMDBClient
	anilist     *matching.AniListClient
	push        PushNotifier
	settings    *repository.SettingRepository
	dataDir     string
	limiter     *APILimiter
	pausedUntil time.Time
	titleSvc    *TitleService
	writeDB     *sql.DB
}

func NewTaskQueueWorker(
	tasks *repository.TaskRepository,
	titles *repository.TitleRepository,
	events *repository.WatchEventRepository,
	genres *repository.GenreRepository,
	pipeline *matching.Pipeline,
	tmdb *matching.TMDBClient,
	anilist *matching.AniListClient,
	push PushNotifier,
	settings *repository.SettingRepository,
	dataDir string,
	titleSvc *TitleService,
	writeDB *sql.DB,
) *TaskQueueWorker {
	return &TaskQueueWorker{
		tasks:    tasks,
		titles:   titles,
		events:   events,
		genres:   genres,
		pipeline: pipeline,
		tmdb:     tmdb,
		anilist:  anilist,
		push:     push,
		settings: settings,
		dataDir:  dataDir,
		limiter:  NewAPILimiter(2, 1),
		titleSvc: titleSvc,
		writeDB:  writeDB,
	}
}

// Start launches the worker loop. It polls for due tasks every 30 seconds.
func (w *TaskQueueWorker) Start(ctx context.Context) {
	// Rescue stuck tasks from previous crashes
	if err := w.tasks.ResetRunning(); err != nil {
		log.Printf("task queue: reset running tasks: %v", err)
	}

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
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("task queue: panic in worker loop: %v", r)
						time.Sleep(30 * time.Second)
					}
				}()

				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						w.processDueTasks(ctx)
					}
				}
			}()

			if ctx.Err() != nil {
				return
			}
		}
	}()
}

func (w *TaskQueueWorker) processDueTasks(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("task queue: panic in processDueTasks: %v", r)
		}
	}()

	if time.Now().Before(w.pausedUntil) {
		return
	}

	tasks, err := w.tasks.FetchDue(10)
	if err != nil {
		log.Printf("task queue: fetch due: %v", err)
		return
	}

	for _, task := range tasks {
		if time.Now().Before(w.pausedUntil) {
			break
		}
		_ = w.limiter.Wait(ctx)
		taskCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		w.ProcessTask(taskCtx, task)
		cancel()
	}
}

func (w *TaskQueueWorker) ProcessTask(ctx context.Context, task model.Task) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("task queue: panic processing task %d: %v", task.ID, r)
			_ = w.tasks.Fail(task.ID, fmt.Sprintf("panic: %v", r), time.Now().Add(time.Hour))
		}
	}()

	var err error

	switch task.TaskType {
	case model.TaskTypeEnrichment:
		err = w.handleEnrichment(ctx, task)
	case model.TaskTypeRefresh:
		err = w.handleRefresh(ctx, task)
	case model.TaskTypeCoverFetch:
		err = w.handleCoverFetch(ctx, task)
	default:
		log.Printf("task queue: unknown task type %q for task %d", task.TaskType, task.ID)
		_ = w.tasks.Delete(task.ID)
		return
	}

	if err != nil {
		retryAfter := matching.ExtractRetryAfter(err)

		// Global backoff: if we hit a rate limit, pause the whole queue
		if matching.IsRetryableError(err) && (matching.ExtractRetryAfter(err) > 0 || matching.IsRateLimitError(err)) {
			pauseDuration := 5 * time.Minute
			if retryAfter > pauseDuration {
				pauseDuration = retryAfter
			}
			w.pausedUntil = time.Now().Add(pauseDuration)
			log.Printf("task queue: rate limit hit, pausing worker until %s", w.pausedUntil.Format("15:04:05"))
		}

		nextRunAt := calculateNextRunAt(task.Attempts+1, retryAfter)
		if failErr := w.tasks.Fail(task.ID, err.Error(), nextRunAt); failErr != nil {
			log.Printf("task queue: fail task %d: %v", task.ID, failErr)
		}

		// Check if task just died (day 7 + last attempt)
		if task.Day >= 7 && task.Attempts+1 >= task.MaxAttempts {
			w.notifyDeadTask(task)
		}

		return
	}
	if err := w.tasks.Complete(task.ID); err != nil {
		log.Printf("task queue: complete task %d: %v", task.ID, err)
	}
}

func (w *TaskQueueWorker) handleEnrichment(ctx context.Context, task model.Task) error {
	var payload EnrichmentPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode enrichment payload: %w", err)
	}

	if w.pipeline == nil {
		return fmt.Errorf("pipeline not configured")
	}

	result, err := w.pipeline.Run(ctx, matching.MatchInput{
		Title:   payload.TitleName,
		Year:    payload.Year,
		Type:    payload.TitleType,
		IsAnime: payload.IsAnime,
		TMDBID:  payload.TMDBID,
		TVDBID:  payload.TVDBID,
	})
	if err != nil {
		return err
	}

	update := buildEnrichmentUpdate(result, payload)

	var genreList []string
	if result.Genres != "" && w.genres != nil {
		if err := json.Unmarshal([]byte(result.Genres), &genreList); err != nil {
			log.Printf("enrichment: decode genres for title %d: %v", payload.TitleID, err)
			genreList = nil
		}
	}

	err = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		titlesTx := repository.NewTitleWriter(tx)
		genresTx := repository.NewGenreRepository(tx)

		if err := titlesTx.Update(ctx, payload.TitleID, update); err != nil {
			return err
		}

		recalcWatchtime(ctx, tx, payload.TitleID, result.Runtime)

		if len(result.Names) > 0 {
			if err := titlesTx.ReplaceNames(ctx, payload.TitleID, result.Names); err != nil {
				log.Printf("enrichment: replace names for title %d: %v", payload.TitleID, err)
			}
		}

		if len(genreList) > 0 {
			if err := genresTx.ReplaceForTitle(ctx, payload.TitleID, genreList); err != nil {
				log.Printf("enrichment: save genres for title %d: %v", payload.TitleID, err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Merge runs outside the persistence tx: it opens its own tx and may
	// delete the source title, so it must not nest inside another writer.
	if _, err := w.resolveAnimeConflict(ctx, result, payload); err != nil {
		return err
	}

	return nil
}

// buildEnrichmentUpdate translates a pipeline MatchResult into a TitleUpdate
// diff. Pure: no side effects, safe to call outside a transaction.
func buildEnrichmentUpdate(result *matching.MatchResult, payload EnrichmentPayload) repository.TitleUpdate {
	update := repository.TitleUpdate{
		MatchStatus: &result.MatchStatus,
		MatchSource: &result.MatchSource,
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
	if result.IsAnime {
		update.IsAnime = &result.IsAnime
	}
	if result.ReleaseDate != "" {
		update.ReleaseDate = &result.ReleaseDate
	}
	return update
}

// recalcWatchtime refreshes total_watch_minutes when runtime was (re)learned.
// Runs against the caller's transaction so all enrichment writes commit or
// roll back as one. Errors are logged and swallowed — watchtime is derived,
// never authoritative.
func recalcWatchtime(ctx context.Context, tx *sql.Tx, titleID int64, runtime *int) {
	if runtime == nil {
		return
	}
	events := repository.NewWatchEventRepository(tx)
	writer := repository.NewTitleWriter(tx)
	count, err := events.CountByTitleID(titleID)
	if err != nil {
		log.Printf("enrichment: count watch events for title %d: %v", titleID, err)
		return
	}
	newTotal := count * *runtime
	if err := writer.Update(ctx, titleID, repository.TitleUpdate{TotalWatchMinutes: &newTotal}); err != nil {
		log.Printf("enrichment: update total_watch_minutes for title %d: %v", titleID, err)
	}
}

// resolveAnimeConflict handles the IMDB-collision case where the pipeline
// identifies an anime that already exists under another local title. Returns
// merged=true when the source title has been consumed and the caller should
// stop processing further enrichment writes.
func (w *TaskQueueWorker) resolveAnimeConflict(ctx context.Context, result *matching.MatchResult, payload EnrichmentPayload) (bool, error) {
	if result.IMDBID == "" || !result.IsAnime {
		return false, nil
	}
	existing, err := w.titles.FindByExternalID(&result.IMDBID, nil, nil, nil, nil)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("enrichment: FindByExternalID error: %v", err)
		}
		return false, nil
	}
	if existing == nil || existing.ID == payload.TitleID || existing.Type == model.TitleTypeMovie {
		return false, nil
	}
	log.Printf("enrichment: discovered IMDB conflict (%s). Merging anime %d into %d (%s)", result.IMDBID, payload.TitleID, existing.ID, existing.Type)
	if err := w.titleSvc.Merge(ctx, w.writeDB, existing.ID, payload.TitleID, nil); err != nil {
		log.Printf("enrichment: merge failed: %v", err)
		return false, nil
	}
	log.Printf("enrichment: successfully merged title %d into %d", payload.TitleID, existing.ID)
	return true, nil
}

func (w *TaskQueueWorker) handleRefresh(ctx context.Context, task model.Task) error {
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
			details, err := w.tmdb.GetMovieDetails(ctx, *title.TMDBID)
			if err != nil {
				return err
			}
			if details.PosterPath != nil {
				coverPath, err := w.tmdb.DownloadCover(*details.PosterPath, fmt.Sprintf("%s/covers", w.dataDir))
				if err == nil {
					_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
						return repository.NewTitleWriter(tx).Update(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath})
					})
					title.CoverURL = &coverPath
				}
			}
		} else {
			details, err := w.tmdb.GetTVDetails(ctx, *title.TMDBID)
			if err != nil {
				return err
			}
			if details.PosterPath != nil {
				coverPath, err := w.tmdb.DownloadCover(*details.PosterPath, fmt.Sprintf("%s/covers", w.dataDir))
				if err == nil {
					_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
						return repository.NewTitleWriter(tx).Update(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath})
					})
					title.CoverURL = &coverPath
				}
			}
		}
	}

	// Fallback: AniList cover
	if title.CoverURL == nil && title.AniListID != nil {
		w.downloadAniListCover(ctx, title)
	}

	return nil
}

func (w *TaskQueueWorker) handleCoverFetch(ctx context.Context, task model.Task) error {
	var payload CoverFetchPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode cover_fetch payload: %w", err)
	}

	coversDir := filepath.Join(w.dataDir, "covers")

	// Try TMDB
	if w.tmdb != nil && payload.TMDBID != 0 {
		var posterPath *string
		if payload.TitleType == model.TitleTypeMovie {
			details, err := w.tmdb.GetMovieDetails(ctx, payload.TMDBID)
			if err != nil {
				return err
			}
			posterPath = details.PosterPath
		} else {
			details, err := w.tmdb.GetTVDetails(ctx, payload.TMDBID)
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
			_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
				return repository.NewTitleWriter(tx).Update(ctx, payload.TitleID, repository.TitleUpdate{CoverURL: &coverPath})
			})
			return nil
		}
	}

	// Fallback: AniList
	if w.anilist != nil && payload.AniListID != 0 {
		details, err := w.anilist.GetAnimeDetails(ctx, payload.AniListID)
		if err != nil {
			return fmt.Errorf("anilist cover fetch: %w", err)
		}
		if details.CoverURL != "" {
			coverPath, err := w.anilist.DownloadCover(details.CoverURL, coversDir)
			if err != nil {
				return fmt.Errorf("download anilist cover: %w", err)
			}
			_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
				return repository.NewTitleWriter(tx).Update(ctx, payload.TitleID, repository.TitleUpdate{CoverURL: &coverPath})
			})
		}
	}

	return nil
}

func (w *TaskQueueWorker) downloadAniListCover(ctx context.Context, title *model.Title) {
	if w.anilist == nil || title.AniListID == nil {
		return
	}

	details, err := w.anilist.GetAnimeDetails(ctx, *title.AniListID)
	if err != nil || details.CoverURL == "" {
		return
	}

	coverPath, err := w.anilist.DownloadCover(details.CoverURL, fmt.Sprintf("%s/covers", w.dataDir))
	if err != nil {
		return
	}

	_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath})
	})
	title.CoverURL = &coverPath
}

func (w *TaskQueueWorker) notifyDeadTask(task model.Task) {
	if !IsNotificationEnabled(w.settings, NotifDeadTask) {
		return
	}

	// Extract title name from payload for the notification
	titleName := "unknown"
	var ep EnrichmentPayload
	if err := json.Unmarshal([]byte(task.Payload), &ep); err == nil && ep.TitleName != "" {
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
