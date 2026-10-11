package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

type ArrWebhookService struct {
	writeDB   *sql.DB
	titleRepo *repository.TitleRepository
	settings  *repository.SettingRepository
	push      PushNotifier
}

func NewArrWebhookService(
	writeDB *sql.DB,
	titleRepo *repository.TitleRepository,
	settings *repository.SettingRepository,
	push PushNotifier,
) *ArrWebhookService {
	return &ArrWebhookService{
		writeDB:   writeDB,
		titleRepo: titleRepo,
		settings:  settings,
		push:      push,
	}
}

// ProcessArrWebhook handles an incoming webhook from Radarr or Sonarr.
func (s *ArrWebhookService) ProcessArrWebhook(ctx context.Context, payload *model.ArrWebhookPayload) error {
	if payload == nil {
		return nil
	}

	if payload.EventType == "Test" {
		log.Printf("arr webhook: test event received (instance: %s)", payload.InstanceName)
		return nil
	}

	if payload.EventType != "Download" && payload.EventType != "Upgrade" {
		log.Printf("arr webhook: ignored unhandled eventType %q", payload.EventType)
		return nil
	}

	if payload.Movie != nil {
		return s.processRadarrDownload(ctx, payload.Movie)
	}

	if payload.Series != nil {
		return s.processSonarrDownload(ctx, payload.Series, payload.Episodes)
	}

	return nil
}

func (s *ArrWebhookService) processRadarrDownload(ctx context.Context, movie *model.ArrMoviePayload) error {
	var targetTitle *model.Title
	var err error

	if movie.ID > 0 {
		targetTitle, err = s.titleRepo.FindByRadarrID(movie.ID)
	}
	if targetTitle == nil && movie.TMDBID > 0 {
		movieType := model.TitleTypeMovie
		targetTitle, err = s.titleRepo.FindByExternalID(nil, &movie.TMDBID, nil, nil, nil, &movieType)
	}
	if targetTitle == nil && movie.IMDBID != "" {
		movieType := model.TitleTypeMovie
		targetTitle, err = s.titleRepo.FindByExternalID(&movie.IMDBID, nil, nil, nil, nil, &movieType)
	}

	if targetTitle == nil {
		log.Printf("arr webhook: movie not found in library: %q (tmdb: %d, radarr: %d, err: %v)", movie.Title, movie.TMDBID, movie.ID, err)
		return nil
	}

	now := time.Now().UTC()
	err = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		tw := repository.NewTitleWriter(tx)
		if err := tw.MarkDownloaded(ctx, targetTitle.ID, now); err != nil {
			return err
		}
		if targetTitle.RadarrID == nil && movie.ID > 0 {
			if err := tw.Update(ctx, targetTitle.ID, repository.TitleUpdate{RadarrID: &movie.ID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("process radarr download: %w", err)
	}

	// Post-commit side effect: send notification if enabled
	if s.push != nil && IsNotificationEnabled(s.settings, NotifMediaDownloaded) {
		name := targetTitle.PrimaryName()
		if name == "" {
			name = movie.Title
		}
		body := fmt.Sprintf("« %s » est disponible au visionnage", name)
		link := fmt.Sprintf("/title/%d", targetTitle.ID)
		if pushErr := s.push.SendNotification(ctx, "Trackarr", body, link); pushErr != nil {
			log.Printf("arr webhook: send notification error: %v", pushErr)
		}
	}

	return nil
}

func (s *ArrWebhookService) processSonarrDownload(ctx context.Context, series *model.ArrSeriesPayload, episodes []model.ArrEpisodePayload) error {
	var targetTitle *model.Title
	var err error

	if series.ID > 0 {
		targetTitle, err = s.titleRepo.FindBySonarrID(series.ID)
	}
	if targetTitle == nil && series.TVDBID > 0 {
		seriesType := model.TitleTypeSeries
		targetTitle, err = s.titleRepo.FindByExternalID(nil, nil, nil, nil, &series.TVDBID, &seriesType)
	}
	if targetTitle == nil && series.IMDBID != "" {
		seriesType := model.TitleTypeSeries
		targetTitle, err = s.titleRepo.FindByExternalID(&series.IMDBID, nil, nil, nil, nil, &seriesType)
	}

	if targetTitle == nil {
		log.Printf("arr webhook: series not found in library: %q (tvdb: %d, sonarr: %d, err: %v)", series.Title, series.TVDBID, series.ID, err)
		return nil
	}

	now := time.Now().UTC()
	err = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		tw := repository.NewTitleWriter(tx)
		ew := repository.NewEpisodeWriter(tx)

		if targetTitle.SonarrID == nil && series.ID > 0 {
			if err := tw.Update(ctx, targetTitle.ID, repository.TitleUpdate{SonarrID: &series.ID}); err != nil {
				return err
			}
		}

		for _, ep := range episodes {
			if ep.SeasonNumber >= 0 && ep.EpisodeNumber > 0 {
				if _, err := ew.MarkDownloadedBySeasonAndEpisode(ctx, targetTitle.ID, ep.SeasonNumber, ep.EpisodeNumber, now); err != nil {
					log.Printf("arr webhook: mark episode downloaded error (title %d S%dE%d): %v", targetTitle.ID, ep.SeasonNumber, ep.EpisodeNumber, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("process sonarr download: %w", err)
	}

	// Post-commit side effect: send notification if enabled
	if s.push != nil && IsNotificationEnabled(s.settings, NotifMediaDownloaded) && len(episodes) > 0 {
		name := targetTitle.PrimaryName()
		if name == "" {
			name = series.Title
		}
		var body string
		if len(episodes) == 1 {
			body = fmt.Sprintf("« %s » S%02dE%02d est disponible", name, episodes[0].SeasonNumber, episodes[0].EpisodeNumber)
		} else {
			body = fmt.Sprintf("« %s » (%d épisodes) sont disponibles", name, len(episodes))
		}
		link := fmt.Sprintf("/title/%d", targetTitle.ID)
		if pushErr := s.push.SendNotification(ctx, "Trackarr", body, link); pushErr != nil {
			log.Printf("arr webhook: send notification error: %v", pushErr)
		}
	}

	return nil
}
