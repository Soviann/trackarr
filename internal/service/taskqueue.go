package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"path/filepath"
	"sync"
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
	AniListID int64           `json:"anilist_id,omitempty"`
	// LockedIDs lists external-ID fields this enrichment run must NOT write
	// (values from LockIMDB/LockTMDB/LockTVDB/LockAniList). The manual ID editor
	// sets these so a user-provided or deliberately-emptied ID is never
	// overwritten or back-filled by auto-matching. Empty = enrich every field.
	LockedIDs []string `json:"locked_ids,omitempty"`
	// PreserveMatch keeps the title's current match_status/match_source instead
	// of overwriting them with the pipeline result — set when the run follows a
	// manual edit so the "matched manually" state survives a metadata refresh.
	PreserveMatch bool `json:"preserve_match,omitempty"`
}

// External-ID field keys for EnrichmentPayload.LockedIDs.
const (
	LockIMDB    = "imdb"
	LockTMDB    = "tmdb"
	LockTVDB    = "tvdb"
	LockAniList = "anilist"
)

type RefreshPayload struct {
	TitleID int64 `json:"title_id"`
}

type CoverFetchPayload struct {
	TitleID   int64           `json:"title_id"`
	TMDBID    int64           `json:"tmdb_id"`
	AniListID int64           `json:"anilist_id,omitempty"`
	TitleType model.TitleType `json:"title_type"`
}

// AniListPushSeasonPayload carries the season whose state must be pushed to
// AniList. The push service re-derives status/progress/rating from the DB at
// run time, so older queued jobs still send an up-to-date snapshot.
type AniListPushSeasonPayload struct {
	SeasonID int64 `json:"season_id"`
}

// AniListPushMoviePayload carries the anime movie title whose state must be
// pushed to AniList. The push uses titles.anilist_id directly.
type AniListPushMoviePayload struct {
	TitleID int64 `json:"title_id"`
}

// AniListPusher is the subset of AniListPushService the worker depends on.
// Kept narrow so tests can inject a fake without wiring the real HTTP client.
type AniListPusher interface {
	PushSeasonState(ctx context.Context, seasonID int64) error
	PushMovieState(ctx context.Context, titleID int64) error
}

