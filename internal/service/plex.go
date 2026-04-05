package service

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	plexwebhooks "github.com/hekmon/plexwebhooks"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type PlexExternalIDs struct {
	IMDB string
	TMDB int64
	TVDB int64
}

func ParseGUIDs(guids []*url.URL) PlexExternalIDs {
	var ids PlexExternalIDs
	for _, g := range guids {
		if g == nil {
			continue
		}
		raw := g.String()
		switch {
		case strings.HasPrefix(raw, "imdb://"):
			ids.IMDB = strings.TrimPrefix(raw, "imdb://")
		case strings.HasPrefix(raw, "tmdb://"):
			_, _ = fmt.Sscanf(strings.TrimPrefix(raw, "tmdb://"), "%d", &ids.TMDB)
		case strings.HasPrefix(raw, "tvdb://"):
			_, _ = fmt.Sscanf(strings.TrimPrefix(raw, "tvdb://"), "%d", &ids.TVDB)
		}
	}
	return ids
}

type PlexService struct {
	db       *sql.DB
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
	pipeline *matching.Pipeline // nil = skip matching, create with basic info
	push     PushNotifier
}

func NewPlexService(db *sql.DB, titles *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, pipeline *matching.Pipeline, push PushNotifier) *PlexService {
	return &PlexService{db: db, titles: titles, seasons: seasons, episodes: episodes, events: events, pipeline: pipeline, push: push}
}

func (s *PlexService) ProcessScrobble(payload *plexwebhooks.Payload, rawPayload string) error {
	if payload.Event != plexwebhooks.EventTypeScrobble {
		return nil
	}

	meta := payload.Metadata
	ids := ParseGUIDs(meta.GUIDExternal)

	switch meta.Type {
	case plexwebhooks.MediaTypeMovie:
		return s.processMovie(meta, ids, rawPayload)
	case plexwebhooks.MediaTypeEpisode:
		return s.processEpisode(meta, ids, rawPayload)
	default:
		return fmt.Errorf("unknown media type: %s", meta.Type)
	}
}

func (s *PlexService) processMovie(meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string) error {
	return database.WithTx(s.db, func(tx *sql.Tx) error {
		return s.processMovieInTx(tx, meta, ids, rawPayload)
	})
}

func (s *PlexService) processMovieInTx(tx *sql.Tx, meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string) error {
	titles := repository.NewTitleRepository(tx)
	events := repository.NewWatchEventRepository(tx)
	var imdbID *string
	var tmdbID *int64
	ratingKey := meta.RatingKey

	if ids.IMDB != "" {
		imdbID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		tmdbID = &ids.TMDB
	}

	title, err := titles.FindByExternalID(imdbID, tmdbID, &ratingKey)
	if err != nil {
		newTitle, names := s.buildNewTitle(meta.Title, meta.Year, ids, model.TitleTypeMovie, ratingKey, meta.GUIDExternal)
		newTitle.Status = model.TitleStatusCompleted

		titleID, err := titles.Create(newTitle, names)
		if err != nil {
			return fmt.Errorf("create movie: %w", err)
		}

		_, _ = events.Create(&model.WatchEvent{
			TitleID:     titleID,
			Source:      model.WatchEventSourcePlex,
			PlexPayload: &rawPayload,
		})

		_ = s.push.SendNotification(
			fmt.Sprintf("Note %s ?", meta.Title),
			"Tu viens de regarder ce film",
			fmt.Sprintf("/title/%d", titleID),
		)
		return nil
	}

	_, _ = events.Create(&model.WatchEvent{
		TitleID:     title.ID,
		Source:      model.WatchEventSourcePlex,
		PlexPayload: &rawPayload,
	})

	if title.MyRating == nil {
		_ = s.push.SendNotification(
			fmt.Sprintf("Note %s ?", meta.Title),
			"Tu viens de regarder ce film",
			fmt.Sprintf("/title/%d", title.ID),
		)
	}
	return nil
}

func (s *PlexService) processEpisode(meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string) error {
	return database.WithTx(s.db, func(tx *sql.Tx) error {
		return s.processEpisodeInTx(tx, meta, ids, rawPayload)
	})
}

