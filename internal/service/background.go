package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type BackgroundService struct {
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	tasks    *repository.TaskRepository
	settings *repository.SettingRepository
	tmdb     *matching.TMDBClient
	anilist  *matching.AniListClient
	push     PushNotifier
	dataDir  string
	limiter  *APILimiter
}

func NewBackgroundService(
	titles *repository.TitleRepository,
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
		titles:   titles,
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

func (s *BackgroundService) coversDir() string {
	return filepath.Join(s.dataDir, "covers")
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
}

// RefreshTitles processes all non-completed titles.
// Returns a result per processed title.
func (s *BackgroundService) RefreshTitles() []RefreshResult {
	return s.refreshTitles(false)
}

// RefreshAllTitles processes all titles regardless of status.
// Used to backfill metadata on existing titles.
func (s *BackgroundService) RefreshAllTitles() []RefreshResult {
	return s.refreshTitles(true)
}

func (s *BackgroundService) refreshTitles(includeAll bool) []RefreshResult {
	if s == nil {
		return nil
	}

	titles, err := s.titles.ListAll()
	if err != nil {
		log.Printf("background: list titles: %v", err)
		return nil
	}

	var results []RefreshResult

	for _, title := range titles {
		if !includeAll && (title.Status == model.TitleStatusCompleted || title.Status == model.TitleStatusDropped) {
			continue
		}

		result := s.refreshTitle(&title)
		results = append(results, result)

		_ = s.limiter.Wait(context.Background())
	}

	return results
}

func (s *BackgroundService) refreshTitle(title *model.Title) RefreshResult {
	result := RefreshResult{
		TitleID:   title.ID,
		TitleName: title.PrimaryName(),
	}

	// Step 1: Refresh from TMDB if available
	if s.tmdb != nil && title.TMDBID != nil {
		s.refreshFromTMDB(title, &result)
	}

	// Step 1b: AniList cover fallback for titles without TMDB ID
	if title.CoverURL == nil && title.TMDBID == nil && title.AniListID != nil {
		s.downloadAniListCover(title)
	}

	// Step 2: Auto-complete if series ended and all episodes watched
	// Need full title with seasons/episodes for this check
	if title.Type != model.TitleTypeMovie && title.SeriesStatus != nil {
		if *title.SeriesStatus == model.SeriesStatusEnded || *title.SeriesStatus == model.SeriesStatusCancelled {
			full, err := s.titles.GetByID(title.ID)
			if err == nil && s.allEpisodesWatched(full) {
				completed := model.TitleStatusCompleted
				if err := s.titles.Update(title.ID, repository.TitleUpdate{Status: &completed}); err == nil {
					result.AutoCompleted = true
					log.Printf("background: auto-completed %q", result.TitleName)
				}
			}
		}
	}

	return result
}

func (s *BackgroundService) refreshFromTMDB(title *model.Title, result *RefreshResult) {
	if title.Type == model.TitleTypeMovie {
		s.refreshMovieFromTMDB(title, result)
	} else {
		s.refreshSeriesFromTMDB(title, result)
	}
}

func (s *BackgroundService) refreshMovieFromTMDB(title *model.Title, result *RefreshResult) {
	details, err := s.tmdb.GetMovieDetails(*title.TMDBID)
	if err != nil {
		result.Error = err
		s.enqueueRefreshOnRetryable(title.ID, err)
		return
	}

	// Update cover if missing
	if title.CoverURL == nil && details.PosterPath != nil {
		coverPath, err := s.tmdb.DownloadCover(*details.PosterPath, s.coversDir())
		if err == nil {
			_ = s.titles.Update(title.ID, repository.TitleUpdate{CoverURL: &coverPath})
			title.CoverURL = &coverPath
		}
	}

	// Update metadata from TMDB details
	genres, credits, runtime, rating := matching.ExtractMovieMetadata(details)
	overview := details.Overview
	metaUpdate := repository.TitleUpdate{
		Overview: &overview,
		Genres:   &genres,
		Credits:  &credits,
	}
	if runtime != nil {
		metaUpdate.Runtime = runtime
	}
	if rating != nil {
		metaUpdate.TMDBRating = rating
	}
	_ = s.titles.Update(title.ID, metaUpdate)

	// Fallback: AniList cover
	if title.CoverURL == nil && title.AniListID != nil {
		s.downloadAniListCover(title)
	}
}

func (s *BackgroundService) refreshSeriesFromTMDB(title *model.Title, result *RefreshResult) {
	details, err := s.tmdb.GetTVDetails(*title.TMDBID)
	if err != nil {
		result.Error = err
		s.enqueueRefreshOnRetryable(title.ID, err)
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
		_ = s.titles.Update(title.ID, repository.TitleUpdate{SeriesStatus: newStatus})
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
			_ = s.titles.Update(title.ID, repository.TitleUpdate{CoverURL: &coverPath})
			title.CoverURL = &coverPath
		}
	}

	// Update metadata from TMDB details
	genres, credits, runtime, rating := matching.ExtractTVMetadata(details)
	overview := details.Overview
	metaUpdate := repository.TitleUpdate{
		Overview: &overview,
		Genres:   &genres,
		Credits:  &credits,
	}
	if runtime != nil {
		metaUpdate.Runtime = runtime
	}
	if rating != nil {
		metaUpdate.TMDBRating = rating
	}
	_ = s.titles.Update(title.ID, metaUpdate)

	// Fallback: AniList cover
	if title.CoverURL == nil && title.AniListID != nil {
		s.downloadAniListCover(title)
	}

	// Sync seasons and episodes
	for _, tmdbSeason := range details.Seasons {
		if tmdbSeason.SeasonNumber == 0 {
			continue // Skip specials
		}

		season, err := s.seasons.GetOrCreate(title.ID, tmdbSeason.SeasonNumber)
		if err != nil {
			continue
		}
		_ = s.seasons.UpdateTotalEpisodes(season.ID, tmdbSeason.EpisodeCount)

		// Fetch individual episodes
		episodes, err := s.tmdb.GetTVSeasonEpisodes(*title.TMDBID, tmdbSeason.SeasonNumber)
		if err != nil {
			continue
		}

		for _, tmdbEp := range episodes {
			ep, err := s.episodes.GetOrCreate(season.ID, tmdbEp.EpisodeNumber)
			if err != nil {
				continue
			}
			// Enrich metadata — only update if TMDB has non-empty values
			_ = s.episodes.UpdateMetadata(ep.ID, tmdbEp.Name, tmdbEp.AirDate)
		}

		_ = s.limiter.Wait(context.Background())
	}
}

