package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
)

func (w *TaskQueueWorker) handleRefresh(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload RefreshPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode refresh payload: %w", err)
	}
	logger = logger.With("titleID", payload.TitleID)

	if w.tmdb == nil {
		return fmt.Errorf("TMDB client not configured")
	}

	title, err := w.titles.GetByID(payload.TitleID)
	if err != nil {
		return fmt.Errorf("get title %d: %w", payload.TitleID, err)
	}

	if title.TMDBID != nil && title.CoverURL == nil {
		if title.Type == model.TitleTypeMovie {
			details, err := w.tmdb.GetMovieDetails(ctx, *title.TMDBID)
			if err != nil {
				return err
			}
			if details.PosterPath != nil {
				coverPath, err := w.tmdb.DownloadCover(ctx, *details.PosterPath, fmt.Sprintf("%s/covers", w.dataDir))
				if err == nil {
					_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
						return repository.NewTitleWriter(tx).Update(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath})
					})
					if w.covers != nil {
						w.covers.ExtractAndStoreAccent(ctx, title.ID, coverPath)
					}
					title.CoverURL = &coverPath
				}
			}
		} else {
			details, err := w.tmdb.GetTVDetails(ctx, *title.TMDBID)
			if err != nil {
				return err
			}
			if details.PosterPath != nil {
				coverPath, err := w.tmdb.DownloadCover(ctx, *details.PosterPath, fmt.Sprintf("%s/covers", w.dataDir))
				if err == nil {
					_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
						return repository.NewTitleWriter(tx).Update(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath})
					})
					if w.covers != nil {
						w.covers.ExtractAndStoreAccent(ctx, title.ID, coverPath)
					}
					title.CoverURL = &coverPath
				}
			}
		}
	}

	if title.CoverURL == nil && title.AniListID != nil {
		w.downloadAniListCover(ctx, title, logger)
	}

	return nil
}

func (w *TaskQueueWorker) handleCoverFetch(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload CoverFetchPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode cover_fetch payload: %w", err)
	}

	_ = logger

	coversDir := filepath.Join(w.dataDir, "covers")

	if w.tmdb != nil && payload.TMDBID != 0 {
		var posterPath *string
		if payload.TitleType == model.TitleTypeMovie {
			details, err := w.tmdb.GetMovieDetails(ctx, payload.TMDBID)
			if err != nil {
				return err
			}
			posterPath = details.PosterPath
		} else {
			details, err := w.tmdb.GetTVDetails(ctx, payload.TMDBID)
			if err != nil {
				return err
			}
			posterPath = details.PosterPath
		}

		if posterPath != nil && *posterPath != "" {
			coverPath, err := w.tmdb.DownloadCover(ctx, *posterPath, coversDir)
			if err != nil {
				return err
			}
			_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
				return repository.NewTitleWriter(tx).Update(ctx, payload.TitleID, repository.TitleUpdate{CoverURL: &coverPath})
			})
			if w.covers != nil {
				w.covers.ExtractAndStoreAccent(ctx, payload.TitleID, coverPath)
			}
			return nil
		}
	}

	if w.anilist != nil && payload.AniListID != 0 {
		details, err := w.anilist.GetAnimeDetails(ctx, payload.AniListID)
		if err != nil {
			return fmt.Errorf("anilist cover fetch: %w", err)
		}
		if details.CoverURL != "" {
			coverPath, err := w.anilist.DownloadCover(ctx, details.CoverURL, coversDir)
			if err != nil {
				return fmt.Errorf("download anilist cover: %w", err)
			}
			_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
				return repository.NewTitleWriter(tx).Update(ctx, payload.TitleID, repository.TitleUpdate{CoverURL: &coverPath})
			})
			if w.covers != nil {
				w.covers.ExtractAndStoreAccent(ctx, payload.TitleID, coverPath)
			}
		}
	}

	return nil
}