func (s *PlexService) processEpisodeInTx(tx *sql.Tx, meta plexwebhooks.Metadata, ids PlexExternalIDs, rawPayload string) error {
	titles := repository.NewTitleRepository(tx)
	seasons := repository.NewSeasonRepository(tx)
	episodes := repository.NewEpisodeRepository(tx)
	events := repository.NewWatchEventRepository(tx)

	grandparentKey := meta.GrandparentRatingKey
	var imdbID *string
	var tmdbID *int64

	if ids.IMDB != "" {
		imdbID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		tmdbID = &ids.TMDB
	}

	title, err := titles.FindByExternalID(imdbID, tmdbID, &grandparentKey)
	if err != nil {
		seriesName := meta.GrandparentTitle
		if seriesName == "" {
			seriesName = meta.Title
		}

		newTitle, names := s.buildNewTitle(seriesName, meta.Year, ids, model.TitleTypeSeries, grandparentKey, meta.GUIDExternal)
		newTitle.Status = model.TitleStatusWatching

		titleID, createErr := titles.Create(newTitle, names)
		if createErr != nil {
			return fmt.Errorf("create series: %w", createErr)
		}
		title = &model.Title{ID: titleID}
	}

	season, err := seasons.GetOrCreate(title.ID, meta.ParentIndex)
	if err != nil {
		return fmt.Errorf("get/create season: %w", err)
	}

	ep, err := episodes.GetOrCreate(season.ID, meta.Index)
	if err != nil {
		return fmt.Errorf("get/create episode: %w", err)
	}

	now := time.Now().UTC()
	if !ep.Watched {
		_ = episodes.MarkWatched(ep.ID, now)
	}

	_, _ = events.Create(&model.WatchEvent{
		TitleID:     title.ID,
		EpisodeID:   &ep.ID,
		Source:      model.WatchEventSourcePlex,
		PlexPayload: &rawPayload,
	})

	// Backfill previous episodes — prefer title's TMDB ID (from DB) over Plex GUIDs
	backfillTMDBID := title.TMDBID
	if backfillTMDBID == nil {
		backfillTMDBID = tmdbID
	}
	var tmdbClient *matching.TMDBClient
	if s.pipeline != nil {
		tmdbClient = s.pipeline.TMDB()
	}
	if err := BackfillPreviousEpisodes(tx, tmdbClient, title.ID, backfillTMDBID, meta.ParentIndex, meta.Index, now); err != nil {
		log.Printf("backfill warning: %v", err)
	}

	return nil
}

// buildNewTitle runs the matching pipeline (if available) and constructs a Title + names.
func (s *PlexService) buildNewTitle(title string, year int, ids PlexExternalIDs, titleType model.TitleType, ratingKey string, guids []*url.URL) (*model.Title, []model.TitleName) {
	t := &model.Title{
		Type:          titleType,
		Year:          year,
		PlexRatingKey: &ratingKey,
	}

	if s.pipeline != nil {
		result, err := s.pipeline.Run(matching.MatchInput{
			Title:  title,
			Year:   year,
			Type:   titleType,
			IMDBID: ids.IMDB,
			TMDBID: ids.TMDB,
			TVDBID: ids.TVDB,
		})
		if err == nil {
			t.MatchStatus = result.MatchStatus
			t.MatchSource = &result.MatchSource
			t.OriginalTitle = &title
			t.Type = result.TitleType
			if result.IMDBID != "" {
				t.IMDBID = &result.IMDBID
			}
			if result.TMDBID != 0 {
				t.TMDBID = &result.TMDBID
			}
			if result.TVDBID != 0 {
				t.TVDBID = &result.TVDBID
			}
			if result.AniListID != 0 {
				t.AniListID = &result.AniListID
			}
			if result.CoverFile != "" {
				coverURL := "/covers/" + result.CoverFile
				t.CoverURL = &coverURL
			}
			return t, result.Names
		}
	}

	// Fallback: no pipeline or pipeline error
	t.MatchStatus = model.MatchStatusConfirmed
	t.OriginalTitle = &title
	fallbackSource := matching.MatchSourcePlexIDs
	t.MatchSource = &fallbackSource
	if ids.IMDB != "" {
		t.IMDBID = &ids.IMDB
	}
	if ids.TMDB != 0 {
		t.TMDBID = &ids.TMDB
	}
	if ids.TVDB != 0 {
		t.TVDBID = &ids.TVDB
	}
	names := []model.TitleName{{Name: title, Language: "en", IsPrimary: true}}
	return t, names
}
