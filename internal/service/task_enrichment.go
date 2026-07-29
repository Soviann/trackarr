package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

func (w *TaskQueueWorker) handleEnrichment(ctx context.Context, task model.Task, logger *slog.Logger) error {
	var payload EnrichmentPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("decode enrichment payload: %w", err)
	}
	logger = logger.With("titleID", payload.TitleID)

	if w.pipeline == nil {
		return fmt.Errorf("pipeline not configured")
	}

	result, err := w.pipeline.Run(ctx, matching.MatchInput{
		Title:     payload.TitleName,
		Year:      payload.Year,
		Type:      payload.TitleType,
		IsAnime:   payload.IsAnime,
		IMDBID:    payload.IMDBID,
		TMDBID:    payload.TMDBID,
		TVDBID:    payload.TVDBID,
		AniListID: payload.AniListID,
	})
	if err != nil {
		return err
	}

	update := buildEnrichmentUpdate(result, payload)

	err = database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		titlesTx := repository.NewTitleWriter(tx)
		genresTx := repository.NewGenreWriter(tx)

		if err := titlesTx.Update(ctx, payload.TitleID, update); err != nil {
			return err
		}

		recalcWatchtime(ctx, tx, logger, payload.TitleID, result.Runtime)

		if len(result.Names) > 0 {
			if err := titlesTx.ReplaceNames(ctx, payload.TitleID, result.Names); err != nil {
				logger.Warn("replace title names", "err", err)
			}
		}

		if len(result.Genres) > 0 {
			if err := genresTx.ReplaceForTitle(ctx, payload.TitleID, result.Genres); err != nil {
				logger.Warn("save genres", "err", err)
			}
		}

		if !payload.PreserveMatch && result.MatchStatus == model.MatchStatusConfirmed && isSearchSource(result.MatchSource) {
			detail := fmt.Sprintf("%q → %q", payload.TitleName, resolvedName(result, payload))
			if err := repository.NewMatchEventWriter(tx).Create(ctx, payload.TitleID, model.MatchEventAutoConfirmed, detail); err != nil {
				logger.Warn("write auto-confirm event", "err", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	merged, err := w.resolveAnimeSeason(ctx, result, payload, logger)
	if err != nil {
		return err
	}

	if result.CoverFile != "" {
		w.covers.ExtractAndStoreAccent(ctx, payload.TitleID, result.CoverFile)
	}

	if !merged {
		w.enqueueSeasonBackfill(ctx, result, payload, logger)
	}

	return nil
}

// enqueueSeasonBackfill schedules a refresh for a freshly-matched series so its
// seasons/episodes get populated without waiting for the periodic refresh.
// Movies have no seasons, and seasons are sourced from TMDB, so both checks
// gate the enqueue. Idempotent via the refresh:<id> dedup key.
// enqueueSeasonBackfill schedules a refresh for a freshly-matched series so its
// seasons/episodes get populated without waiting for the periodic refresh.
// Movies have no seasons, and seasons are sourced from TMDB, so both checks
// gate the enqueue. Idempotent via the refresh:<id> dedup key.
func (w *TaskQueueWorker) enqueueSeasonBackfill(ctx context.Context, result *matching.MatchResult, payload EnrichmentPayload, logger *slog.Logger) {
	titleType := result.TitleType
	if titleType == "" {
		titleType = payload.TitleType
	}
	if titleType == model.TitleTypeMovie {
		return
	}
	tmdbID := result.TMDBID
	if tmdbID == 0 {
		tmdbID = payload.TMDBID
	}
	if tmdbID == 0 {
		return
	}

	data, err := json.Marshal(RefreshPayload{TitleID: payload.TitleID})
	if err != nil {
		logger.Warn("marshal season backfill payload", "err", err)
		return
	}
	dedupKey := fmt.Sprintf("refresh:%d", payload.TitleID)
	if err := database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		_, e := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeRefresh, string(data), &dedupKey)
		return e
	}); err != nil {
		logger.Warn("enqueue season backfill refresh", "err", err)
	}
}