// TaskQueueWorker processes retryable tasks from the queue.
type TaskQueueWorker struct {
	log         *slog.Logger
	tasks       *repository.TaskRepository
	titles      *repository.TitleRepository
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
	anilistPush AniListPusher   // optional — configured via SetAniListPush when an AniList client is wired
	covers      *CoverService   // optional — configured via SetCovers; drives accent extraction after every cover save
	shutdownWG  *sync.WaitGroup // optional — joined on shutdown so the worker loop can finish its poll
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
	titleSvc *TitleService,
	writeDB *sql.DB,
) *TaskQueueWorker {
	return &TaskQueueWorker{
		log:      slog.With("worker", "taskqueue"),
		tasks:    tasks,
		titles:   titles,
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

// SetShutdownWG registers a WaitGroup the worker loop increments on start and
// decrements on exit, so Serve() can wait for any in-flight poll iteration
// before closing the database.
func (w *TaskQueueWorker) SetShutdownWG(wg *sync.WaitGroup) {
	if w == nil {
		return
	}
	w.shutdownWG = wg
}

// SetAPILimiter replaces the default limiter with a shared one so task queue
// processing shares the 2rps budget with BackgroundService and CoverService.
func (w *TaskQueueWorker) SetAPILimiter(limiter *APILimiter) {
	if w == nil || limiter == nil {
		return
	}
	w.limiter = limiter
}

// SetAniListPush wires the AniList push service so the worker can process
// anilist_push_season / anilist_push_movie tasks. Left unset, those task
// kinds fail fast with a descriptive error instead of silently dropping.
func (w *TaskQueueWorker) SetAniListPush(push AniListPusher) {
	if w == nil {
		return
	}
	w.anilistPush = push
}

// SetCovers wires the CoverService so the worker can run accent extraction
// after each cover save. Optional — left unset, cover saves still succeed
// but no accent is computed (tests rely on this nil-tolerance).
func (w *TaskQueueWorker) SetCovers(covers *CoverService) {
	if w == nil {
		return
	}
	w.covers = covers
}

// Start launches the worker loop. It polls for due tasks every 30 seconds.
func (w *TaskQueueWorker) Start(ctx context.Context) {
	// Rescue stuck tasks from previous crashes
	if err := database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		return repository.NewTaskWriter(tx).ResetRunning(ctx)
	}); err != nil {
		w.log.Error("reset running tasks at startup", "err", err)
	}

	// Log queue state at startup
	counts, err := w.tasks.CountByStatus()
	if err == nil {
		total := 0
		for _, c := range counts {
			total += c
		}
		if total > 0 {
			w.log.Info("queue state at startup",
				"pending", counts[model.TaskStatusPending]+counts[model.TaskStatusRunning],
				"sleeping", counts[model.TaskStatusSleeping],
				"dead", counts[model.TaskStatusDead],
			)
		}
	}

	if w.shutdownWG != nil {
		w.shutdownWG.Add(1)
	}
	go func() {
		if w.shutdownWG != nil {
			defer w.shutdownWG.Done()
		}
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						w.log.Error("panic in worker loop", "panic", r)
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
			w.log.Error("panic in processDueTasks", "panic", r)
		}
	}()

	if time.Now().Before(w.pausedUntil) {
		return
	}

	var tasks []model.Task
	if err := database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		var e error
		tasks, e = repository.NewTaskWriter(tx).FetchDue(ctx, 10)
		return e
	}); err != nil {
		w.log.Error("fetch due tasks", "err", err)
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
	logger := w.log.With(
		"taskID", task.ID,
		"type", task.TaskType,
	)
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic processing task", "panic", r)
			_ = database.WithTxContext(context.Background(), w.writeDB, func(tx *sql.Tx) error {
				return repository.NewTaskWriter(tx).Fail(context.Background(), task.ID, fmt.Sprintf("panic: %v", r), time.Now().Add(time.Hour))
			})
		}
	}()

	var err error

	switch task.TaskType {
	case model.TaskTypeEnrichment:
		err = w.handleEnrichment(ctx, task, logger)
	case model.TaskTypeRefresh:
		err = w.handleRefresh(ctx, task, logger)
	case model.TaskTypeCoverFetch:
		err = w.handleCoverFetch(ctx, task, logger)
	case model.TaskTypeAniListPushSeason:
		err = w.handleAniListPushSeason(ctx, task, logger)
	case model.TaskTypeAniListPushMovie:
		err = w.handleAniListPushMovie(ctx, task, logger)
	default:
		logger.Warn("unknown task type", "taskType", task.TaskType)
		bookkeepCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = database.WithTxContext(bookkeepCtx, w.writeDB, func(tx *sql.Tx) error {
			return repository.NewTaskWriter(tx).Delete(bookkeepCtx, task.ID)
		})
		cancel()
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
			logger.Warn("rate limit hit, pausing worker", "until", w.pausedUntil.Format("15:04:05"))
		}

		nextRunAt := calculateNextRunAt(task.Attempts+1, retryAfter)
		// Task bookkeeping must run on a fresh context: ctx might already be
		// canceled (e.g. HTTP client disconnected) and leaving the row in
		// 'running' would let a retry deadlock start-up's ResetRunning.
		bookkeepCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if failErr := database.WithTxContext(bookkeepCtx, w.writeDB, func(tx *sql.Tx) error {
			return repository.NewTaskWriter(tx).Fail(bookkeepCtx, task.ID, err.Error(), nextRunAt)
		}); failErr != nil {
			logger.Error("mark task failed", "err", failErr)
		}
		cancel()

		// Check if task just died (day 7 + last attempt)
		if task.Day >= 7 && task.Attempts+1 >= task.MaxAttempts {
			w.notifyDeadTask(ctx, task, logger)
		}

		return
	}
	// Complete must run on a fresh context for the same reason as Fail above.
	bookkeepCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := database.WithTxContext(bookkeepCtx, w.writeDB, func(tx *sql.Tx) error {
		return repository.NewTaskWriter(tx).Complete(bookkeepCtx, task.ID)
	}); err != nil {
		logger.Error("mark task complete", "err", err)
	}
}

