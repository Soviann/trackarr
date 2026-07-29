package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

// aniListSeasonScoreClient is the subset of matching.AniListClient needed to
// refresh per-season community scores. Narrowing the dependency keeps the
// background service trivially testable without spinning up an httptest server.
type aniListSeasonScoreClient interface {
	GetAnimeDetails(ctx context.Context, anilistID int64) (*matching.AniListDetails, error)
	SearchAnime(ctx context.Context, query string) ([]matching.AniListSearchResult, error)
}

type BackgroundService struct {
	writeDB      *sql.DB
	titles       *repository.TitleRepository
	seasonExtIDs *repository.SeasonExternalIDRepository
	tvdb         *matching.TVDBClient     // optional — nil if TVDB_API_KEY not set
	anilist      aniListSeasonScoreClient // optional — nil disables per-season AniList score refresh
	settings     *repository.SettingRepository
	tmdb         *matching.TMDBClient
	covers       *CoverService
	push         PushNotifier
	limiter      *APILimiter
	shutdownWG   *sync.WaitGroup // optional — joined on shutdown so the ticker goroutine can finish its iteration
}

func NewBackgroundService(
	writeDB *sql.DB,
	titles *repository.TitleRepository,
	settings *repository.SettingRepository,
	tmdb *matching.TMDBClient,
	covers *CoverService,
	push PushNotifier,
) *BackgroundService {
	return &BackgroundService{
		writeDB:      writeDB,
		titles:       titles,
		seasonExtIDs: repository.NewSeasonExternalIDRepository(writeDB),
		settings:     settings,
		tmdb:         tmdb,
		covers:       covers,
		push:         push,
		limiter:      NewAPILimiter(2, 1),
	}
}

// SetShutdownWG registers a WaitGroup the ticker goroutine increments on start
// and decrements on exit, so Serve() can wait for the current iteration to
// finish before closing the database.
func (s *BackgroundService) SetShutdownWG(wg *sync.WaitGroup) {
	if s == nil {
		return
	}
	s.shutdownWG = wg
}

// SetAPILimiter replaces the default limiter with a shared one so background
// refresh, cover fetch and the task queue worker all share a single 2rps budget
// against TMDB/AniList instead of 3 × 2rps competing in parallel.
func (s *BackgroundService) SetAPILimiter(limiter *APILimiter) {
	if s == nil || limiter == nil {
		return
	}
	s.limiter = limiter
}

// updateTitle wraps a title update in its own short transaction. Each
// background refresh step is persisted independently so a crash between
// steps leaves the DB in a valid state: the refresh loop is idempotent and
// re-runs what is still missing.
func (s *BackgroundService) updateTitle(ctx context.Context, id int64, update repository.TitleUpdate) error {
	return database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(ctx, id, update)
	})
}

// SetTVDB injects the TVDB client (optional — called after construction if TVDB is configured).
func (s *BackgroundService) SetTVDB(tvdb *matching.TVDBClient) { s.tvdb = tvdb }

// SetAniList injects the AniList client used by the daily refresh to fetch
// per-season community scores. Optional — when nil the per-season AniList
// score refresh step is skipped.
func (s *BackgroundService) SetAniList(client aniListSeasonScoreClient) {
	if s == nil {
		return
	}
	s.anilist = client
}

// RefreshResult captures what happened for a single title during refresh.
type RefreshResult struct {
	TitleID       int64
	TitleName     string
	AutoCompleted bool
	StatusChanged bool
	OldStatus     model.SeriesStatus
	NewStatus     model.SeriesStatus
	Error         error
	// Refreshed is true when at least one external API (TMDB / TVDB / AniList)
	// answered successfully for this title. Drives last_refreshed_at: we want
	// it to stay frozen on network errors so a stale title is visible as such.
	Refreshed bool
}

// RefreshTitles processes all non-completed titles.
// Returns a result per processed title.
func (s *BackgroundService) RefreshTitles(ctx context.Context) []RefreshResult {
	return s.refreshTitles(ctx, false)
}

// RefreshAllTitles processes all titles regardless of status.
// Used to backfill metadata on existing titles.
func (s *BackgroundService) RefreshAllTitles(ctx context.Context) []RefreshResult {
	return s.refreshTitles(ctx, true)
}

