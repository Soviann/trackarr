package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

func (s *BackgroundService) refreshFromTMDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	if title.Type == model.TitleTypeMovie {
		s.refreshMovieFromTMDB(ctx, title, result)
	} else {
		s.refreshSeriesFromTMDB(ctx, title, result)
	}
}

func (s *BackgroundService) refreshMovieFromTMDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	details, err := s.tmdb.GetMovieDetails(ctx, *title.TMDBID)
	if err != nil {
		result.Error = err
		s.enqueueRefreshOnRetryable(ctx, title.ID, err)
		return
	}
	result.Refreshed = true

	if !s.hasValidCover(title) && details.PosterPath != nil {
		coverPath, err := s.tmdb.DownloadCover(ctx, *details.PosterPath, s.covers.Dir())
		if err == nil {
			logTitleUpdate(title.ID, "movie cover", s.updateTitle(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath}))
			s.covers.ExtractAndStoreAccent(ctx, title.ID, coverPath)
			title.CoverURL = &coverPath
		}
	}

	genres, credits, runtime, rating := matching.ExtractMovieMetadata(details)
	overview := details.Overview
	metaUpdate := repository.TitleUpdate{
		Overview: &overview,
		Credits:  &credits,
	}
	watchProviders := matching.ExtractFlatrateProvidersFR(details.WatchProviders)
	metaUpdate.WatchProviders = &watchProviders
	if runtime != nil {
		metaUpdate.Runtime = runtime
	}
	if rating != nil {
		metaUpdate.TMDBRating = rating
	}
	if oc := matching.ExtractOriginCountry(details.OriginCountry); oc != nil {
		metaUpdate.OriginCountry = oc
	}
	logTitleUpdate(title.ID, "movie metadata", s.updateTitle(ctx, title.ID, metaUpdate))

	if tmdbNames, err := s.tmdb.GetTitleNames(ctx, *title.TMDBID, "movie"); err == nil {
		s.syncTitleNames(ctx, title.ID, tmdbNames)
	}

	if genres != "" {
		var genreList []string
		if err := json.Unmarshal([]byte(genres), &genreList); err == nil && len(genreList) > 0 {
			if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
				return repository.NewGenreWriter(tx).ReplaceForTitle(ctx, title.ID, genreList)
			}); err != nil {
				log.Printf("background: save genres for title %d: %v", title.ID, err)
			}
		}
	}

	if !s.hasValidCover(title) && title.AniListID != nil {
		s.covers.DownloadAniListCover(ctx, title)
	}

	if title.TVDBID == nil && details.ExternalIDs != nil && details.ExternalIDs.TVDBID != 0 {
		tvdbID := details.ExternalIDs.TVDBID
		logTitleUpdate(title.ID, "movie tvdb backfill", s.updateTitle(ctx, title.ID, repository.TitleUpdate{TVDBID: &tvdbID}))
		title.TVDBID = &tvdbID
	}
}

