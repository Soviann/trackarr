package service

import (
	"context"
	"database/sql"
	"log"
	"strconv"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

// refreshFromAniList sources a title's metadata (names, synopsis, genres,
// rating, runtime, cover) from AniList. Used for titles that have an AniList ID
// but no TMDB — niche anime absent from TMDB. AniList is then the authority, so
// it OVERWRITES existing values (which are often stale, left over from a removed
// wrong TMDB match). Best-effort: each piece is logged and skipped on failure.
// refreshFromAniList sources a title's metadata (names, synopsis, genres,
// rating, runtime, cover) from AniList. Used for titles that have an AniList ID
// but no TMDB — niche anime absent from TMDB. AniList is then the authority, so
// it OVERWRITES existing values (which are often stale, left over from a removed
// wrong TMDB match). Best-effort: each piece is logged and skipped on failure.
func (s *BackgroundService) refreshFromAniList(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	if s.anilist == nil || title.AniListID == nil {
		return
	}
	details, err := s.anilist.GetAnimeDetails(ctx, *title.AniListID)
	if err != nil {
		result.Error = err
		return
	}
	result.Refreshed = true

	primary := details.EnglishTitle
	if primary == "" {
		primary = details.RomajiTitle
	}
	var names []model.TitleName
	if primary != "" {
		names = append(names, model.TitleName{Name: primary, Language: "en", IsPrimary: true})
	}
	if details.RomajiTitle != "" && details.RomajiTitle != primary {
		names = append(names, model.TitleName{Name: details.RomajiTitle, Language: "x-romaji"})
	}
	if len(names) > 0 {
		if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
			return repository.NewTitleWriter(tx).ReplaceNames(ctx, title.ID, names)
		}); err != nil {
			log.Printf("background: anilist names for title %d: %v", title.ID, err)
		} else {
			title.PrimaryName = primary
			result.TitleName = primary
		}
	}

	update := repository.TitleUpdate{}
	if details.Description != "" {
		update.Overview = &details.Description
	}
	if details.AverageScore != nil {
		update.AniListRating = details.AverageScore
	}
	if details.Duration != nil {
		update.Runtime = details.Duration
	}
	if details.CountryOfOrigin != nil {
		update.OriginCountry = details.CountryOfOrigin
	}
	logTitleUpdate(title.ID, "anilist metadata", s.updateTitle(ctx, title.ID, update))

	if len(details.Genres) > 0 {
		if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
			return repository.NewGenreWriter(tx).ReplaceForTitle(ctx, title.ID, details.Genres)
		}); err != nil {
			log.Printf("background: anilist genres for title %d: %v", title.ID, err)
		}
	}

	if title.CoverURL == nil {
		s.covers.DownloadAniListCover(ctx, title)
	}
}

// refreshAniListSeasonScores walks every AniList part of every season of the
// title and stores the current score, episode count, and start date on each
// season_external_ids row (via ListPartsForTitle → UpdatePartMeta).
//
// Uses AniList's public GraphQL endpoint (no auth) — token-invalid handling
// is unnecessary on the call itself. The early-return on the
// anilist_token_invalid flag still applies: when the user's authenticated
// connection is broken (flagged by the push-sync path), we pause unrelated
// AniList traffic so the admin reconnect banner is the loudest signal until
// the user acts on it. Errors are logged per mapping; one bad season cannot
// break the others.
// refreshAniListSeasonScores walks every AniList part of every season of the
// title and stores the current score, episode count, and start date on each
// season_external_ids row (via ListPartsForTitle → UpdatePartMeta).
//
// Uses AniList's public GraphQL endpoint (no auth) — token-invalid handling
// is unnecessary on the call itself. The early-return on the
// anilist_token_invalid flag still applies: when the user's authenticated
// connection is broken (flagged by the push-sync path), we pause unrelated
// AniList traffic so the admin reconnect banner is the loudest signal until
// the user acts on it. Errors are logged per mapping; one bad season cannot
// break the others.
func (s *BackgroundService) refreshAniListSeasonScores(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	if invalid, _ := s.settings.Get(repository.SettingKeyAniListTokenInvalid); invalid == "true" {
		return
	}

	partsBySeason, err := s.seasonExtIDs.ListPartsForTitle(ctx, title.ID, repository.ProviderAniList)
	if err != nil {
		log.Printf("background anilist score: list parts for title %d: %v", title.ID, err)
		return
	}
	if len(partsBySeason) == 0 {
		return
	}

	for seasonID, parts := range partsBySeason {
		for _, part := range parts {
			if err := ctx.Err(); err != nil {
				return
			}
			anilistID, err := strconv.ParseInt(part.ExternalID, 10, 64)
			if err != nil {
				log.Printf("background anilist score: invalid mapping %q for season %d: %v", part.ExternalID, seasonID, err)
				continue
			}
			details, err := s.anilist.GetAnimeDetails(ctx, anilistID)
			if err != nil {
				log.Printf("background anilist score: fetch %d: %v", anilistID, err)
				_ = s.limiter.Wait(ctx)
				continue
			}
			result.Refreshed = true
			if err := s.seasonExtIDs.UpdatePartMeta(
				ctx, seasonID, repository.ProviderAniList, part.ExternalID,
				details.AverageScore, details.Episodes, details.StartDate); err != nil {
				log.Printf("background anilist score: persist season %d part %s: %v", seasonID, part.ExternalID, err)
			}
			_ = s.limiter.Wait(ctx)
		}
	}
}

