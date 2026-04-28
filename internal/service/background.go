package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
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
		if !includeAll && (title.Status == model.TitleStatusCompleted || title.Status == model.TitleStatusDropped) {
			continue
		}

		result := s.refreshTitle(ctx, title)
		results = append(results, result)

		_ = s.limiter.Wait(ctx)
	}

	return results
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

	// Step 1b: AniList cover fallback for titles without TMDB ID
	if title.CoverURL == nil && title.TMDBID == nil && title.AniListID != nil {
		s.covers.DownloadAniListCover(ctx, title)
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

func (s *BackgroundService) refreshFromTMDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	if title.Type == model.TitleTypeMovie {
		s.refreshMovieFromTMDB(ctx, title, result)
	} else {
		s.refreshSeriesFromTMDB(ctx, title, result)
	}
}

// logTitleUpdate logs an error from a title update if non-nil.
func logTitleUpdate(titleID int64, kind string, err error) {
	if err != nil {
		log.Printf("background: update %s for title %d: %v", kind, titleID, err)
	}
}

// refreshFromTVDB fetches TVDB data for titles that have a TVDB ID.
// TVDB ID cross-referencing from TMDB is handled in refreshMovieFromTMDB / refreshSeriesFromTMDB.
// For titles with a TMDB ID, overview and genres are refreshed from TMDB; here only the cover is updated.
// For titles without a TMDB ID, overview and genres are also persisted from TVDB.
func (s *BackgroundService) refreshFromTVDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	if title.TVDBID == nil {
		return
	}
	tvdbID := *title.TVDBID

	update := repository.TitleUpdate{}
	if title.Type == model.TitleTypeMovie {
		details, err := s.tvdb.GetMovieDetails(ctx, tvdbID)
		if err != nil {
			log.Printf("background tvdb movie refresh %d: %v", title.ID, err)
			return
		}
		result.Refreshed = true
		if title.CoverURL == nil && details.Image != "" {
			if filename, err := s.tvdb.DownloadCover(ctx, details.Image, tvdbID, s.covers.Dir()); err == nil {
				update.CoverURL = &filename
			}
		}
		if title.TMDBID == nil {
			if details.Overview != "" {
				ov := details.Overview
				update.Overview = &ov
			}
			var genreList []string
			for _, g := range details.Genres {
				if g.Name != "" {
					genreList = append(genreList, g.Name)
				}
			}
			if len(genreList) > 0 {
				if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
					return repository.NewGenreWriter(tx).ReplaceForTitle(ctx, title.ID, genreList)
				}); err != nil {
					log.Printf("background: save tvdb genres for title %d: %v", title.ID, err)
				}
			}
		}
	} else {
		details, err := s.tvdb.GetSeriesDetails(ctx, tvdbID)
		if err != nil {
			log.Printf("background tvdb series refresh %d: %v", title.ID, err)
			return
		}
		result.Refreshed = true
		if title.CoverURL == nil && details.Image != "" {
			if filename, err := s.tvdb.DownloadCover(ctx, details.Image, tvdbID, s.covers.Dir()); err == nil {
				update.CoverURL = &filename
			}
		}
		if title.TMDBID == nil {
			if details.Overview != "" {
				ov := details.Overview
				update.Overview = &ov
			}
			var genreList []string
			for _, g := range details.Genres {
				if g.Name != "" {
					genreList = append(genreList, g.Name)
				}
			}
			if len(genreList) > 0 {
				if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
					return repository.NewGenreWriter(tx).ReplaceForTitle(ctx, title.ID, genreList)
				}); err != nil {
					log.Printf("background: save tvdb genres for title %d: %v", title.ID, err)
				}
			}
		}
	}
	if update.CoverURL != nil || update.Overview != nil {
		logTitleUpdate(title.ID, "tvdb refresh", s.updateTitle(ctx, title.ID, update))
		if update.CoverURL != nil {
			s.covers.ExtractAndStoreAccent(ctx, title.ID, *update.CoverURL)
		}
	}
}