func (s *BackgroundService) refreshSeriesFromTMDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	details, err := s.tmdb.GetTVDetails(ctx, *title.TMDBID)
	if err != nil {
		result.Error = err
		s.enqueueRefreshOnRetryable(ctx, title.ID, err)
		return
	}
	result.Refreshed = true

	newStatus := mapTMDBSeriesStatus(details)
	if newStatus != nil && (title.SeriesStatus == nil || *newStatus != *title.SeriesStatus) {
		result.StatusChanged = true
		if title.SeriesStatus != nil {
			result.OldStatus = *title.SeriesStatus
		}
		result.NewStatus = *newStatus
		logTitleUpdate(title.ID, "series status", s.updateTitle(ctx, title.ID, repository.TitleUpdate{SeriesStatus: newStatus}))
		title.SeriesStatus = newStatus

		if (*newStatus == model.SeriesStatusEnded || *newStatus == model.SeriesStatusCancelled) && IsNotificationEnabled(s.settings, NotifSeriesEnded) {
			if err := s.push.SendNotification(
				ctx,
				"PlexTracker",
				fmt.Sprintf("%s — Series ended", title.PrimaryName),
				fmt.Sprintf("/title/%d", title.ID),
			); err != nil {
				log.Printf("series-ended push failed for title %d: %v", title.ID, err)
			}
		}
	}

	if !s.hasValidCover(title) && details.PosterPath != nil {
		coverPath, err := s.tmdb.DownloadCover(ctx, *details.PosterPath, s.covers.Dir())
		if err == nil {
			logTitleUpdate(title.ID, "series cover", s.updateTitle(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath}))
			s.covers.ExtractAndStoreAccent(ctx, title.ID, coverPath)
			title.CoverURL = &coverPath
		}
	}

	genres, credits, runtime, rating := matching.ExtractTVMetadata(details)
	overview := details.Overview
	metaUpdate := repository.TitleUpdate{
		Overview: &overview,
		Credits:  &credits,
	}
	watchProviders := matching.ExtractFlatrateProvidersFR(details.WatchProviders)
	metaUpdate.WatchProviders = &watchProviders
	if runtime != nil {
		metaUpdate.Runtime = runtime
	}
	if rating != nil {
		metaUpdate.TMDBRating = rating
	}

	if details.NextEpisodeToAir != nil && details.NextEpisodeToAir.AirDate != "" {
		airDate := details.NextEpisodeToAir.AirDate
		airEp := fmt.Sprintf("S%d E%d", details.NextEpisodeToAir.SeasonNumber, details.NextEpisodeToAir.EpisodeNumber)
		metaUpdate.NextAirDate = &airDate
		metaUpdate.NextAirEpisode = &airEp
	}
	if oc := matching.ExtractOriginCountry(details.OriginCountry); oc != nil {
		metaUpdate.OriginCountry = oc
	}
	logTitleUpdate(title.ID, "series metadata", s.updateTitle(ctx, title.ID, metaUpdate))

	if tmdbNames, err := s.tmdb.GetTitleNames(ctx, *title.TMDBID, "tv"); err == nil {
		s.syncTitleNames(ctx, title.ID, tmdbNames)
	}

	if genres != "" {
		var genreList []string
		if err := json.Unmarshal([]byte(genres), &genreList); err == nil && len(genreList) > 0 {
			if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
				return repository.NewGenreWriter(tx).ReplaceForTitle(ctx, title.ID, genreList)
			}); err != nil {
				log.Printf("background: save genres for title %d: %v", title.ID, err)
			}
		}
	}

	if !s.hasValidCover(title) && title.AniListID != nil {
		s.covers.DownloadAniListCover(ctx, title)
	}

	if title.TVDBID == nil && details.ExternalIDs != nil && details.ExternalIDs.TVDBID != 0 {
		tvdbID := details.ExternalIDs.TVDBID
		logTitleUpdate(title.ID, "series tvdb backfill", s.updateTitle(ctx, title.ID, repository.TitleUpdate{TVDBID: &tvdbID}))
		title.TVDBID = &tvdbID
	}

	syncedFromTVDB := false
	if s.tvdb != nil && title.TVDBID != nil {
		syncedFromTVDB = s.refreshSeriesFromTVDB(ctx, title, result)
	}

	if !syncedFromTVDB {

		for _, tmdbSeason := range details.Seasons {
			if err := ctx.Err(); err != nil {
				return
			}

			if tmdbSeason.SeasonNumber == 0 {
				continue
			}

			tmdbEpisodes, err := s.tmdb.GetTVSeasonEpisodes(ctx, *title.TMDBID, tmdbSeason.SeasonNumber)
			if err != nil {
				continue
			}

			entries := make([]repository.EpisodeUpsert, len(tmdbEpisodes))
			for i, ep := range tmdbEpisodes {
				entries[i] = repository.EpisodeUpsert{
					EpisodeNumber: ep.EpisodeNumber,
					Name:          ep.Name,
					AirDate:       ep.AirDate,
				}
			}

			_ = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
				season, err := repository.NewSeasonWriter(tx).Upsert(ctx, title.ID, tmdbSeason.SeasonNumber, tmdbSeason.EpisodeCount)
				if err != nil {
					return err
				}
				return repository.NewEpisodeWriter(tx).UpsertBatch(ctx, season.ID, entries)
			})

			_ = s.limiter.Wait(ctx)
		}
	}
}