func (s *BackgroundService) backfillAniListID(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	if s.anilist == nil || title.PrimaryName == "" || !title.IsAnime {
		return
	}
	results, err := s.anilist.SearchAnime(ctx, title.PrimaryName)
	if err != nil || len(results) == 0 {
		return
	}
	anilistID := results[0].ID
	result.Refreshed = true
	logTitleUpdate(title.ID, "anilist_id backfill", s.updateTitle(ctx, title.ID, repository.TitleUpdate{AniListID: &anilistID}))
	title.AniListID = &anilistID

	partsBySeason, err := s.seasonExtIDs.ListPartsForTitle(ctx, title.ID, repository.ProviderAniList)
	if err == nil && len(partsBySeason) == 0 {
		var seasonID int64
		err := s.writeDB.QueryRowContext(ctx, `SELECT id FROM seasons WHERE title_id = ? AND season_number = 1`, title.ID).Scan(&seasonID)
		if err == nil {
			_ = s.seasonExtIDs.Add(ctx, seasonID, repository.ProviderAniList, strconv.FormatInt(anilistID, 10))
		}
	}
}

// refreshAniListRelations walks every mapped season part and queries AniList relations
// to collect side stories, movies, OVAs, and spin-offs, saving them to title_relations.
func (s *BackgroundService) refreshAniListRelations(ctx context.Context, title *repository.TitleLite, result *RefreshResult) {
	if s.anilist == nil || !title.IsAnime {
		return
	}
	if invalid, _ := s.settings.Get(repository.SettingKeyAniListTokenInvalid); invalid == "true" {
		return
	}

	partsBySeason, err := s.seasonExtIDs.ListPartsForTitle(ctx, title.ID, repository.ProviderAniList)
	if err != nil {
		log.Printf("background anilist relations: list parts for title %d: %v", title.ID, err)
		return
	}

	var relations []model.TitleRelation
	seenExternal := make(map[int64]bool)

	// Collect relations for each season
	for seasonID, parts := range partsBySeason {
		for _, part := range parts {
			if err := ctx.Err(); err != nil {
				return
			}
			anilistID, err := strconv.ParseInt(part.ExternalID, 10, 64)
			if err != nil {
				continue
			}

			nodes, err := s.anilist.GetFranchiseRelations(ctx, anilistID)
			if err != nil {
				log.Printf("background anilist relations: fetch %d: %v", anilistID, err)
				_ = s.limiter.Wait(ctx)
				continue
			}
			result.Refreshed = true

			sid := seasonID
			for _, node := range nodes {
				if seenExternal[node.ID] {
					continue
				}
				seenExternal[node.ID] = true

				var romajiPtr *string
				if node.RomajiTitle != "" && node.RomajiTitle != node.Title {
					romajiPtr = &node.RomajiTitle
				}
				var coverPtr *string
				if node.CoverURL != "" {
					coverPtr = &node.CoverURL
				}

				relations = append(relations, model.TitleRelation{
					TitleID:      title.ID,
					SeasonID:     &sid,
					Provider:     repository.ProviderAniList,
					ExternalID:   node.ID,
					RelationType: model.RelationType(node.RelationType),
					Format:       node.Format,
					Title:        node.Title,
					RomajiTitle:  romajiPtr,
					CoverURL:     coverPtr,
					Year:         node.Year,
					Score:        node.Score,
					EpisodeCount: node.EpisodeCount,
					Duration:     node.Duration,
					Overview:     node.Overview,
					SortOrder:    len(relations) + 1,
				})
			}
			_ = s.limiter.Wait(ctx)
		}
	}

	// Also check root title AniListID if present and not yet covered
	if title.AniListID != nil && !seenExternal[*title.AniListID] {
		nodes, err := s.anilist.GetFranchiseRelations(ctx, *title.AniListID)
		if err == nil {
			result.Refreshed = true
			for _, node := range nodes {
				if seenExternal[node.ID] {
					continue
				}
				seenExternal[node.ID] = true

				var romajiPtr *string
				if node.RomajiTitle != "" && node.RomajiTitle != node.Title {
					romajiPtr = &node.RomajiTitle
				}
				var coverPtr *string
				if node.CoverURL != "" {
					coverPtr = &node.CoverURL
				}

				relations = append(relations, model.TitleRelation{
					TitleID:      title.ID,
					SeasonID:     nil,
					Provider:     repository.ProviderAniList,
					ExternalID:   node.ID,
					RelationType: model.RelationType(node.RelationType),
					Format:       node.Format,
					Title:        node.Title,
					RomajiTitle:  romajiPtr,
					CoverURL:     coverPtr,
					Year:         node.Year,
					Score:        node.Score,
					EpisodeCount: node.EpisodeCount,
					Duration:     node.Duration,
					Overview:     node.Overview,
					SortOrder:    len(relations) + 1,
				})
			}
		}
	}

	if len(relations) > 0 {
		if err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
			return repository.NewTitleRelationWriter(tx).UpsertBatch(ctx, title.ID, relations)
		}); err != nil {
			log.Printf("background anilist relations: persist for title %d: %v", title.ID, err)
		}
	}
}