// RefreshByID refreshes metadata for a single title.
func (s *BackgroundService) RefreshByID(ctx context.Context, titleID int64) error {
	title, err := s.titles.GetLiteByID(ctx, titleID)
	if err != nil {
		return fmt.Errorf("background: get title %d: %w", titleID, err)
	}
	s.refreshTitle(ctx, title)
	return nil
}

func (s *BackgroundService) refreshTitles(ctx context.Context, includeAll bool) []RefreshResult {
	if s == nil {
		return nil
	}

	titles, err := s.titles.ListAllForRefresh(ctx)
	if err != nil {
		log.Printf("background: list titles: %v", err)
		return nil
	}

	results := make([]RefreshResult, 0, len(titles))

	for i := range titles {
		if err := ctx.Err(); err != nil {
			log.Printf("background: refresh cancelled: %v", err)
			return results
		}

		title := &titles[i]
		if !includeAll && (title.Status == model.TitleStatusCompleted || title.Status == model.TitleStatusDropped) && !needsEpisodeBackfill(title) {
			continue
		}

		result := s.refreshTitle(ctx, title)
		results = append(results, result)

		_ = s.limiter.Wait(ctx)
	}

	return results
}

// needsEpisodeBackfill reports whether a completed/dropped title should be
// refreshed anyway to fetch its episode list. The daily cron normally skips
// completed/dropped titles, but one that was never TMDB-synced (no
// total_episodes on any season) has no episode list at all — a Simkl-imported
// "completed" series, or one only ever touched by scrobbles. Restricted to
// non-movies with a TMDB id, the only source that can supply the list; without
// it the title would be re-processed fruitlessly on every pass.
func needsEpisodeBackfill(t *repository.TitleLite) bool {
	return t.Type != model.TitleTypeMovie && t.TMDBID != nil && !t.HasSyncedSeasons
}

func (s *BackgroundService) refreshTitle(ctx context.Context, title *repository.TitleLite) RefreshResult {
	result := RefreshResult{
		TitleID:   title.ID,
		TitleName: title.PrimaryName,
	}

	// Step 1: Refresh from TMDB if available
	if s.tmdb != nil && title.TMDBID != nil {
		s.refreshFromTMDB(ctx, title, &result)
	}

	// Step 1b: AniList as the full metadata source when there's no TMDB — niche
	// anime that aren't on TMDB. Sources names/synopsis/genres/rating/cover from
	// AniList (subsumes the old cover-only fallback).
	if title.TMDBID == nil && title.AniListID != nil {
		s.refreshFromAniList(ctx, title, &result)
	}

	// Step 1c: TVDB enrichment — fetch rating, cover fallback, and tvdb_id cross-ref
	if s.tvdb != nil {
		s.refreshFromTVDB(ctx, title, &result)
	}

	// Step 1d: AniList per-season community score (anime only). Each mapped
	// season produces one AniList Media-by-id call against the public
	// (unauthenticated) GraphQL endpoint. Errors are logged per season —
	// one bad mapping never breaks the rest of the refresh.
	if s.anilist != nil && title.IsAnime {
		s.refreshAniListSeasonScores(ctx, title, &result)
	}

	// Step 1e: AniList ID auto-backfill for anime titles missing an AniList link.
	if s.anilist != nil && title.IsAnime && title.AniListID == nil {
		s.backfillAniListID(ctx, title, &result)
	}

	// Step 2: Auto-complete if series ended and all episodes watched
	if title.Type != model.TitleTypeMovie && title.SeriesStatus != nil {
		if *title.SeriesStatus == model.SeriesStatusEnded || *title.SeriesStatus == model.SeriesStatusCancelled {
			hasUnwatched, err := s.titles.HasUnwatchedEpisodes(title.ID)
			if err == nil && !hasUnwatched {
				completed := model.TitleStatusCompleted
				if err := s.updateTitle(ctx, title.ID, repository.TitleUpdate{Status: &completed}); err == nil {
					result.AutoCompleted = true
					log.Printf("background: auto-completed %q", result.TitleName)
				}
			}
		}
	}

	// Step 2b: enforce "a completed series has every episode watched". Runs after
	// Step 1 has (re)populated the list, so episodes freshly backfilled for an
	// import-completed title get marked too. Idempotent (only flips unwatched rows).
	if title.Type != model.TitleTypeMovie && title.Status == model.TitleStatusCompleted {
		s.completeEpisodes(ctx, title.ID)
	}

	// Step 3: stamp last_refreshed_at only if at least one external API
	// actually answered. Network failures, rate limits, or "no external IDs"
	// leave the timestamp frozen — that's the signal a user reads to decide
	// whether a manual refresh might surface new episodes.
	if result.Refreshed {
		if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
			return repository.NewTitleWriter(tx).MarkRefreshed(ctx, title.ID, time.Now().UTC())
		}); err != nil {
			log.Printf("background: mark refreshed for title %d: %v", title.ID, err)
		}
	}

	return result
}

