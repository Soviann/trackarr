package service

import (
	"fmt"
	"log"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type BackgroundService struct {
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	tmdb     *matching.TMDBClient
	push     *PushService
}

func NewBackgroundService(
	titles *repository.TitleRepository,
	seasons *repository.SeasonRepository,
	episodes *repository.EpisodeRepository,
	tmdb *matching.TMDBClient,
	push *PushService,
) *BackgroundService {
	return &BackgroundService{
		titles:   titles,
		seasons:  seasons,
		episodes: episodes,
		tmdb:     tmdb,
		push:     push,
	}
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
	if s == nil {
		return nil
	}

	// List non-completed, confirmed titles
	titles, err := s.titles.List(repository.TitleFilter{})
	if err != nil {
		log.Printf("background: list titles: %v", err)
		return nil
	}

	var results []RefreshResult

	for _, title := range titles {
		if title.Status == model.TitleStatusCompleted || title.Status == model.TitleStatusDropped {
			continue
		}

		result := s.refreshTitle(&title)
		results = append(results, result)

		// Rate limiting between titles
		time.Sleep(500 * time.Millisecond)
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
		return
	}

	// Update cover if missing
	if title.CoverURL == nil && details.PosterPath != nil {
		coverPath, err := s.tmdb.DownloadCover(*details.PosterPath, "")
		if err == nil {
			s.titles.Update(title.ID, repository.TitleUpdate{CoverURL: &coverPath})
		}
	}
}

func (s *BackgroundService) refreshSeriesFromTMDB(title *model.Title, result *RefreshResult) {
	details, err := s.tmdb.GetTVDetails(*title.TMDBID)
	if err != nil {
		result.Error = err
		return
	}

	// Detect series status change
	newStatus := mapTMDBSeriesStatus(details)
	if newStatus != nil && title.SeriesStatus != nil && *newStatus != *title.SeriesStatus {
		result.StatusChanged = true
		result.OldStatus = *title.SeriesStatus
		result.NewStatus = *newStatus
		s.titles.Update(title.ID, repository.TitleUpdate{SeriesStatus: newStatus})
		title.SeriesStatus = newStatus

		if *newStatus == model.SeriesStatusEnded || *newStatus == model.SeriesStatusCancelled {
			s.push.SendNotification(
				title.PrimaryName(),
				"La série est terminée",
				fmt.Sprintf("/title/%d", title.ID),
			)
		}
	}

	// Update cover if missing
	if title.CoverURL == nil && details.PosterPath != nil {
		coverPath, err := s.tmdb.DownloadCover(*details.PosterPath, "")
		if err == nil {
			s.titles.Update(title.ID, repository.TitleUpdate{CoverURL: &coverPath})
		}
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
		s.seasons.UpdateTotalEpisodes(season.ID, tmdbSeason.EpisodeCount)

		// Fetch individual episodes
		episodes, err := s.tmdb.GetTVSeasonEpisodes(*title.TMDBID, tmdbSeason.SeasonNumber)
		if err != nil {
			continue
		}

		for _, tmdbEp := range episodes {
			s.episodes.GetOrCreate(season.ID, tmdbEp.EpisodeNumber)
		}

		time.Sleep(250 * time.Millisecond) // Rate limiting per season
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
	// TMDB doesn't expose status directly in our current model,
	// but we can infer from season data. For now return nil.
	// This will be enhanced when TMDB status field is added.
	return nil
}

// StartTicker launches the background refresh on a daily interval.
func (s *BackgroundService) StartTicker(interval time.Duration) {
	if s == nil {
		return
	}

	go func() {
		// Run once at startup after a short delay
		time.Sleep(30 * time.Second)
		log.Println("background: starting initial refresh")
		s.RefreshTitles()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("background: starting scheduled refresh")
			s.RefreshTitles()
		}
	}()
}