func (w *TaskQueueWorker) handleEnrichment(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload EnrichmentPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode enrichment payload: %w", err)
	}
	logger = logger.With("titleID", payload.TitleID)

	if w.pipeline == nil {
		return fmt.Errorf("pipeline not configured")
	}

	result, err := w.pipeline.Run(ctx, matching.MatchInput{
		Title:     payload.TitleName,
		Year:      payload.Year,
		Type:      payload.TitleType,
		IsAnime:   payload.IsAnime,
		IMDBID:    payload.IMDBID,
		TMDBID:    payload.TMDBID,
		TVDBID:    payload.TVDBID,
		AniListID: payload.AniListID,
	})
	if err != nil {
		return err
	}

	update := buildEnrichmentUpdate(result, payload)

	err = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		titlesTx := repository.NewTitleWriter(tx)
		genresTx := repository.NewGenreWriter(tx)

		if err := titlesTx.Update(ctx, payload.TitleID, update); err != nil {
			return err
		}

		recalcWatchtime(ctx, tx, logger, payload.TitleID, result.Runtime)

		if len(result.Names) > 0 {
			if err := titlesTx.ReplaceNames(ctx, payload.TitleID, result.Names); err != nil {
				logger.Warn("replace title names", "err", err)
			}
		}

		if len(result.Genres) > 0 {
			if err := genresTx.ReplaceForTitle(ctx, payload.TitleID, result.Genres); err != nil {
				logger.Warn("save genres", "err", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Merge runs outside the persistence tx: it opens its own tx and may
	// delete the source title, so it must not nest inside another writer.
	merged, err := w.resolveAnimeConflict(ctx, result, payload, logger)
	if err != nil {
		return err
	}

	// Pipeline writes the cover file to disk and propagates only its filename
	// via MatchResult.CoverFile — the actual UPDATE happened above. This is
	// the single hook that covers the entire pipeline path (TMDB / TVDB /
	// AniList cover branches in matching/pipeline.go).
	if result.CoverFile != "" {
		w.covers.ExtractAndStoreAccent(ctx, payload.TitleID, result.CoverFile)
	}

	// Enrichment writes IDs/metadata but never creates seasons — only a refresh
	// does. Enqueue one now so a just-matched series shows its episode list in
	// review immediately, instead of staying empty until the next periodic
	// refresh cycle. Skipped when the conflict resolver merged the title away.
	if !merged {
		w.enqueueSeasonBackfill(ctx, result, payload, logger)
	}

	return nil
}

// enqueueSeasonBackfill schedules a refresh for a freshly-matched series so its
// seasons/episodes get populated without waiting for the periodic refresh.
// Movies have no seasons, and seasons are sourced from TMDB, so both checks
// gate the enqueue. Idempotent via the refresh:<id> dedup key.
func (w *TaskQueueWorker) enqueueSeasonBackfill(ctx context.Context, result *matching.MatchResult, payload EnrichmentPayload, logger *slog.Logger) {
	titleType := result.TitleType
	if titleType == "" {
		titleType = payload.TitleType
	}
	if titleType == model.TitleTypeMovie {
		return
	}
	tmdbID := result.TMDBID
	if tmdbID == 0 {
		tmdbID = payload.TMDBID
	}
	if tmdbID == 0 {
		return
	}

	data, err := json.Marshal(RefreshPayload{TitleID: payload.TitleID})
	if err != nil {
		logger.Warn("marshal season backfill payload", "err", err)
		return
	}
	dedupKey := fmt.Sprintf("refresh:%d", payload.TitleID)
	if err := database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		_, e := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeRefresh, string(data), &dedupKey)
		return e
	}); err != nil {
		logger.Warn("enqueue season backfill refresh", "err", err)
	}
}