func (s *BackgroundService) allEpisodesWatched(title *model.Title) bool {
	for _, season := range title.Seasons {
		episodes, err := s.episodes.GetBySeasonID(season.ID)
		if err != nil {
			return false
		}
		for _, ep := range episodes {
			if !ep.Watched {
				return false
			}
		}
		// No episodes = can't confirm all watched
		if len(episodes) == 0 {
			return false
		}
	}
	return len(title.Seasons) > 0
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
func (s *BackgroundService) FetchMissingCovers() int {
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
				details, err := s.tmdb.GetMovieDetails(*title.TMDBID)
				if err != nil {
					s.enqueueCoverOnRetryable(title.ID, *title.TMDBID, title.AniListID, title.Type, err)
				} else {
					posterPath = details.PosterPath
				}
			} else {
				details, err := s.tmdb.GetTVDetails(*title.TMDBID)
				if err != nil {
					s.enqueueCoverOnRetryable(title.ID, *title.TMDBID, title.AniListID, title.Type, err)
				} else {
					posterPath = details.PosterPath
				}
			}

			if posterPath != nil && *posterPath != "" {
				coverPath, err := s.tmdb.DownloadCover(*posterPath, s.coversDir())
				if err == nil {
					_ = s.titles.Update(title.ID, repository.TitleUpdate{CoverURL: &coverPath})
					fetched++
					_ = s.limiter.Wait(context.Background())
					continue
				}
			}
		}

		// Fallback: AniList
		if title.AniListID != nil && s.downloadAniListCover(&title) {
			fetched++
		}

		_ = s.limiter.Wait(context.Background())
	}

	return fetched
}

// downloadAniListCover fetches and saves the cover from AniList for a title.
// Returns true if the cover was successfully downloaded and saved.
func (s *BackgroundService) downloadAniListCover(title *model.Title) bool {
	if s.anilist == nil || title.AniListID == nil {
		return false
	}

	details, err := s.anilist.GetAnimeDetails(*title.AniListID)
	if err != nil || details.CoverURL == "" {
		return false
	}

	coverPath, err := s.anilist.DownloadCover(details.CoverURL, s.coversDir())
	if err != nil {
		return false
	}

	_ = s.titles.Update(title.ID, repository.TitleUpdate{CoverURL: &coverPath})
	return true
}

// StartTicker launches the background refresh on a daily interval.
func (s *BackgroundService) StartTicker(interval time.Duration) {
	if s == nil {
		return
	}

	go func() {
		// Run once at startup after a short delay
		time.Sleep(30 * time.Second)
		log.Println("background: fetching missing covers")
		if n := s.FetchMissingCovers(); n > 0 {
			log.Printf("background: fetched %d missing covers", n)
		}
		log.Println("background: starting initial refresh")
		s.RefreshTitles()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("background: starting scheduled refresh")
			s.RefreshTitles()
			
			day := time.Now().Weekday()
			log.Printf("background: starting unused covers cleanup for %s", day.String())
			s.CleanupUnusedCovers(day)
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
func (s *BackgroundService) CleanupUnusedCovers(day time.Weekday) {
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
			_ = s.limiter.Wait(context.Background())
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

func (s *BackgroundService) enqueueRefreshOnRetryable(titleID int64, err error) {
	if s.tasks == nil || !matching.IsRetryableError(err) {
		return
	}
	payload, _ := json.Marshal(RefreshPayload{TitleID: titleID})
	dedupKey := fmt.Sprintf("refresh:%d", titleID)
	if _, enqErr := s.tasks.Enqueue(model.TaskTypeRefresh, string(payload), &dedupKey); enqErr != nil {
		log.Printf("enqueue refresh for title %d: %v", titleID, enqErr)
	}
}

func (s *BackgroundService) enqueueCoverOnRetryable(titleID, tmdbID int64, anilistID *int64, titleType model.TitleType, err error) {
	if s.tasks == nil || !matching.IsRetryableError(err) {
		return
	}
	p := CoverFetchPayload{TitleID: titleID, TMDBID: tmdbID, TitleType: titleType}
	if anilistID != nil {
		p.AniListID = *anilistID
	}
	payload, _ := json.Marshal(p)
	dedupKey := fmt.Sprintf("cover_fetch:%d", titleID)
	if _, enqErr := s.tasks.Enqueue(model.TaskTypeCoverFetch, string(payload), &dedupKey); enqErr != nil {
		log.Printf("enqueue cover fetch for title %d: %v", titleID, enqErr)
	}
}
