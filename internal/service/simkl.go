package service

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type SimklBackup struct {
	Movies []SimklItem `json:"movies"`
	Shows  []SimklItem `json:"shows"`
	Anime  []SimklItem `json:"anime"`
}

type SimklItem struct {
	Status        string        `json:"status"`
	UserRating    *int          `json:"user_rating"`
	LastWatchedAt string        `json:"last_watched_at"`
	AnimeType     string        `json:"anime_type"`
	Seasons       []SimklSeason `json:"seasons"`
	// Nested media object (key varies: "movie" or "show")
	Movie *SimklMedia `json:"movie"`
	Show  *SimklMedia `json:"show"`
}

// Media returns the nested media object (movie or show).
func (i SimklItem) Media() *SimklMedia {
	if i.Movie != nil {
		return i.Movie
	}
	return i.Show
}

type SimklMedia struct {
	Title string   `json:"title"`
	Year  int      `json:"year"`
	IDs   SimklIDs `json:"ids"`
}

type SimklIDs struct {
	IMDB    string    `json:"imdb"`
	TMDB    flexInt64 `json:"tmdb"`
	AniList flexInt64 `json:"anilist"`
	TVDB    flexInt64 `json:"tvdb"`
}

// flexInt64 handles JSON values that may be int or string.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(data []byte) error {
	// Try int first
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		*f = flexInt64(i)
		return nil
	}
	// Try string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "" {
			*f = 0
			return nil
		}
		_, err := fmt.Sscanf(s, "%d", &i)
		if err != nil {
			return fmt.Errorf("flexInt64: cannot parse %q: %w", s, err)
		}
		*f = flexInt64(i)
		return nil
	}
	return fmt.Errorf("flexInt64: cannot unmarshal %s", string(data))
}

type SimklSeason struct {
	Number   int            `json:"number"`
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
	tasks    *repository.TaskRepository // optional, enables enrichment enqueue
	db       database.DBTX              // optional, enables episode backfill
}

type SimklImporterOption func(*SimklImporter)

func WithTaskRepository(tasks *repository.TaskRepository) SimklImporterOption {
	return func(s *SimklImporter) { s.tasks = tasks }
}

func WithBackfillDeps(db database.DBTX) SimklImporterOption {
	return func(s *SimklImporter) { s.db = db }
}

func NewSimklImporter(titles *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, opts ...SimklImporterOption) *SimklImporter {
	s := &SimklImporter{titles: titles, seasons: seasons, episodes: episodes, events: events}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *SimklImporter) Import(backup *SimklBackup, dryRun bool) (*ImportResult, error) {
	result := &ImportResult{}

	// Process movies
	for _, item := range backup.Movies {
		if err := s.importItem(item, model.TitleTypeMovie, false, dryRun, result); err != nil {
			result.Errors++
		}
	}

	// Process shows
	for _, item := range backup.Shows {
		if err := s.importItem(item, model.TitleTypeSeries, false, dryRun, result); err != nil {
			result.Errors++
		}
	}

	// Process anime
	for _, item := range backup.Anime {
		titleType := model.TitleTypeSeries
		if item.AnimeType == "movie" {
			titleType = model.TitleTypeMovie
		}
		if err := s.importItem(item, titleType, true, dryRun, result); err != nil {
			result.Errors++
		}
	}

	return result, nil
}