// buildEnrichmentUpdate translates a pipeline MatchResult into a TitleUpdate
// diff. Pure: no side effects, safe to call outside a transaction.
func buildEnrichmentUpdate(result *matching.MatchResult, payload EnrichmentPayload) repository.TitleUpdate {
	locked := make(map[string]bool, len(payload.LockedIDs))
	for _, k := range payload.LockedIDs {
		locked[k] = true
	}

	update := repository.TitleUpdate{}
	// A manual edit owns the match state; only a fresh (re)match rewrites it.
	if !payload.PreserveMatch {
		update.MatchStatus = &result.MatchStatus
		update.MatchSource = &result.MatchSource
	}
	if result.IMDBID != "" && !locked[LockIMDB] {
		update.IMDBID = &result.IMDBID
	}
	if result.TMDBID != 0 && !locked[LockTMDB] {
		update.TMDBID = &result.TMDBID
	}
	if result.TVDBID != 0 && !locked[LockTVDB] {
		update.TVDBID = &result.TVDBID
	}
	if result.AniListID != 0 && !locked[LockAniList] {
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
func recalcWatchtime(ctx context.Context, tx *sql.Tx, logger *slog.Logger, titleID int64, runtime *int) {
	if runtime == nil {
		return
	}
	events := repository.NewWatchEventRepository(tx)
	writer := repository.NewTitleWriter(tx)
	count, err := events.CountByTitleID(titleID)
	if err != nil {
		logger.Warn("count watch events", "err", err)
		return
	}
	newTotal := count * *runtime
	if err := writer.Update(ctx, titleID, repository.TitleUpdate{TotalWatchMinutes: &newTotal}); err != nil {
		logger.Warn("update total_watch_minutes", "err", err)
	}
}

// resolveAnimeConflict handles the IMDB-collision case where the pipeline
// identifies an anime that already exists under another local title. Returns
// merged=true when the source title has been consumed and the caller should
// stop processing further enrichment writes.
func (w *TaskQueueWorker) resolveAnimeConflict(ctx context.Context, result *matching.MatchResult, payload EnrichmentPayload, logger *slog.Logger) (bool, error) {
	if result.IMDBID == "" || !result.IsAnime {
		return false, nil
	}
	existing, err := w.titles.FindByExternalID(&result.IMDBID, nil, nil, nil, nil)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logger.Warn("FindByExternalID", "err", err)
		}
		return false, nil
	}
	if existing == nil || existing.ID == payload.TitleID || existing.Type == model.TitleTypeMovie {
		return false, nil
	}
	logger.Info("IMDB conflict, merging anime",
		"imdbID", result.IMDBID,
		"intoTitleID", existing.ID,
		"existingType", existing.Type,
	)
	if err := w.titleSvc.Merge(ctx, w.writeDB, existing.ID, payload.TitleID, nil); err != nil {
		logger.Error("merge after IMDB conflict", "err", err)
		return false, nil
	}
	logger.Info("merged title after IMDB conflict", "intoTitleID", existing.ID)
	return true, nil
}

func (w *TaskQueueWorker) handleRefresh(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload RefreshPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode refresh payload: %w", err)
	}
	logger = logger.With("titleID", payload.TitleID)

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
				coverPath, err := w.tmdb.DownloadCover(ctx, *details.PosterPath, fmt.Sprintf("%s/covers", w.dataDir))
				if err == nil {
					_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
						return repository.NewTitleWriter(tx).Update(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath})
					})
					w.covers.ExtractAndStoreAccent(ctx, title.ID, coverPath)
					title.CoverURL = &coverPath
				}
			}
		} else {
			details, err := w.tmdb.GetTVDetails(ctx, *title.TMDBID)
			if err != nil {
				return err
			}
			if details.PosterPath != nil {
				coverPath, err := w.tmdb.DownloadCover(ctx, *details.PosterPath, fmt.Sprintf("%s/covers", w.dataDir))
				if err == nil {
					_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
						return repository.NewTitleWriter(tx).Update(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath})
					})
					w.covers.ExtractAndStoreAccent(ctx, title.ID, coverPath)
					title.CoverURL = &coverPath
				}
			}
		}
	}

	// Fallback: AniList cover
	if title.CoverURL == nil && title.AniListID != nil {
		w.downloadAniListCover(ctx, title, logger)
	}

	return nil
}

