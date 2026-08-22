package service

import (
	"context"
	"database/sql"
	"log"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

// refreshFromTVDB fetches TVDB data for titles that have a TVDB ID.
// TVDB ID cross-referencing from TMDB is handled in refreshMovieFromTMDB / refreshSeriesFromTMDB.
// For titles with a TMDB ID, overview and genres are refreshed from TMDB; here only the cover is updated.
// For titles without a TMDB ID, overview and genres are also persisted from TVDB.
// refreshFromTVDB fetches TVDB data for titles that have a TVDB ID.
// TVDB ID cross-referencing from TMDB is handled in refreshMovieFromTMDB / refreshSeriesFromTMDB.
// For titles with a TMDB ID, overview and genres are refreshed from TMDB; here only the cover is updated.
// For titles without a TMDB ID, overview and genres are also persisted from TVDB.
func (s *BackgroundService) refreshFromTVDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) map[string]string {
	if title.TVDBID == nil {
		return nil
	}
	tvdbID := *title.TVDBID

	update := repository.TitleUpdate{}
	var tvdbNames map[string]string
	if title.Type == model.TitleTypeMovie {
		details, err := s.tvdb.GetMovieDetails(ctx, tvdbID)
		if err != nil {
			log.Printf("background tvdb movie refresh %d: %v", title.ID, err)
			return nil
		}
		result.Refreshed = true
		tvdbNames = details.Names()
		if !s.hasValidCover(title) && details.Image != "" {
			if filename, err := s.tvdb.DownloadCover(ctx, details.Image, tvdbID, s.covers.Dir()); err == nil {
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
			if len(genreList) > 0 {
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
			return nil
		}
		result.Refreshed = true
		tvdbNames = details.Names()
		if title.CoverURL == nil && details.Image != "" {
			if filename, err := s.tvdb.DownloadCover(ctx, details.Image, tvdbID, s.covers.Dir()); err == nil {
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
			if len(genreList) > 0 {
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
		if update.CoverURL != nil {
			s.covers.ExtractAndStoreAccent(ctx, title.ID, *update.CoverURL)
		}
	}
	return tvdbNames
}

// refreshSeriesFromTVDB syncs season and episode listings from TVDB.
// Returns true if TVDB season sync succeeded.
// refreshSeriesFromTVDB syncs season and episode listings from TVDB.
// Returns true if TVDB season sync succeeded.
func (s *BackgroundService) refreshSeriesFromTVDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) bool {
	if s.tvdb == nil || title.TVDBID == nil {
		return false
	}
	tvdbID := *title.TVDBID

	episodesBySeason, err := s.tvdb.GetSeriesEpisodes(ctx, tvdbID)
	if err != nil {
		log.Printf("background tvdb series episodes refresh %d: %v", title.ID, err)
		return false
	}
	result.Refreshed = true

	for seasonNum, episodes := range episodesBySeason {
		if err := ctx.Err(); err != nil {
			return true
		}
		if seasonNum == 0 {
			continue
		}

		entries := make([]repository.EpisodeUpsert, len(episodes))
		maxEp := 0
		for i, ep := range episodes {
			if ep.Number > maxEp {
				maxEp = ep.Number
			}
			entries[i] = repository.EpisodeUpsert{
				EpisodeNumber: ep.Number,
				Name:          ep.Name,
				AirDate:       ep.Aired,
			}
		}

		_ = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
			season, err := repository.NewSeasonWriter(tx).Upsert(ctx, title.ID, seasonNum, len(episodes))
			if err != nil {
				return err
			}
			if err := repository.NewEpisodeWriter(tx).UpsertBatch(ctx, season.ID, entries); err != nil {
				return err
			}
			if maxEp > 0 {
				return repository.NewEpisodeWriter(tx).DeleteBeyond(ctx, season.ID, maxEp)
			}
			return nil
		})
	}

	return true
}
