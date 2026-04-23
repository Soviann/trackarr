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

// TMDBSeasonInfo is the minimal season metadata needed to backfill previous
// seasons. Callers fetch this from TMDB outside the write transaction to keep
// HTTP I/O out of the sole SQLite write connection.
type TMDBSeasonInfo struct {
	Number       int
	EpisodeCount int
}

// fetchTMDBSeasons fetches season metadata from TMDB. Returns nil on any
// failure (missing TMDB client, missing ID, or network error) so the caller
// can still run a same-season backfill.
func fetchTMDBSeasons(ctx context.Context, tmdb *matching.TMDBClient, titleID int64, tmdbID *int64) []TMDBSeasonInfo {
	if tmdb == nil || tmdbID == nil {
		return nil
	}
	details, err := tmdb.GetTVDetails(ctx, *tmdbID)
	if err != nil {
		log.Printf("backfill: TMDB fetch failed for title %d: %v", titleID, err)
		return nil
	}
	seasons := make([]TMDBSeasonInfo, 0, len(details.Seasons))
	for _, s := range details.Seasons {
		if s.SeasonNumber == 0 {
			continue // skip specials
		}
		seasons = append(seasons, TMDBSeasonInfo{
			Number:       s.SeasonNumber,
			EpisodeCount: s.EpisodeCount,
		})
	}
	return seasons
}

// BackfillForEpisode resolves season/episode context, fetches TMDB metadata
// outside the write transaction, then runs the DB-only backfill inside a
// transaction tied to ctx.
func (s *BackfillService) BackfillForEpisode(ctx context.Context, titleID int64, episode *model.Episode) {
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

	tmdbSeasons := fetchTMDBSeasons(ctx, s.tmdb, title.ID, title.TMDBID)

	if err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		return BackfillPreviousEpisodes(ctx, tx, title.ID, tmdbSeasons, season.SeasonNumber, episode.Episode, time.Now().UTC())
	}); err != nil {
		log.Printf("backfill warning for title %d: %v", titleID, err)
	}
}

// BackfillPreviousEpisodes creates and marks as watched all episodes before the
// given season/episode number. Previous-season backfill requires prefetched
// TMDB season data (see fetchTMDBSeasons); without it, only episodes in the
// current season are backfilled. Must run inside a transaction: episode
// writes are tx-only and the entire backfill (season upsert, episode upsert,
// batch mark, watch events) must commit atomically.
func BackfillPreviousEpisodes(
	ctx context.Context,
	tx *sql.Tx,
	titleID int64,
	tmdbSeasons []TMDBSeasonInfo,
	triggerSeasonNum int,
	triggerEpisodeNum int,
	watchedAt time.Time,
) error {
	// S01E01 has nothing to backfill
	if triggerSeasonNum <= 1 && triggerEpisodeNum <= 1 {
		return nil
	}

	seasons := repository.NewSeasonWriter(tx)
	episodes := repository.NewEpisodeWriter(tx)
	events := repository.NewWatchEventRepository(tx)

	var toMarkIDs []int64
	var toEventTitleID = titleID

	// Backfill previous seasons (only with TMDB data)
	for _, si := range tmdbSeasons {
		if si.Number >= triggerSeasonNum {
			continue
		}
		season, err := seasons.GetOrCreate(ctx, titleID, si.Number)
		if err != nil {
			log.Printf("backfill: create season %d: %v", si.Number, err)
			continue
		}
		if err := seasons.UpdateTotalEpisodes(ctx, season.ID, si.EpisodeCount); err != nil {
			log.Printf("backfill: update total episodes for season %d: %v", season.ID, err)
		}

		for epNum := 1; epNum <= si.EpisodeCount; epNum++ {
			ep, err := episodes.GetOrCreate(ctx, season.ID, epNum)
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
		season, err := seasons.GetOrCreate(ctx, titleID, triggerSeasonNum)
		if err != nil {
			return err
		}

		// Update total_episodes from TMDB if available
		for _, si := range tmdbSeasons {
			if si.Number == triggerSeasonNum {
				if err := seasons.UpdateTotalEpisodes(ctx, season.ID, si.EpisodeCount); err != nil {
					log.Printf("backfill: update total episodes for season %d: %v", season.ID, err)
				}
				break
			}
		}

		for epNum := 1; epNum < triggerEpisodeNum; epNum++ {
			ep, err := episodes.GetOrCreate(ctx, season.ID, epNum)
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

	if err := episodes.BatchMarkWatched(ctx, toMarkIDs, watchedAt); err != nil {
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
