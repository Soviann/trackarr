package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type PlexPayload struct {
	Event    string       `json:"event"`
	Metadata PlexMetadata `json:"Metadata"`
}

type PlexMetadata struct {
	Title                string     `json:"title"`
	GrandparentTitle     string     `json:"grandparentTitle"`
	Year                 int        `json:"year"`
	Type                 string     `json:"type"` // "movie", "episode"
	ParentIndex          int        `json:"parentIndex"`
	Index                int        `json:"index"`
	RatingKey            string     `json:"ratingKey"`
	GrandparentRatingKey string     `json:"grandparentRatingKey"`
	GUID                 []PlexGUID `json:"Guid"`
}

type PlexGUID struct {
	ID string `json:"id"` // "imdb://tt1234567", "tmdb://12345", "tvdb://12345"
}

type PlexExternalIDs struct {
	IMDB string
	TMDB int64
	TVDB int64
}

func ParseGUIDs(guids []PlexGUID) PlexExternalIDs {
	var ids PlexExternalIDs
	for _, g := range guids {
		if strings.HasPrefix(g.ID, "imdb://") {
			ids.IMDB = strings.TrimPrefix(g.ID, "imdb://")
		} else if strings.HasPrefix(g.ID, "tmdb://") {
			fmt.Sscanf(strings.TrimPrefix(g.ID, "tmdb://"), "%d", &ids.TMDB)
		} else if strings.HasPrefix(g.ID, "tvdb://") {
			fmt.Sscanf(strings.TrimPrefix(g.ID, "tvdb://"), "%d", &ids.TVDB)
		}
	}
	return ids
}

type PlexService struct {
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
}

func NewPlexService(titles *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository) *PlexService {
	return &PlexService{titles: titles, seasons: seasons, episodes: episodes, events: events}
}

func (s *PlexService) ProcessScrobble(payload *PlexPayload, rawPayload string) error {
	if payload.Event != "media.scrobble" {
		return nil
	}

	meta := payload.Metadata
	ids := ParseGUIDs(meta.GUID)

	switch meta.Type {
	case "movie":
		return s.processMovie(meta, ids, rawPayload)
	case "episode":
		return s.processEpisode(meta, ids, rawPayload)
	default:
		return fmt.Errorf("unknown media type: %s", meta.Type)
	}
}

func (s *PlexService) processMovie(meta PlexMetadata, ids PlexExternalIDs, rawPayload string) error {
	// Try to find existing title
	var imdbID *string
	var tmdbID *int64
	ratingKey := meta.RatingKey

	if ids.IMDB != "" {
		imdbID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		tmdbID = &ids.TMDB
	}

	title, err := s.titles.FindByExternalID(imdbID, tmdbID, &ratingKey)
	if err != nil {
		// Create new title
		newTitle := &model.Title{
			Type:          model.TitleTypeMovie,
			Year:          meta.Year,
			Status:        model.TitleStatusCompleted,
			MatchStatus:   model.MatchStatusConfirmed,
			PlexRatingKey: &ratingKey,
		}
		if ids.IMDB != "" {
			newTitle.IMDBID = &ids.IMDB
		}
		if ids.TMDB != 0 {
			newTitle.TMDBID = &ids.TMDB
		}
		if ids.TVDB != 0 {
			newTitle.TVDBID = &ids.TVDB
		}

		names := []model.TitleName{{Name: meta.Title, Language: "en", IsPrimary: true}}
		titleID, err := s.titles.Create(newTitle, names)
		if err != nil {
			return fmt.Errorf("create movie: %w", err)
		}

		s.events.Create(&model.WatchEvent{
			TitleID:     titleID,
			Source:      model.WatchEventSourcePlex,
			PlexPayload: &rawPayload,
		})
		return nil
	}

	// Log re-watch event
	s.events.Create(&model.WatchEvent{
		TitleID:     title.ID,
		Source:      model.WatchEventSourcePlex,
		PlexPayload: &rawPayload,
	})
	return nil
}

func (s *PlexService) processEpisode(meta PlexMetadata, ids PlexExternalIDs, rawPayload string) error {
	// Find title by grandparent rating key or external IDs
	grandparentKey := meta.GrandparentRatingKey
	var imdbID *string
	var tmdbID *int64

	if ids.IMDB != "" {
		imdbID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		tmdbID = &ids.TMDB
	}

	title, err := s.titles.FindByExternalID(imdbID, tmdbID, &grandparentKey)
	if err != nil {
		// Create new series
		seriesName := meta.GrandparentTitle
		if seriesName == "" {
			seriesName = meta.Title
		}

		newTitle := &model.Title{
			Type:          model.TitleTypeSeries,
			Year:          meta.Year,
			Status:        model.TitleStatusWatching,
			MatchStatus:   model.MatchStatusConfirmed,
			PlexRatingKey: &grandparentKey,
		}
		if ids.IMDB != "" {
			newTitle.IMDBID = &ids.IMDB
		}
		if ids.TMDB != 0 {
			newTitle.TMDBID = &ids.TMDB
		}
		if ids.TVDB != 0 {
			newTitle.TVDBID = &ids.TVDB
		}

		names := []model.TitleName{{Name: seriesName, Language: "en", IsPrimary: true}}
		titleID, createErr := s.titles.Create(newTitle, names)
		if createErr != nil {
			return fmt.Errorf("create series: %w", createErr)
		}
		title = &model.Title{ID: titleID}
	}

	// Get or create season and episode
	season, err := s.seasons.GetOrCreate(title.ID, meta.ParentIndex)
	if err != nil {
		return fmt.Errorf("get/create season: %w", err)
	}

	ep, err := s.episodes.GetOrCreate(season.ID, meta.Index)
	if err != nil {
		return fmt.Errorf("get/create episode: %w", err)
	}

	// Mark watched (if not already)
	if !ep.Watched {
		s.episodes.MarkWatched(ep.ID, time.Now().UTC())
	}

	// Log watch event
	s.events.Create(&model.WatchEvent{
		TitleID:     title.ID,
		EpisodeID:   &ep.ID,
		Source:      model.WatchEventSourcePlex,
		PlexPayload: &rawPayload,
	})

	return nil
}

func ParsePlexPayload(data []byte) (*PlexPayload, error) {
	var payload PlexPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse plex payload: %w", err)
	}
	return &payload, nil
}
