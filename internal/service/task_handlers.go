package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
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