// resolveAnimeConflict handles the IMDB-collision case where the pipeline
// identifies an anime that already exists under another local title. Returns
// merged=true when the source title has been consumed and the caller should
// stop processing further enrichment writes.
//
// explicitOffset is forwarded to Merge: nil falls back to Gemini name-parsing
// for the season offset (legacy behaviour); a non-nil pointer uses that offset
// directly (a chain root colliding on IMDb is the same show's season 1 → 0).
// resolveAnimeConflict handles the IMDB-collision case where the pipeline
// identifies an anime that already exists under another local title. Returns
// merged=true when the source title has been consumed and the caller should
// stop processing further enrichment writes.
//
// explicitOffset is forwarded to Merge: nil falls back to Gemini name-parsing
// for the season offset (legacy behaviour); a non-nil pointer uses that offset
// directly (a chain root colliding on IMDb is the same show's season 1 → 0).
func (w *TaskQueueWorker) resolveAnimeConflict(ctx context.Context, result *matching.MatchResult, payload EnrichmentPayload, explicitOffset *int, logger *slog.Logger) (bool, error) {
	if result.IMDBID == "" || !result.IsAnime {
		return false, nil
	}
	existing, err := w.titles.FindByExternalID(&result.IMDBID, nil, nil, nil, nil)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logger.Warn("FindByExternalID", "err", err)
		}
		return false, nil
	}
	if existing == nil || existing.ID == payload.TitleID || existing.Type == model.TitleTypeMovie {
		return false, nil
	}

	if existing.AniListID != nil && *existing.AniListID != 0 && *existing.AniListID != result.AniListID {
		logger.Info("skip IMDB-collision merge into distinct AniList sibling",
			"imdbID", result.IMDBID,
			"existingID", existing.ID,
			"existingAniList", *existing.AniListID,
			"selfAniList", result.AniListID,
		)
		return false, nil
	}
	logger.Info("IMDB conflict, merging anime",
		"imdbID", result.IMDBID,
		"intoTitleID", existing.ID,
		"existingType", existing.Type,
	)
	if err := w.titleSvc.Merge(ctx, w.writeDB, existing.ID, payload.TitleID, explicitOffset); err != nil {
		logger.Error("merge after IMDB conflict", "err", err)
		return false, nil
	}
	logger.Info("merged title after IMDB conflict", "intoTitleID", existing.ID)
	return true, nil
}

// resolveAnimeSeason auto-attaches an anime season to its parent series using
// AniList PREQUEL relations, with id-safety guards against merging distinct
// franchise members. Returns merged=true when the source title was consumed
// (so the caller skips season backfill). Non-anime / no-AniList-id titles, and
// any resolve/lookup failure, fall back to the legacy IMDb-collision path.
// resolveAnimeSeason auto-attaches an anime season to its parent series using
// AniList PREQUEL relations, with id-safety guards against merging distinct
// franchise members. Returns merged=true when the source title was consumed
// (so the caller skips season backfill). Non-anime / no-AniList-id titles, and
// any resolve/lookup failure, fall back to the legacy IMDb-collision path.
func (w *TaskQueueWorker) resolveAnimeSeason(ctx context.Context, result *matching.MatchResult, payload EnrichmentPayload, logger *slog.Logger) (bool, error) {

	if !result.IsAnime || result.AniListID == 0 {
		return w.resolveAnimeConflict(ctx, result, payload, nil, logger)
	}

	// Rule 2: resolve failure (incl. nil pipeline) → legacy behaviour.
	var chain *matching.SeasonChain
	if w.pipeline != nil {
		c, err := w.pipeline.ResolveAniListSeason(ctx, result.AniListID)
		if err != nil {
			logger.Warn("resolve AniList season chain", "anilistID", result.AniListID, "err", err)
		} else {
			chain = c
		}
	}

	// Parent lookups (only the meaningful ones once we have a season entry).
	var parentByIDs, parentByRoot *model.Title
	if chain != nil && !chain.IsRoot && chain.RootIsSeries {
		seriesType := model.TitleTypeSeries
		var imdbPtr *string
		if result.IMDBID != "" {
			imdbPtr = &result.IMDBID
		}
		var tmdbPtr *int64
		if result.TMDBID != 0 {
			tmdbPtr = &result.TMDBID
		}
		if imdbPtr != nil || tmdbPtr != nil {
			t, err := w.titles.FindByExternalID(imdbPtr, tmdbPtr, nil, nil, &seriesType)
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					logger.Warn("FindByExternalID (own ids)", "err", err)
				}
			} else if t != nil && t.ID != payload.TitleID {
				parentByIDs = t
			}
		}

		rootT, rootErr := w.titles.FindByExternalID(nil, nil, nil, &chain.RootID, &seriesType)
		if rootErr != nil {
			if !errors.Is(rootErr, sql.ErrNoRows) {
				logger.Warn("FindByExternalID (root anilist)", "err", rootErr)
			}
		} else if rootT != nil && rootT.ID != payload.TitleID {
			parentByRoot = rootT
		}
	}

	action := decideSeasonAction(chain, result, parentByIDs, parentByRoot)

	switch action.Kind {
	case seasonActionNone:
		if chain != nil {
			logger.Info("anime season left standalone",
				"anilistID", result.AniListID,
				"rootID", chain.RootID,
				"seasonNumber", chain.SeasonNumber,
			)
		}
		return false, nil
	case seasonActionLegacy:
		return w.resolveAnimeConflict(ctx, result, payload, nil, logger)
	case seasonActionLegacyRoot:
		offset := action.Offset
		return w.resolveAnimeConflict(ctx, result, payload, &offset, logger)
	case seasonActionMergeInto:
		return w.attachSeason(ctx, payload, chain, action.ParentID, action.Offset, "", logger)
	case seasonActionCreateRoot:
		return w.createParentAndAttach(ctx, result, payload, chain, action.Offset, logger)
	default:
		return false, nil
	}
}