// completeEpisodes marks every not-yet-watched episode of a completed title as
// watched and accrues their watchtime. No watch_events are written: these are
// historical/backfilled completions, so they must not pollute the activity feed
// or fabricate viewing streaks (which read watch_events), while still counting
// toward total watchtime (which sums titles.total_watch_minutes).
func (s *BackgroundService) completeEpisodes(ctx context.Context, titleID int64) {
	now := time.Now().UTC()
	if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		n, err := repository.NewEpisodeWriter(tx).MarkAllWatchedForTitle(ctx, titleID, now)
		if err != nil {
			return err
		}
		return repository.NewTitleWriter(tx).AddWatchMinutesForEpisodes(ctx, titleID, n)
	}); err != nil {
		log.Printf("background: complete episodes for title %d: %v", titleID, err)
	}
}

// syncTitleNames backfills alternate-language names for a title from one or more
// source maps (language -> name), inserting only entries not already stored.
// Never deletes, so anime romaji and merged-season aliases survive a refresh.
// Best-effort: failures are logged, not propagated.
func (s *BackgroundService) syncTitleNames(ctx context.Context, titleID int64, sources ...map[string]string) {
	var names []model.TitleName
	for _, src := range sources {
		for lang, name := range src {
			if name == "" {
				continue
			}
			names = append(names, model.TitleName{Name: name, Language: lang})
		}
	}
	if len(names) == 0 {
		return
	}
	if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).AddMissingNames(ctx, titleID, names)
	}); err != nil {
		log.Printf("background: backfill names for title %d: %v", titleID, err)
	}
}

// logTitleUpdate logs an error from a title update if non-nil.
func logTitleUpdate(titleID int64, kind string, err error) {
	if err != nil {
		log.Printf("background: update %s for title %d: %v", kind, titleID, err)
	}
}

func (s *BackgroundService) hasValidCover(title *repository.TitleLite) bool {
	if title == nil || title.CoverURL == nil || *title.CoverURL == "" {
		return false
	}
	if s.covers != nil && !s.covers.HasCoverFile(*title.CoverURL) {
		return false
	}
	return true
}

// refreshFromTVDB fetches TVDB data for titles that have a TVDB ID.
// TVDB ID cross-referencing from TMDB is handled in refreshMovieFromTMDB / refreshSeriesFromTMDB.
// For titles with a TMDB ID, overview and genres are refreshed from TMDB; here only the cover is updated.
// For titles without a TMDB ID, overview and genres are also persisted from TVDB.

// Update cover if missing or file deleted on disk

// Update metadata from TMDB details

// Backfill multilingual names (en/fr) — re-syncs translations missing on
// titles matched before translations were captured. Additive, so the
// anime romaji name set by matching survives.

// Persist genres to title_genres table

// Fallback: AniList cover

// Cross-reference TVDB ID if not yet stored (avoids a duplicate TMDB fetch in refreshFromTVDB)

// Detect series status change

// Update cover if missing or file deleted on disk

// Update metadata from TMDB details

// Populate next_air_date and next_air_episode from TMDB next_episode_to_air

// Backfill multilingual names (en/fr) — see refreshMovieFromTMDB.

// Persist genres to title_genres table

// Fallback: AniList cover

// Backfill TVDB ID early if available from TMDB external IDs

// Sync seasons and episodes — prefer TVDB if available