func (w *TaskQueueWorker) handleCoverFetch(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload CoverFetchPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode cover_fetch payload: %w", err)
	}
	// logger carries taskID/type and would be enriched with titleID once
	// handleCoverFetch grows actual log lines.
	_ = logger

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
			coverPath, err := w.tmdb.DownloadCover(ctx, *posterPath, coversDir)
			if err != nil {
				return err
			}
			_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
				return repository.NewTitleWriter(tx).Update(ctx, payload.TitleID, repository.TitleUpdate{CoverURL: &coverPath})
			})
			w.covers.ExtractAndStoreAccent(ctx, payload.TitleID, coverPath)
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
			coverPath, err := w.anilist.DownloadCover(ctx, details.CoverURL, coversDir)
			if err != nil {
				return fmt.Errorf("download anilist cover: %w", err)
			}
			_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
				return repository.NewTitleWriter(tx).Update(ctx, payload.TitleID, repository.TitleUpdate{CoverURL: &coverPath})
			})
			w.covers.ExtractAndStoreAccent(ctx, payload.TitleID, coverPath)
		}
	}

	return nil
}

func (w *TaskQueueWorker) downloadAniListCover(ctx context.Context, title *model.Title, logger *slog.Logger) {
	if w.anilist == nil || title.AniListID == nil {
		return
	}

	details, err := w.anilist.GetAnimeDetails(ctx, *title.AniListID)
	if err != nil || details.CoverURL == "" {
		return
	}

	coverPath, err := w.anilist.DownloadCover(ctx, details.CoverURL, fmt.Sprintf("%s/covers", w.dataDir))
	if err != nil {
		return
	}

	_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath})
	})
	w.covers.ExtractAndStoreAccent(ctx, title.ID, coverPath)
	title.CoverURL = &coverPath
}

func (w *TaskQueueWorker) handleAniListPushSeason(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload AniListPushSeasonPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode anilist_push_season payload: %w", err)
	}
	if w.anilistPush == nil {
		return fmt.Errorf("anilist push service not configured")
	}
	// logger carries taskID/type and would be enriched with seasonID once
	// PushSeasonState accepts a logger (phase 4+).
	_ = logger
	return w.anilistPush.PushSeasonState(ctx, payload.SeasonID)
}

func (w *TaskQueueWorker) handleAniListPushMovie(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload AniListPushMoviePayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode anilist_push_movie payload: %w", err)
	}
	if w.anilistPush == nil {
		return fmt.Errorf("anilist push service not configured")
	}
	_ = logger
	return w.anilistPush.PushMovieState(ctx, payload.TitleID)
}

func (w *TaskQueueWorker) notifyDeadTask(ctx context.Context, task model.Task, logger *slog.Logger) {
	if !IsNotificationEnabled(w.settings, NotifDeadTask) {
		return
	}

	// Extract title name from payload for the notification
	titleName := "unknown"
	var ep EnrichmentPayload
	if err := json.Unmarshal([]byte(task.Payload), &ep); err == nil && ep.TitleName != "" {
		titleName = ep.TitleName
	}

	if err := w.push.SendNotification(
		ctx,
		"PlexTracker",
		fmt.Sprintf("Task failed — Unable to process: %s", titleName),
		"/admin/tasks",
	); err != nil {
		logger.Error("dead-task push failed", "err", err)
	}
}

// calculateNextRunAt computes the next retry time with exponential backoff + jitter.
// Base delay: 30s, max: 1 hour. Jitter: 0-25% of calculated delay.
func calculateNextRunAt(attempts int, retryAfter time.Duration) time.Time {
	base := 30 * time.Second
	delay := time.Duration(float64(base) * math.Pow(2, float64(attempts-1)))

	delay = min(delay, time.Hour)

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
