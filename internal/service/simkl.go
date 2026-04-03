package service

import (
	"fmt"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type SimklBackup struct {
	Movies []SimklItem `json:"movies"`
	Shows  []SimklItem `json:"shows"`
	Anime  []SimklItem `json:"anime"`
}

type SimklItem struct {
	Title         string       `json:"title"`
	Year          int          `json:"year"`
	Status        string       `json:"status"`
	UserRating    *int         `json:"user_rating"`
	LastWatchedAt string       `json:"last_watched_at"`
	AnimeType     string       `json:"anime_type"` // "tv", "movie", "ova", "ona", "special"
	IDs           SimklIDs     `json:"ids"`
	Seasons       []SimklSeason `json:"seasons"`
}

type SimklIDs struct {
	IMDB    string `json:"imdb"`
	TMDB    int64  `json:"tmdb"`
	AniList int64  `json:"anilist"`
	TVDB    int64  `json:"tvdb"`
}

type SimklSeason struct {
	Number   int           `json:"number"`
	Episodes []SimklEpisode `json:"episodes"`
}

type SimklEpisode struct {
	Number    int    `json:"number"`
	WatchedAt string `json:"watched_at"`
}

type ImportResult struct {
	Created int
	Skipped int
	Errors  int
}

type SimklImporter struct {
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
}

func NewSimklImporter(titles *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository) *SimklImporter {
	return &SimklImporter{titles: titles, seasons: seasons, episodes: episodes, events: events}
}

func (s *SimklImporter) Import(backup *SimklBackup, dryRun bool) (*ImportResult, error) {
	result := &ImportResult{}

	// Process movies
	for _, item := range backup.Movies {
		if err := s.importItem(item, model.TitleTypeMovie, result); err != nil {
			result.Errors++
		}
	}

	// Process shows
	for _, item := range backup.Shows {
		if err := s.importItem(item, model.TitleTypeSeries, result); err != nil {
			result.Errors++
		}
	}

	// Process anime
	for _, item := range backup.Anime {
		titleType := model.TitleTypeAnime
		if item.AnimeType == "movie" {
			titleType = model.TitleTypeMovie
		}
		if err := s.importItem(item, titleType, result); err != nil {
			result.Errors++
		}
	}

	return result, nil
}

func (s *SimklImporter) importItem(item SimklItem, titleType model.TitleType, result *ImportResult) error {
	// Check for duplicates by external ID
	var imdbID *string
	var tmdbID *int64
	if item.IDs.IMDB != "" {
		imdbID = &item.IDs.IMDB
	}
	if item.IDs.TMDB != 0 {
		tmdbID = &item.IDs.TMDB
	}

	if existing, err := s.titles.FindByExternalID(imdbID, tmdbID, nil); err == nil && existing != nil {
		result.Skipped++
		return nil
	}

	// Map status
	status := mapSimklStatus(item.Status)

	// Build title
	title := &model.Title{
		Type:        titleType,
		Year:        item.Year,
		Status:      status,
		MatchStatus: model.MatchStatusConfirmed,
		MyRating:    item.UserRating,
	}

	if item.IDs.IMDB != "" {
		title.IMDBID = &item.IDs.IMDB
	}
	if item.IDs.TMDB != 0 {
		title.TMDBID = &item.IDs.TMDB
	}
	if item.IDs.AniList != 0 {
		title.AniListID = &item.IDs.AniList
	}
	if item.IDs.TVDB != 0 {
		title.TVDBID = &item.IDs.TVDB
	}

	names := []model.TitleName{{Name: item.Title, Language: "en", IsPrimary: true}}

	titleID, err := s.titles.Create(title, names)
	if err != nil {
		return fmt.Errorf("create title %q: %w", item.Title, err)
	}

	// Import seasons/episodes
	for _, simklSeason := range item.Seasons {
		season, err := s.seasons.GetOrCreate(titleID, simklSeason.Number)
		if err != nil {
			continue
		}

		for _, simklEp := range simklSeason.Episodes {
			ep, err := s.episodes.GetOrCreate(season.ID, simklEp.Number)
			if err != nil {
				continue
			}

			watchedAt := time.Now().UTC()
			if simklEp.WatchedAt != "" {
				if t, err := time.Parse(time.RFC3339, simklEp.WatchedAt); err == nil {
					watchedAt = t
				}
			}

			s.episodes.MarkWatched(ep.ID, watchedAt)
			s.events.Create(&model.WatchEvent{
				TitleID:   titleID,
				EpisodeID: &ep.ID,
				Source:    model.WatchEventSourceManual,
			})
		}
	}

	// For movies, also log watch event with last_watched_at
	if titleType == model.TitleTypeMovie && item.LastWatchedAt != "" {
		s.events.Create(&model.WatchEvent{
			TitleID: titleID,
			Source:  model.WatchEventSourceManual,
		})
	}

	result.Created++
	return nil
}

func mapSimklStatus(status string) model.TitleStatus {
	switch status {
	case "completed":
		return model.TitleStatusCompleted
	case "watching":
		return model.TitleStatusWatching
	case "plantowatch":
		return model.TitleStatusPlanToWatch
	case "hold":
		return model.TitleStatusWatching
	case "notinteresting":
		return model.TitleStatusDropped
	default:
		return model.TitleStatusWatching
	}
}