func (s *BackgroundService) refreshMovieFromTMDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	details, err := s.tmdb.GetMovieDetails(ctx, *title.TMDBID)
	if err != nil {
		result.Error = err
		s.enqueueRefreshOnRetryable(ctx, title.ID, err)
		return
	}
	result.Refreshed = true

	// Update cover if missing
	if title.CoverURL == nil && details.PosterPath != nil {
		coverPath, err := s.tmdb.DownloadCover(ctx, *details.PosterPath, s.covers.Dir())
		if err == nil {
			logTitleUpdate(title.ID, "movie cover", s.updateTitle(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath}))
			s.covers.ExtractAndStoreAccent(ctx, title.ID, coverPath)
			title.CoverURL = &coverPath
		}
	}

	// Update metadata from TMDB details
	genres, credits, runtime, rating := matching.ExtractMovieMetadata(details)
	overview := details.Overview
	metaUpdate := repository.TitleUpdate{
		Overview: &overview,
		Credits:  &credits,
	}
	if runtime != nil {
		metaUpdate.Runtime = runtime
	}
	if rating != nil {
		metaUpdate.TMDBRating = rating
	}
	logTitleUpdate(title.ID, "movie metadata", s.updateTitle(ctx, title.ID, metaUpdate))

	// Persist genres to title_genres table
	if genres != "" {
		var genreList []string
		if err := json.Unmarshal([]byte(genres), &genreList); err == nil && len(genreList) > 0 {
			if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
				return repository.NewGenreWriter(tx).ReplaceForTitle(ctx, title.ID, genreList)
			}); err != nil {
				log.Printf("background: save genres for title %d: %v", title.ID, err)
			}
		}
	}

	// Fallback: AniList cover
	if title.CoverURL == nil && title.AniListID != nil {
		s.covers.DownloadAniListCover(ctx, title)
	}

	// Cross-reference TVDB ID if not yet stored (avoids a duplicate TMDB fetch in refreshFromTVDB)
	if title.TVDBID == nil && details.ExternalIDs != nil && details.ExternalIDs.TVDBID != 0 {
		tvdbID := details.ExternalIDs.TVDBID
		logTitleUpdate(title.ID, "movie tvdb backfill", s.updateTitle(ctx, title.ID, repository.TitleUpdate{TVDBID: &tvdbID}))
		title.TVDBID = &tvdbID
	}
}

func (s *BackgroundService) refreshSeriesFromTMDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	details, err := s.tmdb.GetTVDetails(ctx, *title.TMDBID)
	if err != nil {
		result.Error = err
		s.enqueueRefreshOnRetryable(ctx, title.ID, err)
		return
	}
	result.Refreshed = true

	// Detect series status change
	newStatus := mapTMDBSeriesStatus(details)
	if newStatus != nil && (title.SeriesStatus == nil || *newStatus != *title.SeriesStatus) {
		result.StatusChanged = true
		if title.SeriesStatus != nil {
			result.OldStatus = *title.SeriesStatus
		}
		result.NewStatus = *newStatus
		logTitleUpdate(title.ID, "series status", s.updateTitle(ctx, title.ID, repository.TitleUpdate{SeriesStatus: newStatus}))
		title.SeriesStatus = newStatus

		if (*newStatus == model.SeriesStatusEnded || *newStatus == model.SeriesStatusCancelled) && IsNotificationEnabled(s.settings, NotifSeriesEnded) {
			if err := s.push.SendNotification(
				ctx,
				"PlexTracker",
				fmt.Sprintf("%s — Series ended", title.PrimaryName),
				fmt.Sprintf("/title/%d", title.ID),
			); err != nil {
				log.Printf("series-ended push failed for title %d: %v", title.ID, err)
			}
		}
	}

	// Update cover if missing
	if title.CoverURL == nil && details.PosterPath != nil {
		coverPath, err := s.tmdb.DownloadCover(ctx, *details.PosterPath, s.covers.Dir())
		if err == nil {
			logTitleUpdate(title.ID, "series cover", s.updateTitle(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath}))
			s.covers.ExtractAndStoreAccent(ctx, title.ID, coverPath)
			title.CoverURL = &coverPath
		}
	}

	// Update metadata from TMDB details
	genres, credits, runtime, rating := matching.ExtractTVMetadata(details)
	overview := details.Overview
	metaUpdate := repository.TitleUpdate{
		Overview: &overview,
		Credits:  &credits,
	}
	if runtime != nil {
		metaUpdate.Runtime = runtime
	}
	if rating != nil {
		metaUpdate.TMDBRating = rating
	}
	// Populate next_air_date and next_air_episode from TMDB next_episode_to_air
	if details.NextEpisodeToAir != nil && details.NextEpisodeToAir.AirDate != "" {
		airDate := details.NextEpisodeToAir.AirDate
		airEp := fmt.Sprintf("S%d E%d", details.NextEpisodeToAir.SeasonNumber, details.NextEpisodeToAir.EpisodeNumber)
		metaUpdate.NextAirDate = &airDate
		metaUpdate.NextAirEpisode = &airEp
	}
	logTitleUpdate(title.ID, "series metadata", s.updateTitle(ctx, title.ID, metaUpdate))

	// Persist genres to title_genres table
	if genres != "" {
		var genreList []string
		if err := json.Unmarshal([]byte(genres), &genreList); err == nil && len(genreList) > 0 {
			if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
				return repository.NewGenreWriter(tx).ReplaceForTitle(ctx, title.ID, genreList)
			}); err != nil {
				log.Printf("background: save genres for title %d: %v", title.ID, err)
			}
		}
	}

	// Fallback: AniList cover
	if title.CoverURL == nil && title.AniListID != nil {
		s.covers.DownloadAniListCover(ctx, title)
	}

	// Sync seasons and episodes — season + its episodes share one transaction
	// so a crash between season upsert and episode upsert cannot leave
	// total_episodes out of sync with the actual episode rows.
	for _, tmdbSeason := range details.Seasons {
		if err := ctx.Err(); err != nil {
			return
		}

		if tmdbSeason.SeasonNumber == 0 {
			continue // Skip specials
		}

		// Fetch individual episodes outside the write transaction to keep
		// TMDB HTTP latency off the sole write connection.
		tmdbEpisodes, err := s.tmdb.GetTVSeasonEpisodes(ctx, *title.TMDBID, tmdbSeason.SeasonNumber)
		if err != nil {
			continue
		}

		entries := make([]repository.EpisodeUpsert, len(tmdbEpisodes))
		for i, ep := range tmdbEpisodes {
			entries[i] = repository.EpisodeUpsert{
				EpisodeNumber: ep.EpisodeNumber,
				Name:          ep.Name,
				AirDate:       ep.AirDate,
			}
		}

		_ = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
			season, err := repository.NewSeasonWriter(tx).Upsert(ctx, title.ID, tmdbSeason.SeasonNumber, tmdbSeason.EpisodeCount)
			if err != nil {
				return err
			}
			return repository.NewEpisodeWriter(tx).UpsertBatch(ctx, season.ID, entries)
		})

		_ = s.limiter.Wait(ctx)
	}

	// Cross-reference TVDB ID if not yet stored (avoids a duplicate TMDB fetch in refreshFromTVDB)
	if title.TVDBID == nil && details.ExternalIDs != nil && details.ExternalIDs.TVDBID != 0 {
		tvdbID := details.ExternalIDs.TVDBID
		logTitleUpdate(title.ID, "series tvdb backfill", s.updateTitle(ctx, title.ID, repository.TitleUpdate{TVDBID: &tvdbID}))
		title.TVDBID = &tvdbID
	}
}

