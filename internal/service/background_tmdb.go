package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
)

func (s *BackgroundService) refreshFromTMDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) map[string]string {
	if title.Type == model.TitleTypeMovie {
		return s.refreshMovieFromTMDB(ctx, title, result)
	}
	return s.refreshSeriesFromTMDB(ctx, title, result)
}

func (s *BackgroundService) refreshMovieFromTMDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) map[string]string {
	details, err := s.tmdb.GetMovieDetails(ctx, *title.TMDBID)
	if err != nil {
		result.Error = err
		s.enqueueRefreshOnRetryable(ctx, title.ID, err)
		return nil
	}
	result.Refreshed = true

	if s.covers != nil && !s.hasValidCover(title) && details.PosterPath != nil {
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

	var tmdbNames map[string]string
	if names, err := s.tmdb.GetTitleNames(ctx, *title.TMDBID, "movie"); err == nil {
		tmdbNames = names
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

	s.refreshTMDBMovieCollection(ctx, title, details, result)

	return tmdbNames
}

func (s *BackgroundService) refreshSeriesFromTMDB(ctx context.Context, title *repository.TitleLite, result *RefreshResult) map[string]string {
	details, err := s.tmdb.GetTVDetails(ctx, *title.TMDBID)
	if err != nil {
		result.Error = err
		s.enqueueRefreshOnRetryable(ctx, title.ID, err)
		return nil
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
				"Trackarr",
				fmt.Sprintf("%s — Series ended", title.PrimaryName),
				fmt.Sprintf("/title/%d", title.ID),
			); err != nil {
				log.Printf("series-ended push failed for title %d: %v", title.ID, err)
			}
		}
	}

	if s.covers != nil && !s.hasValidCover(title) && details.PosterPath != nil {
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

	var tmdbNames map[string]string
	if names, err := s.tmdb.GetTitleNames(ctx, *title.TMDBID, "tv"); err == nil {
		tmdbNames = names
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
		var ptSeasons []model.Season
		if fullTitle, err := s.titles.GetByID(title.ID); err == nil && fullTitle != nil {
			ptSeasons = fullTitle.Seasons
		}
		ptSeasonMap := make(map[int]*model.Season)
		maxPTSeasonNum := 0
		for i := range ptSeasons {
			ptSeasonMap[ptSeasons[i].SeasonNumber] = &ptSeasons[i]
			if ptSeasons[i].SeasonNumber > maxPTSeasonNum {
				maxPTSeasonNum = ptSeasons[i].SeasonNumber
			}
		}

		partsBySeason, _ := s.seasonExtIDs.ListPartsForTitle(ctx, title.ID, repository.ProviderAniList)

		for _, tmdbSeason := range details.Seasons {
			if err := ctx.Err(); err != nil {
				return tmdbNames
			}
			if tmdbSeason.SeasonNumber == 0 {
				continue
			}

			tmdbEpisodes, err := s.tmdb.GetTVSeasonEpisodes(ctx, *title.TMDBID, tmdbSeason.SeasonNumber)
			if err != nil {
				continue
			}

			// Determine target Trackarr season number:
			// 1. Direct match: ptSeasonMap has tmdbSeason.SeasonNumber AND (no higher maxPTSeasonNum or not anime)
			// 2. AniList start_date / external_id match for anime seasons
			// 3. Fallback: map by start_date or tmdbSeason.SeasonNumber
			targetSeasonNum := tmdbSeason.SeasonNumber

			if title.IsAnime && len(ptSeasons) > 0 {
				// Check if TMDB season air date matches a mapped AniList part's start date
				if tmdbSeason.AirDate != "" {
					for seasonID, parts := range partsBySeason {
						for _, part := range parts {
							if part.StartDate != nil && *part.StartDate == tmdbSeason.AirDate {
								for _, pts := range ptSeasons {
									if pts.ID == seasonID {
										targetSeasonNum = pts.SeasonNumber
										break
									}
								}
							}
						}
					}
				}

				// If TMDB Season 1 contains combined episodes for multiple Trackarr seasons (S1, S2, S3...)
				if tmdbSeason.SeasonNumber == 1 && len(tmdbEpisodes) > 0 && maxPTSeasonNum > 1 {
					cum := 0
					for _, pts := range ptSeasons {
						if cum >= len(tmdbEpisodes) {
							break
						}
						epCount := 0
						if pts.TotalEpisodes != nil {
							epCount = *pts.TotalEpisodes
						}
						if parts, ok := partsBySeason[pts.ID]; ok && len(parts) > 0 && parts[0].EpisodeCount != nil {
							epCount = *parts[0].EpisodeCount
						}
						if epCount <= 0 {
							epCount = len(tmdbEpisodes) - cum
						}

						end := cum + epCount
						if end > len(tmdbEpisodes) {
							end = len(tmdbEpisodes)
						}

						slice := tmdbEpisodes[cum:end]
						entries := make([]repository.EpisodeUpsert, len(slice))
						for i, ep := range slice {
							entries[i] = repository.EpisodeUpsert{
								EpisodeNumber: i + 1,
								Name:          ep.Name,
								AirDate:       ep.AirDate,
							}
						}
						currPTS := pts
						_ = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
							season, err := repository.NewSeasonWriter(tx).Upsert(ctx, title.ID, currPTS.SeasonNumber, len(slice))
							if err != nil {
								return err
							}
							if err := repository.NewEpisodeWriter(tx).UpsertBatch(ctx, season.ID, entries); err != nil {
								return err
							}
							return repository.NewEpisodeWriter(tx).DeleteBeyond(ctx, season.ID, len(slice))
						})
						cum = end
					}
					_ = s.limiter.Wait(ctx)
					continue
				}
			}

			entries := make([]repository.EpisodeUpsert, len(tmdbEpisodes))
			maxEp := 0
			for i, ep := range tmdbEpisodes {
				if ep.EpisodeNumber > maxEp {
					maxEp = ep.EpisodeNumber
				}
				entries[i] = repository.EpisodeUpsert{
					EpisodeNumber: ep.EpisodeNumber,
					Name:          ep.Name,
					AirDate:       ep.AirDate,
				}
			}

			_ = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
				season, err := repository.NewSeasonWriter(tx).Upsert(ctx, title.ID, targetSeasonNum, tmdbSeason.EpisodeCount)
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

			_ = s.limiter.Wait(ctx)
		}
	}

	return tmdbNames
}