func (w *TaskQueueWorker) downloadAniListCover(ctx context.Context, title *model.Title, logger *slog.Logger) {
	if w.anilist == nil || title.AniListID == nil {
		return
	}

	details, err := w.anilist.GetAnimeDetails(ctx, *title.AniListID)
	if err != nil || details.CoverURL == "" {
		return
	}

	coverPath, err := w.anilist.DownloadCover(ctx, details.CoverURL, fmt.Sprintf("%s/covers", w.dataDir))
	if err != nil {
		return
	}

	_ = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath})
	})
	if w.covers != nil {
		w.covers.ExtractAndStoreAccent(ctx, title.ID, coverPath)
	}
	title.CoverURL = &coverPath
}

func (w *TaskQueueWorker) handleAniListPushSeason(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload AniListPushSeasonPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode anilist_push_season payload: %w", err)
	}
	if w.anilistPush == nil {
		return fmt.Errorf("anilist push service not configured")
	}

	_ = logger
	return w.anilistPush.PushSeasonState(ctx, payload.SeasonID)
}

func (w *TaskQueueWorker) handleAniListPushMovie(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload AniListPushMoviePayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode anilist_push_movie payload: %w", err)
	}
	if w.anilistPush == nil {
		return fmt.Errorf("anilist push service not configured")
	}
	_ = logger
	return w.anilistPush.PushMovieState(ctx, payload.TitleID)
}

func (w *TaskQueueWorker) handleGenerateWrapped(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload GenerateWrappedPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode generate_wrapped payload: %w", err)
	}
	if payload.Year <= 0 {
		return fmt.Errorf("invalid year %d for generate_wrapped", payload.Year)
	}

	logger = logger.With("year", payload.Year)
	logger.Info("generating wrapped retrospective snapshot")

	if w.statsRepo == nil || w.wrappedRepo == nil {
		return fmt.Errorf("statsRepo or wrappedRepo not configured on worker")
	}

	// 1. Gather stats data
	rawStats, data, err := w.statsRepo.GetWrappedData(ctx, payload.Year)
	if err != nil {
		return fmt.Errorf("get wrapped data for %d: %w", payload.Year, err)
	}

	// 2. Generate persona (Gemini or Fallback)
	var persona *model.WrappedAIPersona
	if w.pipeline != nil && w.pipeline.AI() != nil {
		var aiErr error
		persona, aiErr = w.pipeline.AI().GenerateWrappedStory(ctx, rawStats)
		if aiErr != nil {
			// If not max attempts, fail task so task queue retries with backoff
			if task.Attempts < 5 {
				return fmt.Errorf("gemini wrapped generation failed: %w", aiErr)
			}
			logger.Warn("gemini wrapped generation failed after max attempts, falling back to deterministic persona", "err", aiErr)
		}
	}
	if persona == nil {
		persona = matching.FallbackWrappedPersona(rawStats)
	}
	data.Persona = *persona

	// 3. Save snapshot in database
	if w.writeDB != nil {
		if err := database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
			return repository.NewWrappedWriter(tx).SaveSnapshot(ctx, payload.Year, data)
		}); err != nil {
			return fmt.Errorf("save wrapped snapshot for %d: %w", payload.Year, err)
		}
	}
	logger.Info("wrapped retrospective snapshot saved successfully", "year", payload.Year)

	// 4. Send Web Push notification if enabled
	if w.push != nil && IsNotificationEnabled(w.settings, NotifWrappedReady) {
		title := "Trackarr Wrapped"
		body := fmt.Sprintf("✨ Votre Trackarr Wrapped %d est prêt ! Découvrez votre rétrospective de l'année.", payload.Year)
		link := fmt.Sprintf("/wrapped?year=%d", payload.Year)
		if pushErr := w.push.SendNotification(ctx, title, body, link); pushErr != nil {
			logger.Warn("send wrapped push notification failed", "err", pushErr)
		}
	}

	return nil
}
