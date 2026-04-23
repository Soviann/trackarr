package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type BackgroundService struct {
	writeDB  *sql.DB
	titles   *repository.TitleRepository
	genres   *repository.GenreRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	tvdb     *matching.TVDBClient // optional — nil if TVDB_API_KEY not set
	tasks    *repository.TaskRepository
	settings *repository.SettingRepository
	tmdb     *matching.TMDBClient
	anilist  *matching.AniListClient
	push     PushNotifier
	dataDir  string
	limiter  *APILimiter
}

func NewBackgroundService(
	writeDB *sql.DB,
	titles *repository.TitleRepository,
	genres *repository.GenreRepository,
	seasons *repository.SeasonRepository,
	episodes *repository.EpisodeRepository,
	tasks *repository.TaskRepository,
	settings *repository.SettingRepository,
	tmdb *matching.TMDBClient,
	anilist *matching.AniListClient,
	push PushNotifier,
	dataDir string,
) *BackgroundService {
	return &BackgroundService{
		writeDB:  writeDB,
		titles:   titles,
		genres:   genres,
		seasons:  seasons,
		episodes: episodes,
		tasks:    tasks,
		settings: settings,
		tmdb:     tmdb,
		anilist:  anilist,
		push:     push,
		dataDir:  dataDir,
		limiter:  NewAPILimiter(2, 1),
	}
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

func (s *BackgroundService) coversDir() string {
	return filepath.Join(s.dataDir, "covers")
}

// SetTVDB injects the TVDB client (optional — called after construction if TVDB is configured).
func (s *BackgroundService) SetTVDB(tvdb *matching.TVDBClient) { s.tvdb = tvdb }

// RefreshResult captures what happened for a single title during refresh.
type RefreshResult struct {
	TitleID       int64
	TitleName     string
	AutoCompleted bool
	StatusChanged bool
	OldStatus     model.SeriesStatus
	NewStatus     model.SeriesStatus
	Error         error
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
	title, err := s.titles.GetByID(titleID)
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

	titles, err := s.titles.ListAll()
	if err != nil {
		log.Printf("background: list titles: %v", err)
		return nil
	}

	results := make([]RefreshResult, 0, len(titles))

	for _, title := range titles {
		if !includeAll && (title.Status == model.TitleStatusCompleted || title.Status == model.TitleStatusDropped) {
			continue
		}

		result := s.refreshTitle(ctx, &title)
		results = append(results, result)

		_ = s.limiter.Wait(ctx)
	}

	return results
}

func (s *BackgroundService) refreshTitle(ctx context.Context, title *model.Title) RefreshResult {
	result := RefreshResult{
		TitleID:   title.ID,
		TitleName: title.PrimaryName(),
	}

	// Step 1: Refresh from TMDB if available
	if s.tmdb != nil && title.TMDBID != nil {
		s.refreshFromTMDB(ctx, title, &result)
	}

	// Step 1b: AniList cover fallback for titles without TMDB ID
	if title.CoverURL == nil && title.TMDBID == nil && title.AniListID != nil {
		s.downloadAniListCover(ctx, title)
	}

	// Step 1c: TVDB enrichment — fetch rating, cover fallback, and tvdb_id cross-ref
	if s.tvdb != nil {
		s.refreshFromTVDB(ctx, title)
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

	return result
}

func (s *BackgroundService) refreshFromTMDB(ctx context.Context, title *model.Title, result *RefreshResult) {
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
func (s *BackgroundService) refreshFromTVDB(ctx context.Context, title *model.Title) {
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
		if title.CoverURL == nil && details.Image != "" {
			if filename, err := s.tvdb.DownloadCover(details.Image, tvdbID, s.coversDir()); err == nil {
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
			if len(genreList) > 0 && s.genres != nil {
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
		if title.CoverURL == nil && details.Image != "" {
			if filename, err := s.tvdb.DownloadCover(details.Image, tvdbID, s.coversDir()); err == nil {
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
			if len(genreList) > 0 && s.genres != nil {
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
	}
}

func (s *BackgroundService) refreshMovieFromTMDB(ctx context.Context, title *model.Title, result *RefreshResult) {
	details, err := s.tmdb.GetMovieDetails(ctx, *title.TMDBID)
	if err != nil {
		result.Error = err
		s.enqueueRefreshOnRetryable(ctx, title.ID, err)
		return
	}

	// Update cover if missing
	if title.CoverURL == nil && details.PosterPath != nil {
		coverPath, err := s.tmdb.DownloadCover(*details.PosterPath, s.coversDir())
		if err == nil {
			logTitleUpdate(title.ID, "movie cover", s.updateTitle(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath}))
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
	if genres != "" && s.genres != nil {
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
		s.downloadAniListCover(ctx, title)
	}

	// Cross-reference TVDB ID if not yet stored (avoids a duplicate TMDB fetch in refreshFromTVDB)
	if title.TVDBID == nil && details.ExternalIDs != nil && details.ExternalIDs.TVDBID != 0 {
		tvdbID := details.ExternalIDs.TVDBID
		logTitleUpdate(title.ID, "movie tvdb backfill", s.updateTitle(ctx, title.ID, repository.TitleUpdate{TVDBID: &tvdbID}))
		title.TVDBID = &tvdbID
	}
}

func (s *BackgroundService) refreshSeriesFromTMDB(ctx context.Context, title *model.Title, result *RefreshResult) {
	details, err := s.tmdb.GetTVDetails(ctx, *title.TMDBID)
	if err != nil {
		result.Error = err
		s.enqueueRefreshOnRetryable(ctx, title.ID, err)
		return
	}

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
			_ = s.push.SendNotification(
				"PlexTracker",
				fmt.Sprintf("%s — Series ended", title.PrimaryName()),
				fmt.Sprintf("/title/%d", title.ID),
			)
		}
	}

	// Update cover if missing
	if title.CoverURL == nil && details.PosterPath != nil {
		coverPath, err := s.tmdb.DownloadCover(*details.PosterPath, s.coversDir())
		if err == nil {
			logTitleUpdate(title.ID, "series cover", s.updateTitle(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath}))
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
	if genres != "" && s.genres != nil {
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
		s.downloadAniListCover(ctx, title)
	}

	// Sync seasons and episodes — season + its episodes share one transaction
	// so a crash between season upsert and episode upsert cannot leave
	// total_episodes out of sync with the actual episode rows.
	for _, tmdbSeason := range details.Seasons {
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

// FetchMissingCovers downloads covers for all titles without a cover.
// Tries TMDB first (if TMDB ID available), then falls back to AniList.
func (s *BackgroundService) FetchMissingCovers(ctx context.Context) int {
	if s == nil {
		return 0
	}

	titles, err := s.titles.ListAll()
	if err != nil {
		log.Printf("background: list titles for covers: %v", err)
		return 0
	}

	fetched := 0
	for _, title := range titles {
		if title.CoverURL != nil {
			continue
		}

		// Try TMDB
		if title.TMDBID != nil {
			var posterPath *string
			if title.Type == model.TitleTypeMovie {
				details, err := s.tmdb.GetMovieDetails(ctx, *title.TMDBID)
				if err != nil {
					s.enqueueCoverOnRetryable(ctx, title.ID, *title.TMDBID, title.AniListID, title.Type, err)
				} else {
					posterPath = details.PosterPath
				}
			} else {
				details, err := s.tmdb.GetTVDetails(ctx, *title.TMDBID)
				if err != nil {
					s.enqueueCoverOnRetryable(ctx, title.ID, *title.TMDBID, title.AniListID, title.Type, err)
				} else {
					posterPath = details.PosterPath
				}
			}

			if posterPath != nil && *posterPath != "" {
				coverPath, err := s.tmdb.DownloadCover(*posterPath, s.coversDir())
				if err == nil {
					logTitleUpdate(title.ID, "missing cover", s.updateTitle(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath}))
					fetched++
					_ = s.limiter.Wait(ctx)
					continue
				}
			}
		}

		// Fallback: AniList
		if title.AniListID != nil && s.downloadAniListCover(ctx, &title) {
			fetched++
		}

		_ = s.limiter.Wait(ctx)
	}

	return fetched
}

// downloadAniListCover fetches and saves the cover from AniList for a title.
// Returns true if the cover was successfully downloaded and saved.
func (s *BackgroundService) downloadAniListCover(ctx context.Context, title *model.Title) bool {
	if s.anilist == nil || title.AniListID == nil {
		return false
	}

	details, err := s.anilist.GetAnimeDetails(ctx, *title.AniListID)
	if err != nil || details.CoverURL == "" {
		return false
	}

	coverPath, err := s.anilist.DownloadCover(details.CoverURL, s.coversDir())
	if err != nil {
		return false
	}

	logTitleUpdate(title.ID, "anilist cover", s.updateTitle(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath}))
	return true
}

// StartTicker launches the background refresh on a daily interval.
func (s *BackgroundService) StartTicker(ctx context.Context, interval time.Duration) {
	if s == nil {
		return
	}

	go func() {
		// Run once at startup after a short delay
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}

		log.Println("background: fetching missing covers")
		if n := s.FetchMissingCovers(ctx); n > 0 {
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
				s.CleanupUnusedCovers(ctx, day)
			}
		}
	}()
}

func getDailyPrefixes(day time.Weekday) []rune {
	switch day {
	case time.Sunday:
		return []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i'}
	case time.Monday:
		return []rune{'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r'}
	case time.Tuesday:
		return []rune{'s', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A'}
	case time.Wednesday:
		return []rune{'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J'}
	case time.Thursday:
		return []rune{'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S'}
	case time.Friday:
		return []rune{'T', 'U', 'V', 'W', 'X', 'Y', 'Z', '0', '1'}
	case time.Saturday:
		return []rune{'2', '3', '4', '5', '6', '7', '8', '9', '_', '-'}
	default:
		return nil
	}
}

// CleanupUnusedCovers deletes orphaned cover files sharded by the starting character of the filename.
func (s *BackgroundService) CleanupUnusedCovers(ctx context.Context, day time.Weekday) {
	if s == nil || s.titles == nil {
		return
	}

	prefixes := getDailyPrefixes(day)
	if len(prefixes) == 0 {
		return
	}

	prefixMap := make(map[rune]bool)
	for _, p := range prefixes {
		prefixMap[p] = true
	}

	coversDir := s.coversDir()
	entries, err := os.ReadDir(coversDir)
	if err != nil {
		log.Printf("background: read covers dir: %v", err)
		return
	}

	var batch []string
	deleted := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if len(name) == 0 {
			continue
		}

		firstChar := rune(name[0])
		if !prefixMap[firstChar] {
			continue
		}

		batch = append(batch, name)

		if len(batch) >= 100 {
			deleted += s.processCoverBatch(coversDir, batch)
			batch = batch[:0]
			_ = s.limiter.Wait(ctx)
		}
	}

	if len(batch) > 0 {
		deleted += s.processCoverBatch(coversDir, batch)
	}

	if deleted > 0 {
		log.Printf("background: deleted %d unused covers for %s", deleted, day.String())
	}
}

func (s *BackgroundService) processCoverBatch(coversDir string, batch []string) int {
	used, err := s.titles.GetUsedCoversInBatch(batch)
	if err != nil {
		log.Printf("background: get used covers batch: %v", err)
		return 0
	}

	deleted := 0
	for _, name := range batch {
		if !used[name] {
			path := filepath.Join(coversDir, name)
			if err := os.Remove(path); err != nil {
				log.Printf("background: delete unused cover %s: %v", name, err)
			} else {
				deleted++
			}
		}
	}
	return deleted
}

func (s *BackgroundService) enqueueRefreshOnRetryable(ctx context.Context, titleID int64, err error) {
	if s.tasks == nil || !matching.IsRetryableError(err) {
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

func (s *BackgroundService) enqueueCoverOnRetryable(ctx context.Context, titleID, tmdbID int64, anilistID *int64, titleType model.TitleType, err error) {
	if s.tasks == nil || !matching.IsRetryableError(err) {
		return
	}
	p := CoverFetchPayload{TitleID: titleID, TMDBID: tmdbID, TitleType: titleType}
	if anilistID != nil {
		p.AniListID = *anilistID
	}
	payload, marshalErr := json.Marshal(p)
	if marshalErr != nil {
		log.Printf("enqueue cover fetch for title %d: marshal payload: %v", titleID, marshalErr)
		return
	}
	dedupKey := fmt.Sprintf("cover_fetch:%d", titleID)
	if enqErr := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		_, e := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeCoverFetch, string(payload), &dedupKey)
		return e
	}); enqErr != nil {
		log.Printf("enqueue cover fetch for title %d: %v", titleID, enqErr)
	}
}