// Fallback: sync seasons and episodes from TMDB

// Skip specials

// Fetch individual episodes outside the write transaction to keep
// TMDB HTTP latency off the sole write connection.

// refreshSeriesFromTVDB syncs season and episode listings from TVDB.
// Returns true if TVDB season sync succeeded.

// Skip specials

// refreshFromAniList sources a title's metadata (names, synopsis, genres,
// rating, runtime, cover) from AniList. Used for titles that have an AniList ID
// but no TMDB — niche anime absent from TMDB. AniList is then the authority, so
// it OVERWRITES existing values (which are often stale, left over from a removed
// wrong TMDB match). Best-effort: each piece is logged and skipped on failure.

// Names: English primary (fall back to romaji), romaji as alternate.

// Cover from AniList when missing. SetExternalIDs clears the stale cover when
// the TMDB/TVDB sources are removed, so this fills it on the next refresh
// without re-downloading every cycle.

// refreshAniListSeasonScores walks every AniList part of every season of the
// title and stores the current score, episode count, and start date on each
// season_external_ids row (via ListPartsForTitle → UpdatePartMeta).
//
// Uses AniList's public GraphQL endpoint (no auth) — token-invalid handling
// is unnecessary on the call itself. The early-return on the
// anilist_token_invalid flag still applies: when the user's authenticated
// connection is broken (flagged by the push-sync path), we pause unrelated
// AniList traffic so the admin reconnect banner is the loudest signal until
// the user acts on it. Errors are logged per mapping; one bad season cannot
// break the others.

func mapTMDBSeriesStatus(details *matching.TMDBTVDetails) *model.SeriesStatus {
	var status model.SeriesStatus
	switch details.Status {
	case "Ended":
		status = model.SeriesStatusEnded
	case "Canceled":
		status = model.SeriesStatusCancelled
	case "Returning Series":
		status = model.SeriesStatusReturning
	case "In Production", "Planned", "Pilot":
		status = model.SeriesStatusInProduction
	default:
		return nil
	}
	return &status
}

// StartTicker launches the background refresh on a daily interval.
func (s *BackgroundService) StartTicker(ctx context.Context, interval time.Duration) {
	if s == nil {
		return
	}

	if s.shutdownWG != nil {
		s.shutdownWG.Add(1)
	}
	go func() {
		if s.shutdownWG != nil {
			defer s.shutdownWG.Done()
		}
		// Outer loop restarts the ticker after a panic so a single bad refresh
		// iteration cannot silently kill the daily schedule. Mirrors the
		// panic-recovery pattern used by TaskQueueWorker.Start.
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("background: panic in ticker loop: %v", r)
						time.Sleep(30 * time.Second)
					}
				}()

				select {
				case <-ctx.Done():
					return
				case <-time.After(30 * time.Second):
				}

				log.Println("background: fetching missing covers")
				if n := s.covers.FetchMissingCovers(ctx); n > 0 {
					log.Printf("background: fetched %d missing covers", n)
				}
				log.Println("background: starting initial refresh")
				s.RefreshTitles(ctx)

				ticker := time.NewTicker(interval)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						log.Println("background: starting scheduled refresh")
						s.RefreshTitles(ctx)

						day := time.Now().Weekday()
						log.Printf("background: starting unused covers cleanup for %s", day.String())
						s.covers.CleanupUnusedCovers(ctx, day)
					}
				}
			}()

			if ctx.Err() != nil {
				return
			}
		}
	}()
}

func (s *BackgroundService) enqueueRefreshOnRetryable(ctx context.Context, titleID int64, err error) {
	if !matching.IsRetryableError(err) {
		return
	}
	payload, marshalErr := json.Marshal(RefreshPayload{TitleID: titleID})
	if marshalErr != nil {
		log.Printf("enqueue refresh for title %d: marshal payload: %v", titleID, marshalErr)
		return
	}
	dedupKey := fmt.Sprintf("refresh:%d", titleID)
	if enqErr := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		_, e := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeRefresh, string(payload), &dedupKey)
		return e
	}); enqErr != nil {
		log.Printf("enqueue refresh for title %d: %v", titleID, enqErr)
	}
}
