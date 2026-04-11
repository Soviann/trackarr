package service

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

// BackfillService orchestrates episode backfill from the handler layer.
type BackfillService struct {
	db   *sql.DB
	tmdb *matching.TMDBClient
}

func NewBackfillService(db *sql.DB, tmdb *matching.TMDBClient) *BackfillService {
	return &BackfillService{db: db, tmdb: tmdb}
}

// BackfillForEpisode resolves season/episode context and runs backfill in a transaction.
func (s *BackfillService) BackfillForEpisode(titleID int64, episode *model.Episode) {
	seasonRepo := repository.NewSeasonRepository(s.db)
	titleRepo := repository.NewTitleRepository(s.db)

	season, err := seasonRepo.GetByID(episode.SeasonID)
	if err != nil {
		log.Printf("backfill: get season: %v", err)
		return
	}

	title, err := titleRepo.GetByID(titleID)
	if err != nil {
		log.Printf("backfill: get title: %v", err)
		return
	}

	if err := database.WithTx(s.db, func(tx *sql.Tx) error {
		return BackfillPreviousEpisodes(tx, s.tmdb, title.ID, title.TMDBID, season.SeasonNumber, episode.Episode, time.Now().UTC())
	}); err != nil {
		log.Printf("backfill warning for title %d: %v", titleID, err)
	}
}

// BackfillPreviousEpisodes creates and marks as watched all episodes before the
// given season/episode number. If TMDB is available, previous seasons are also
// backfilled. Without TMDB, only episodes in the current season are backfilled.
func BackfillPreviousEpisodes(
	db database.DBTX,
	tmdb *matching.TMDBClient,
	titleID int64,
	tmdbID *int64,
	triggerSeasonNum int,
	triggerEpisodeNum int,
	watchedAt time.Time,
) error {
	// S01E01 has nothing to backfill
	if triggerSeasonNum <= 1 && triggerEpisodeNum <= 1 {
		return nil
	}

	seasons := repository.NewSeasonRepository(db)
	episodes := repository.NewEpisodeRepository(db)
	events := repository.NewWatchEventRepository(db)

	var toMarkIDs []int64
	var toEventTitleID = titleID

	// Fetch TMDB season data if available
	type seasonInfo struct {
		Number       int
		EpisodeCount int
	}
	var tmdbSeasons []seasonInfo

	if tmdb != nil && tmdbID != nil {
		details, err := tmdb.GetTVDetails(context.Background(), *tmdbID)
		if err != nil {
			log.Printf("backfill: TMDB fetch failed for title %d: %v", titleID, err)
		} else {
			for _, s := range details.Seasons {
				if s.SeasonNumber == 0 {
					continue // skip specials
				}
				tmdbSeasons = append(tmdbSeasons, seasonInfo{
					Number:       s.SeasonNumber,
					EpisodeCount: s.EpisodeCount,
				})
			}
		}
	}

	// Backfill previous seasons (only with TMDB data)
	for _, si := range tmdbSeasons {
		if si.Number >= triggerSeasonNum {
			continue
		}
		season, err := seasons.GetOrCreate(titleID, si.Number)
		if err != nil {
			log.Printf("backfill: create season %d: %v", si.Number, err)
			continue
		}
		if err := seasons.UpdateTotalEpisodes(season.ID, si.EpisodeCount); err != nil {
			log.Printf("backfill: update total episodes for season %d: %v", season.ID, err)
		}

		for epNum := 1; epNum <= si.EpisodeCount; epNum++ {
			ep, err := episodes.GetOrCreate(season.ID, epNum)
			if err != nil {
				continue
			}
			if !ep.Watched {
				toMarkIDs = append(toMarkIDs, ep.ID)
			}
		}
	}

	// Backfill current season (episodes before trigger)
	if triggerEpisodeNum > 1 {
		season, err := seasons.GetOrCreate(titleID, triggerSeasonNum)
		if err != nil {
			return err
		}

		// Update total_episodes from TMDB if available
		for _, si := range tmdbSeasons {
			if si.Number == triggerSeasonNum {
				if err := seasons.UpdateTotalEpisodes(season.ID, si.EpisodeCount); err != nil {
					log.Printf("backfill: update total episodes for season %d: %v", season.ID, err)
				}
				break
			}
		}

		for epNum := 1; epNum < triggerEpisodeNum; epNum++ {
			ep, err := episodes.GetOrCreate(season.ID, epNum)
			if err != nil {
				continue
			}
			if !ep.Watched {
				toMarkIDs = append(toMarkIDs, ep.ID)
			}
		}
	}

	if len(toMarkIDs) == 0 {
		return nil
	}

	if err := episodes.BatchMarkWatched(toMarkIDs, watchedAt); err != nil {
		return err
	}

	// Create watch events for backfilled episodes
	watchEvents := make([]model.WatchEvent, len(toMarkIDs))
	for i, epID := range toMarkIDs {
		id := epID
		watchEvents[i] = model.WatchEvent{
			TitleID:   toEventTitleID,
			EpisodeID: &id,
			Source:    model.WatchEventSourceBackfill,
		}
	}

	return events.BatchCreate(watchEvents)
}
