package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
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
	arrSvc      *ArrService     // optional — configured via SetArrService
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

// SetTMDB injects the TMDB client.
func (w *TaskQueueWorker) SetTMDB(tmdb *matching.TMDBClient) {
	if w == nil {
		return
	}
	w.tmdb = tmdb
}

// SetAniList injects the AniList client.
func (w *TaskQueueWorker) SetAniList(anilist *matching.AniListClient) {
	if w == nil {
		return
	}
	w.anilist = anilist
}

// SetPipeline injects the matching Pipeline.
func (w *TaskQueueWorker) SetPipeline(pipeline *matching.Pipeline) {
	if w == nil {
		return
	}
	w.pipeline = pipeline
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

// SetArrService wires the ArrService so the worker can process radarr/sonarr push tasks.
func (w *TaskQueueWorker) SetArrService(arrSvc *ArrService) {
	if w == nil {
		return
	}
	w.arrSvc = arrSvc
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
	case model.TaskTypeRadarrPush:
		err = w.handleArrPush(ctx, task, logger, "radarr")
	case model.TaskTypeSonarrPush:
		err = w.handleArrPush(ctx, task, logger, "sonarr")
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

// Merge runs outside the persistence tx: it opens its own tx and may
// delete the source title, so it must not nest inside another writer.

// Pipeline writes the cover file to disk and propagates only its filename
// via MatchResult.CoverFile — the actual UPDATE happened above. This is
// the single hook that covers the entire pipeline path (TMDB / TVDB /
// AniList cover branches in matching/pipeline.go).

// Enrichment writes IDs/metadata but never creates seasons — only a refresh
// does. Enqueue one now so a just-matched series shows its episode list in
// review immediately, instead of staying empty until the next periodic
// refresh cycle. Skipped when the conflict resolver merged the title away.

// enqueueSeasonBackfill schedules a refresh for a freshly-matched series so its
// seasons/episodes get populated without waiting for the periodic refresh.
// Movies have no seasons, and seasons are sourced from TMDB, so both checks
// gate the enqueue. Idempotent via the refresh:<id> dedup key.

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
	// Year is user-visible (tiles, search) but was historically only written at
	// import time, so a missing or stale value could never heal. Derive it from
	// the resolved release date on every (re)match.
	if y := releaseYear(result.ReleaseDate); y != 0 && y != payload.Year {
		update.Year = &y
	}
	return update
}

// releaseYear extracts the year from a TMDB/TVDB release date, 0 when absent
// or malformed.
func releaseYear(releaseDate string) int {
	t, err := time.Parse("2006-01-02", releaseDate)
	if err != nil {
		return 0
	}
	return t.Year()
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
//
// explicitOffset is forwarded to Merge: nil falls back to Gemini name-parsing
// for the season offset (legacy behaviour); a non-nil pointer uses that offset
// directly (a chain root colliding on IMDb is the same show's season 1 → 0).

// Guard on `existing`'s AniList id: a distinct non-zero id means `existing`
// is a different franchise member (typically a sequel sharing the parent's
// imdb). The current title must never merge INTO it — let that member's own
// enrichment merge it into the root instead. This fires even when the
// current title has no AniList id of its own (result.AniListID == 0, the
// path that reaches here): we cannot prove same-show, so leaving a standalone
// title (surfaced by the Season-audit tool) is the safe fallback, whereas a
// wrong-direction merge would silently destroy data. The merge proceeds only
// when `existing` has no/0 AniList id (a legacy/Plex duplicate) or the same
// id as the current title (a true duplicate).

// seasonActionKind enumerates the outcomes of the season-attachment decision.
type seasonActionKind int

const (
	seasonActionNone       seasonActionKind = iota // leave standalone
	seasonActionLegacy                             // IMDb-collision path, Gemini-inferred offset (nil)
	seasonActionLegacyRoot                         // IMDb-collision path, explicit offset 0
	seasonActionMergeInto                          // merge into ParentID with Offset
	seasonActionCreateRoot                         // create the root, then merge with Offset
)

// seasonAction is the decision produced by decideSeasonAction.
type seasonAction struct {
	Kind     seasonActionKind
	ParentID int64
	Offset   int
}

// decideSeasonAction encodes the franchise-protection rules for relations-driven
// season attachment. It is pure (no DB/HTTP) so the rule table is unit-testable.
//
//   - chain == nil (resolve failed) or chain.IsRoot → fall back to the legacy
//     IMDb-collision path. A root colliding on IMDb is the same show's season 1,
//     so it gets an explicit offset 0 (legacyRoot); a failed resolve keeps the
//     Gemini-inferred offset (legacy).
//   - A non-series root is never a merge target — a TV season must not attach to
//     a movie/special root.
//   - parentByIDs (found by the entry's OWN IMDb/TMDB id) proves same-show:
//     Simkl season entries carry the PARENT's id, so a shared id == same show →
//     always safe to merge.
//   - parentByRoot (found by the root AniList id) is relations-only evidence:
//     merge ONLY when the entry has no external identity of its own. Otherwise
//     it is a distinct franchise member (e.g. Dragon Ball Z has its own IMDb)
//     and must stay standalone.
//   - No parent found: create the root and merge, but again only when the entry
//     has no identity of its own (same protection).
func decideSeasonAction(chain *matching.SeasonChain, result *matching.MatchResult, parentByIDs, parentByRoot *model.Title) seasonAction {
	if chain == nil {
		return seasonAction{Kind: seasonActionLegacy}
	}
	if chain.IsRoot {
		return seasonAction{Kind: seasonActionLegacyRoot, Offset: 0}
	}
	if !chain.RootIsSeries {
		// Never merge a season into a movie/special root.
		return seasonAction{Kind: seasonActionNone}
	}

	offset := chain.SeasonNumber - 1

	// Shared external id proves same-show: always safe to merge. Prefer the
	// AniList chain root (parentByRoot, looked up by the unique root AniList id)
	// as the target so 3+ cours sharing one parent imdb all merge into the true
	// root — parentByIDs is a LIMIT-1 imdb match and may point at a sibling.
	if parentByIDs != nil {
		target := parentByIDs
		if parentByRoot != nil {
			target = parentByRoot
		}
		return seasonAction{Kind: seasonActionMergeInto, ParentID: target.ID, Offset: offset}
	}

	hasOwnIdentity := result.IMDBID != "" || result.TMDBID != 0 || result.TVDBID != 0

	// Relations-only evidence: attach only an entry without its own identity.
	if parentByRoot != nil {
		if hasOwnIdentity {
			return seasonAction{Kind: seasonActionNone}
		}
		return seasonAction{Kind: seasonActionMergeInto, ParentID: parentByRoot.ID, Offset: offset}
	}

	// No parent exists yet. Create the root only for an identity-less entry.
	if hasOwnIdentity {
		return seasonAction{Kind: seasonActionNone}
	}
	return seasonAction{Kind: seasonActionCreateRoot, Offset: offset}
}

// resolveAnimeSeason auto-attaches an anime season to its parent series using
// AniList PREQUEL relations, with id-safety guards against merging distinct
// franchise members. Returns merged=true when the source title was consumed
// (so the caller skips season backfill). Non-anime / no-AniList-id titles, and
// any resolve/lookup failure, fall back to the legacy IMDb-collision path.

// Rule 1: not anime or no AniList id → legacy behaviour, unchanged.

// Rule 2: resolve failure (incl. nil pipeline) → legacy behaviour.

// Parent lookups (only the meaningful ones once we have a season entry).

// Always resolve the chain root by its (unique) AniList id so
// decideSeasonAction can target it deterministically, even when
// parentByIDs already found a same-show sibling (3+ cours case).

// attachSeason merges the source title into parentID with the given offset and,
// on success, records a season_attached event on the parent. parentNameOverride
// is used when the parent was just created (its primary name isn't queryable
// via GetByID in the same flow); empty means fetch the parent's primary name.

// Fetch the source name BEFORE Merge — Merge deletes the source row.

// createParentAndAttach creates a new root series title for the chain, enqueues
// its enrichment, then merges the source season into it. The source's Year is
// copied so the new parent isn't year-zero when AniList enrichment is pending.

// Enqueue enrichment for the freshly-created parent so it gains metadata.

// Try TMDB cover

// Fallback: AniList cover

// logger carries taskID/type and would be enriched with titleID once
// handleCoverFetch grows actual log lines.

// Try TMDB

// Fallback: AniList

// logger carries taskID/type and would be enriched with seasonID once
// PushSeasonState accepts a logger (phase 4+).

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
		"Trackarr",
		fmt.Sprintf("Task failed — Unable to process: %s", titleName),
		"/admin/tasks",
	); err != nil {
		logger.Error("dead-task push failed", "err", err)
	}
}

// isSearchSource reports whether the match came from a search strategy — the
// only sources where auto-confirmation is a decision worth surfacing (ID-based
// sources were already confirmed by construction).
func isSearchSource(source string) bool {
	switch source {
	case matching.MatchSourceTMDBSearch, matching.MatchSourceAniListSearch, matching.MatchSourceGeminiFuzzy:
		return true
	}
	return false
}

// resolvedName returns the resolved primary name from the pipeline result.
// TMDB-enriched names are appended after any input-seeded fallback, so the
// last primary English name is the most authoritative resolved title.
// Falls back to the first name in the list, then to the payload title.
func resolvedName(result *matching.MatchResult, payload EnrichmentPayload) string {
	// Prefer the last primary name — pipeline appends TMDB names after any
	// input fallback, so the last primary entry is the most authoritative.
	last := ""
	for _, n := range result.Names {
		if n.IsPrimary && n.Name != "" {
			last = n.Name
		}
	}
	if last != "" {
		return last
	}
	if len(result.Names) > 0 {
		return result.Names[0].Name
	}
	return payload.TitleName
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