func (s *SimklImporter) importItem(item SimklItem, titleType model.TitleType, isAnime bool, dryRun bool, result *ImportResult) error {
	media := item.Media()
	if media == nil {
		result.Errors++
		return fmt.Errorf("no media object found")
	}

	// Check for duplicates by external ID
	var imdbID *string
	var tmdbID *int64
	if media.IDs.IMDB != "" {
		imdbID = &media.IDs.IMDB
	}
	if media.IDs.TMDB != 0 {
		v := int64(media.IDs.TMDB)
		tmdbID = &v
	}

	if existing, err := s.titles.FindByExternalID(imdbID, tmdbID, nil, nil, &titleType); err == nil && existing != nil {
		log.Printf("simkl import: skipped %q (%s) — already exists as %q (id=%d)", media.Title, titleType, existing.PrimaryName(), existing.ID)
		result.Skipped++
		return nil
	}

	if dryRun {
		result.Created++
		return nil
	}

	// Map status
	status := mapSimklStatus(item.Status)

	// Build title
	title := &model.Title{
		Type:        titleType,
		IsAnime:     isAnime,
		Year:        media.Year,
		Status:      status,
		MatchStatus: model.MatchStatusConfirmed,
		MyRating:    item.UserRating,
	}

	if media.IDs.IMDB != "" {
		title.IMDBID = &media.IDs.IMDB
	}
	if media.IDs.TMDB != 0 {
		v := int64(media.IDs.TMDB)
		title.TMDBID = &v
	}
	if media.IDs.AniList != 0 {
		v := int64(media.IDs.AniList)
		title.AniListID = &v
	}
	if media.IDs.TVDB != 0 {
		v := int64(media.IDs.TVDB)
		title.TVDBID = &v
	}

	names := []model.TitleName{{Name: media.Title, Language: "en", IsPrimary: true}}

	titleID, err := s.titles.Create(title, names)
	if err != nil {
		return fmt.Errorf("create title %q: %w", media.Title, err)
	}

	s.enqueueEnrichment(titleID, media.Title, media.Year, titleType, isAnime, media.IDs)

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

			if err := s.episodes.MarkWatched(ep.ID, watchedAt); err != nil {
				log.Printf("simkl import: mark episode %d watched for title %d: %v", ep.ID, titleID, err)
			}
			_, _ = s.events.Create(&model.WatchEvent{
				TitleID:   titleID,
				EpisodeID: &ep.ID,
				Source:    model.WatchEventSourceManual,
			})
		}
	}

	// Backfill previous episodes
	if s.db != nil && len(item.Seasons) > 0 {
		maxSeason, maxEpisode := 0, 0
		var latestWatchedAt time.Time
		for _, ss := range item.Seasons {
			for _, ep := range ss.Episodes {
				if ss.Number > maxSeason || (ss.Number == maxSeason && ep.Number > maxEpisode) {
					maxSeason = ss.Number
					maxEpisode = ep.Number
					if t, err := time.Parse(time.RFC3339, ep.WatchedAt); err == nil {
						latestWatchedAt = t
					}
				}
			}
		}
		if latestWatchedAt.IsZero() {
			latestWatchedAt = time.Now().UTC()
		}
		if maxSeason > 0 && maxEpisode > 0 {
			if err := BackfillPreviousEpisodes(s.db, nil, titleID, tmdbID, maxSeason, maxEpisode, latestWatchedAt); err != nil {
				log.Printf("simkl import: backfill for title %d: %v", titleID, err)
				result.Errors++
			}
		}
	}

	// Update last_watched_at if available
	if item.LastWatchedAt != "" {
		if parsedAt, err := time.Parse(time.RFC3339, item.LastWatchedAt); err == nil {
			if err := s.titles.UpdateLastWatchedAt(titleID, parsedAt); err != nil {
				log.Printf("simkl import: update last_watched_at for title %d: %v", titleID, err)
			}
		}
	}

	// For movies, also log watch event for stats (now decoupled from last_watched_at trigger)
	if titleType == model.TitleTypeMovie && item.LastWatchedAt != "" {
		_, _ = s.events.Create(&model.WatchEvent{
			TitleID: titleID,
			Source:  model.WatchEventSourceManual,
		})
	}

	result.Created++
	return nil
}

func (s *SimklImporter) enqueueEnrichment(titleID int64, name string, year int, titleType model.TitleType, isAnime bool, ids SimklIDs) {
	if s.tasks == nil {
		return
	}
	payload, err := json.Marshal(EnrichmentPayload{
		TitleID:   titleID,
		TitleName: name,
		Year:      year,
		TitleType: titleType,
		IsAnime:   isAnime,
		IMDBID:    ids.IMDB,
		TMDBID:    int64(ids.TMDB),
		TVDBID:    int64(ids.TVDB),
	})
	if err != nil {
		log.Printf("simkl import: enqueue enrichment for title %d: marshal payload: %v", titleID, err)
		return
	}
	dedupKey := fmt.Sprintf("enrichment:%d", titleID)
	if _, err := s.tasks.Enqueue(model.TaskTypeEnrichment, string(payload), &dedupKey); err != nil {
		log.Printf("simkl import: enqueue enrichment for title %d: %v", titleID, err)
	}
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