// refreshAniListSeasonScores walks every season of the title that has an
// AniList mapping and stores the current averageScore on
// seasons.anilist_average_score.
//
// Uses AniList's public GraphQL endpoint (no auth) — token-invalid handling
// is unnecessary on the call itself. The early-return on the
// anilist_token_invalid flag still applies: when the user's authenticated
// connection is broken (flagged by the push-sync path), we pause unrelated
// AniList traffic so the admin reconnect banner is the loudest signal until
// the user acts on it. Errors are logged per mapping; one bad season cannot
// break the others.
func (s *BackgroundService) refreshAniListSeasonScores(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	if invalid, _ := s.settings.Get(settingAniListTokenInvalid); invalid == "true" {
		return
	}

	mappings, err := s.seasonExtIDs.ListForTitle(ctx, title.ID, providerAniList)
	if err != nil {
		log.Printf("background anilist score: list mappings for title %d: %v", title.ID, err)
		return
	}
	if len(mappings) == 0 {
		return
	}

	for seasonID, externalID := range mappings {
		if err := ctx.Err(); err != nil {
			return
		}

		anilistID, err := strconv.ParseInt(externalID, 10, 64)
		if err != nil {
			log.Printf("background anilist score: invalid mapping %q for season %d: %v", externalID, seasonID, err)
			continue
		}

		details, err := s.anilist.GetAnimeDetails(ctx, anilistID)
		if err != nil {
			log.Printf("background anilist score: fetch %d: %v", anilistID, err)
			_ = s.limiter.Wait(ctx)
			continue
		}
		result.Refreshed = true

		if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
			return repository.NewSeasonWriter(tx).UpdateAniListAverageScore(ctx, seasonID, details.AverageScore)
		}); err != nil {
			log.Printf("background anilist score: persist season %d: %v", seasonID, err)
		}

		_ = s.limiter.Wait(ctx)
	}
}

func mapTMDBSeriesStatus(details *matching.TMDBTVDetails) *model.SeriesStatus {
	var status model.SeriesStatus
	switch details.Status {
	case "Ended":
		status = model.SeriesStatusEnded
	case "Canceled":
		status = model.SeriesStatusCancelled
	case "Returning Series":
		status = model.SeriesStatusReturning
	case "In Production":
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