// attachSeason merges the source title into parentID with the given offset and,
// on success, records a season_attached event on the parent. parentNameOverride
// is used when the parent was just created (its primary name isn't queryable
// via GetByID in the same flow); empty means fetch the parent's primary name.
// attachSeason merges the source title into parentID with the given offset and,
// on success, records a season_attached event on the parent. parentNameOverride
// is used when the parent was just created (its primary name isn't queryable
// via GetByID in the same flow); empty means fetch the parent's primary name.
func (w *TaskQueueWorker) attachSeason(ctx context.Context, payload EnrichmentPayload, chain *matching.SeasonChain, parentID int64, offset int, parentNameOverride string, logger *slog.Logger) (bool, error) {

	sourceName := ""
	if src, err := w.titles.GetByID(payload.TitleID); err == nil {
		sourceName = src.PrimaryName()
	} else {
		logger.Warn("fetch source title before merge", "err", err)
	}

	parentName := parentNameOverride
	if parentName == "" {
		if parent, err := w.titles.GetByID(parentID); err == nil {
			parentName = parent.PrimaryName()
		} else {
			logger.Warn("fetch parent title name", "err", err)
		}
	}

	logger.Info("attaching anime season",
		"anilistID", payload.AniListID,
		"intoTitleID", parentID,
		"seasonNumber", chain.SeasonNumber,
		"offset", offset,
	)
	off := offset
	if err := w.titleSvc.Merge(ctx, w.writeDB, parentID, payload.TitleID, &off); err != nil {
		logger.Error("merge season into parent", "err", err)
		return false, nil
	}

	detail := fmt.Sprintf("%q attached as Season %d of %q", sourceName, chain.SeasonNumber, parentName)
	if err := database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		return repository.NewMatchEventWriter(tx).Create(ctx, parentID, model.MatchEventSeasonAttached, detail)
	}); err != nil {
		logger.Warn("write season_attached event", "err", err)
	}
	return true, nil
}

// createParentAndAttach creates a new root series title for the chain, enqueues
// its enrichment, then merges the source season into it. The source's Year is
// copied so the new parent isn't year-zero when AniList enrichment is pending.
// createParentAndAttach creates a new root series title for the chain, enqueues
// its enrichment, then merges the source season into it. The source's Year is
// copied so the new parent isn't year-zero when AniList enrichment is pending.
func (w *TaskQueueWorker) createParentAndAttach(ctx context.Context, result *matching.MatchResult, payload EnrichmentPayload, chain *matching.SeasonChain, offset int, logger *slog.Logger) (bool, error) {
	source, err := w.titles.GetByID(payload.TitleID)
	if err != nil {
		logger.Warn("fetch source title for parent creation", "err", err)
		return false, nil
	}

	rootAniList := chain.RootID
	confirmed := model.MatchStatusConfirmed
	matchSource := matching.MatchSourceAniListSearch
	parent := &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        source.Year,
		Status:      source.Status,
		MatchStatus: confirmed,
		MatchSource: &matchSource,
		AniListID:   &rootAniList,
	}
	names := []model.TitleName{{Name: chain.RootTitle, Language: "en", IsPrimary: true}}

	var newID int64
	if err := database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
		id, e := repository.NewTitleWriter(tx).Create(ctx, parent, names)
		if e != nil {
			return e
		}
		newID = id
		return nil
	}); err != nil {
		logger.Error("create root parent title", "err", err)
		return false, nil
	}

	enrichPayload := EnrichmentPayload{
		TitleID:   newID,
		TitleName: chain.RootTitle,
		TitleType: model.TitleTypeSeries,
		IsAnime:   true,
		AniListID: rootAniList,
	}
	if data, e := json.Marshal(enrichPayload); e != nil {
		logger.Warn("marshal parent enrichment payload", "err", e)
	} else {
		dedupKey := fmt.Sprintf("enrichment:%d", newID)
		if err := database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error {
			_, e := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeEnrichment, string(data), &dedupKey)
			return e
		}); err != nil {
			logger.Warn("enqueue parent enrichment", "err", err)
		}
	}

	logger.Info("created root parent for anime season",
		"anilistID", result.AniListID,
		"rootID", chain.RootID,
		"newParentID", newID,
	)
	return w.attachSeason(ctx, payload, chain, newID, offset, chain.RootTitle, logger)
}
