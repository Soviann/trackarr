package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
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
	pipeline *matching.Pipeline // nil = skip matching, create with basic info
	push     *PushService       // nil = no push notifications
}

func NewPlexService(titles *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, pipeline *matching.Pipeline, push *PushService) *PlexService {
	return &PlexService{titles: titles, seasons: seasons, episodes: episodes, events: events, pipeline: pipeline, push: push}
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
		// Create new title — run matching pipeline if available
		newTitle, names := s.buildNewTitle(meta, ids, model.TitleTypeMovie, ratingKey)
		newTitle.Status = model.TitleStatusCompleted

		titleID, err := s.titles.Create(newTitle, names)
		if err != nil {
			return fmt.Errorf("create movie: %w", err)
		}

		s.events.Create(&model.WatchEvent{
			TitleID:     titleID,
			Source:      model.WatchEventSourcePlex,
			PlexPayload: &rawPayload,
		})

		s.push.SendNotification(
			fmt.Sprintf("Note %s ?", meta.Title),
			"Tu viens de regarder ce film",
			fmt.Sprintf("/title/%d", titleID),
		)
		return nil
	}

	s.events.Create(&model.WatchEvent{
		TitleID:     title.ID,
		Source:      model.WatchEventSourcePlex,
		PlexPayload: &rawPayload,
	})

	// Prompt rating for movies on re-scrobble too
	if title.MyRating == nil {
		s.push.SendNotification(
			fmt.Sprintf("Note %s ?", meta.Title),
			"Tu viens de regarder ce film",
			fmt.Sprintf("/title/%d", title.ID),
		)
	}
	return nil
}

func (s *PlexService) processEpisode(meta PlexMetadata, ids PlexExternalIDs, rawPayload string) error {
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
		seriesName := meta.GrandparentTitle
		if seriesName == "" {
			seriesName = meta.Title
		}

		seriesMeta := PlexMetadata{
			Title:    seriesName,
			Year:     meta.Year,
			Type:     "show",
			RatingKey: grandparentKey,
			GUID:     meta.GUID,
		}
		newTitle, names := s.buildNewTitle(seriesMeta, ids, model.TitleTypeSeries, grandparentKey)
		newTitle.Status = model.TitleStatusWatching

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

// buildNewTitle runs the matching pipeline (if available) and constructs a Title + names.
func (s *PlexService) buildNewTitle(meta PlexMetadata, ids PlexExternalIDs, titleType model.TitleType, ratingKey string) (*model.Title, []model.TitleName) {
	title := &model.Title{
		Type:          titleType,
		Year:          meta.Year,
		PlexRatingKey: &ratingKey,
	}

	if s.pipeline != nil {
		result, err := s.pipeline.Run(matching.MatchInput{
			Title:  meta.Title,
			Year:   meta.Year,
			Type:   titleType,
			IMDBID: ids.IMDB,
			TMDBID: ids.TMDB,
			TVDBID: ids.TVDB,
		})
		if err == nil {
			title.MatchStatus = result.MatchStatus
			title.MatchSource = &result.MatchSource
			title.OriginalTitle = &meta.Title
			title.Type = result.TitleType
			if result.IMDBID != "" {
				title.IMDBID = &result.IMDBID
			}
			if result.TMDBID != 0 {
				title.TMDBID = &result.TMDBID
			}
			if result.TVDBID != 0 {
				title.TVDBID = &result.TVDBID
			}
			if result.AniListID != 0 {
				title.AniListID = &result.AniListID
			}
			if result.CoverFile != "" {
				coverURL := "/covers/" + result.CoverFile
				title.CoverURL = &coverURL
			}
			return title, result.Names
		}
	}

	// Fallback: no pipeline or pipeline error
	title.MatchStatus = model.MatchStatusConfirmed
	title.OriginalTitle = &meta.Title
	fallbackSource := matching.MatchSourcePlexIDs
	title.MatchSource = &fallbackSource
	if ids.IMDB != "" {
		title.IMDBID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		title.TMDBID = &ids.TMDB
	}
	if ids.TVDB != 0 {
		title.TVDBID = &ids.TVDB
	}
	names := []model.TitleName{{Name: meta.Title, Language: "en", IsPrimary: true}}
	return title, names
}

func ParsePlexPayload(data []byte) (*PlexPayload, error) {
	var payload PlexPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse plex payload: %w", err)
	}
	return &payload, nil
}
