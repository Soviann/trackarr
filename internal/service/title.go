package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type TitleService struct {
	db       *sql.DB
	titles   *repository.TitleRepository
	tasks    *repository.TaskRepository
	pipeline *matching.Pipeline
}

func NewTitleService(db *sql.DB, titles *repository.TitleRepository, tasks *repository.TaskRepository, pipeline *matching.Pipeline) *TitleService {
	return &TitleService{db: db, titles: titles, tasks: tasks, pipeline: pipeline}
}

// CreateFromScrobble constructs a new title from scrobble metadata and starts matching.
// Caller owns the transaction so the create (or the "existing title found"
// update-in-place branch) shares atomicity with the surrounding webhook work.
func (s *TitleService) CreateFromScrobble(ctx context.Context, tx *sql.Tx, title string, year int, ids ExternalIDs, titleType model.TitleType, ratingKey string, guids []*url.URL, status model.TitleStatus) (int64, error) {
	titles := repository.NewTitleRepository(tx)
	writer := repository.NewTitleWriter(tx)
	t := &model.Title{
		Type:          titleType,
		Year:          year,
		PlexRatingKey: &ratingKey,
		Status:        status,
	}

	var names []model.TitleName

	if s.pipeline != nil {
		result, err := s.pipeline.Run(ctx, matching.MatchInput{
			Title:  title,
			Year:   year,
			Type:   titleType,
			IMDBID: ids.IMDB,
			TMDBID: ids.TMDB,
			TVDBID: ids.TVDB,
		})
		if err == nil {
			t.MatchStatus = result.MatchStatus
			t.MatchSource = &result.MatchSource
			t.OriginalTitle = &title
			t.Type = result.TitleType
			t.IsAnime = result.IsAnime
			if result.IMDBID != "" {
				t.IMDBID = &result.IMDBID
			}
			if result.TMDBID != 0 {
				t.TMDBID = &result.TMDBID
			}
			if result.TVDBID != 0 {
				t.TVDBID = &result.TVDBID
			}
			if result.AniListID != 0 {
				t.AniListID = &result.AniListID
			}
			if result.CoverFile != "" {
				coverURL := "/covers/" + result.CoverFile
				t.CoverURL = &coverURL
			}
			names = result.Names

			// Title already exists under these external IDs: update the Plex key
			// and return the existing ID so scrobbles converge on one record.
			existing, err := titles.FindByExternalID(t.IMDBID, t.TMDBID, nil, t.AniListID, &t.Type)
			if err == nil && existing != nil {
				update := repository.TitleUpdate{
					PlexRatingKey: t.PlexRatingKey,
					Type:          &t.Type,
					IsAnime:       &t.IsAnime,
				}
				if err := writer.Update(ctx, existing.ID, update); err != nil {
					return 0, fmt.Errorf("update existing title with plex key: %w", err)
				}
				return existing.ID, nil
			}
		}
	}

	if len(names) == 0 {
		// Fallback: no pipeline or pipeline error
		t.MatchStatus = model.MatchStatusConfirmed
		t.OriginalTitle = &title
		fallbackSource := matching.MatchSourcePlexIDs
		t.MatchSource = &fallbackSource
		if ids.IMDB != "" {
			t.IMDBID = &ids.IMDB
		}
		if ids.TMDB != 0 {
			t.TMDBID = &ids.TMDB
		}
		if ids.TVDB != 0 {
			t.TVDBID = &ids.TVDB
		}
		names = []model.TitleName{{Name: title, Language: "en", IsPrimary: true}}
	}

	return writer.Create(ctx, t, names)
}

// Rematch updates a title's external IDs and enqueues an enrichment task. The
// service owns the transaction because handlers call it with the pool handle.
func (s *TitleService) Rematch(ctx context.Context, db *sql.DB, id int64, imdbID *string, tmdbID *int64, anilistID *int64, tvdbID *int64) error {
	title, err := s.titles.GetByID(id)
	if err != nil {
		return err
	}

	matchStatus := model.MatchStatusConfirmed
	matchSource := matching.MatchSourceManual
	update := repository.TitleUpdate{
		MatchStatus: &matchStatus,
		MatchSource: &matchSource,
	}
	if tmdbID != nil {
		update.TMDBID = tmdbID
	}
	if imdbID != nil {
		update.IMDBID = imdbID
	}
	if anilistID != nil {
		update.AniListID = anilistID
	}
	if tvdbID != nil {
		update.TVDBID = tvdbID
	}

	if err := database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(ctx, id, update)
	}); err != nil {
		return err
	}

	// Enqueue enrichment task
	payloadTMDB := int64(0)
	if tmdbID != nil {
		payloadTMDB = *tmdbID
	} else if title.TMDBID != nil {
		payloadTMDB = *title.TMDBID
	}
	payloadIMDB := ""
	if imdbID != nil {
		payloadIMDB = *imdbID
	} else if title.IMDBID != nil {
		payloadIMDB = *title.IMDBID
	}
	payloadTVDB := int64(0)
	if tvdbID != nil {
		payloadTVDB = *tvdbID
	} else if title.TVDBID != nil {
		payloadTVDB = *title.TVDBID
	}

	payload, err := json.Marshal(EnrichmentPayload{
		TitleID:   id,
		TitleName: title.PrimaryName(),
		Year:      title.Year,
		TitleType: title.Type,
		IMDBID:    payloadIMDB,
		TMDBID:    payloadTMDB,
		TVDBID:    payloadTVDB,
	})
	if err != nil {
		return fmt.Errorf("marshal enrichment payload: %w", err)
	}
	dedupKey := fmt.Sprintf("enrichment:%d", id)
	return database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		_, err := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeEnrichment, string(payload), &dedupKey)
		return err
	})
}

// ExternalIDEdit is the authoritative snapshot from the manual ID editor. The
// editor always sends all four IDs, so each field carries the desired final
// value and a nil pointer means "clear this ID" (not "leave unchanged"). When
// AniListSeasonID is set, the AniList value routes to that season's mapping
// instead of the title row — for anime series the on-screen AniList link is
// driven by the season, so the title column would be invisible. AutoFill lets
// auto-matching back-fill the IDs left empty; without it, emptied IDs stay empty.
type ExternalIDEdit struct {
	TMDBID          *int64
	IMDBID          *string
	AniListID       *int64
	TVDBID          *int64
	AniListSeasonID *int64
	AutoFill        bool
}

// SetExternalIDs applies a manual external-ID snapshot. Unlike Rematch (which
// re-identifies from a chosen TMDB entry and re-derives everything), this treats
// the user's IDs as authoritative: it writes them exactly, marks the title
// manually matched, and enqueues a metadata refresh that locks the IDs the user
// touched so auto-matching never overwrites them. AutoFill controls whether the
// fields left empty get back-filled by that refresh.
func (s *TitleService) SetExternalIDs(ctx context.Context, db *sql.DB, id int64, edit ExternalIDEdit) error {
	title, err := s.titles.GetByID(id)
	if err != nil {
		return err
	}

	matchStatus := model.MatchStatusConfirmed
	matchSource := matching.MatchSourceManual
	update := repository.TitleUpdate{
		MatchStatus: &matchStatus,
		MatchSource: &matchSource,
	}
	// nil pointer = the user emptied the field → clear it to NULL.
	if edit.TMDBID != nil {
		update.TMDBID = edit.TMDBID
	} else {
		update.ClearTMDBID = true
	}
	if edit.IMDBID != nil {
		update.IMDBID = edit.IMDBID
	} else {
		update.ClearIMDBID = true
	}
	if edit.TVDBID != nil {
		update.TVDBID = edit.TVDBID
	} else {
		update.ClearTVDBID = true
	}
	// AniList always mirrors onto the title row (so the AniList-as-metadata
	// refresh, which keys off titles.anilist_id, can find it). For an anime
	// series we ALSO write the season mapping that drives the on-screen link.
	if edit.AniListID != nil {
		update.AniListID = edit.AniListID
	} else {
		update.ClearAniListID = true
	}
	routeAniListToSeason := edit.AniListSeasonID != nil

	// When both poster sources (TMDB, TVDB) are gone, reset the cover so a later
	// refresh re-derives it from AniList instead of keeping the stale one.
	if edit.TMDBID == nil && edit.TVDBID == nil {
		update.ClearCoverURL = true
	}

	if err := database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		if err := repository.NewTitleWriter(tx).Update(ctx, id, update); err != nil {
			return err
		}
		if routeAniListToSeason {
			seasonID := *edit.AniListSeasonID
			writer := repository.NewSeasonExternalIDWriter(tx)
			if edit.AniListID != nil {
				if err := writer.Add(ctx, seasonID, repository.ProviderAniList, strconv.FormatInt(*edit.AniListID, 10)); err != nil {
					return err
				}
				EnqueueAniListSeasonPush(ctx, tx, seasonID)
			} else if err := writer.Delete(ctx, seasonID, repository.ProviderAniList); err != nil {
				// Title-mode "blank AniList" clears all parts for that season.
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// Metadata refresh. Enqueue the enrichment pipeline when the user supplied a
	// strong anchor the pipeline can resolve from without a fuzzy name search: a
	// TMDB id, or an IMDb id (TMDB's /find/{imdb_id} resolves the rest, and
	// plexIDStrategy short-circuits before any name search). With neither there's
	// no reliable anchor — an AniList-only edit is handled by the caller's
	// AniList-sourced RefreshByID instead — so skip enrichment.
	if edit.TMDBID == nil && edit.IMDBID == nil {
		return nil
	}

	// AutoFill on → lock only the fields the user filled (blanks get back-filled);
	// off → lock all four (no auto ID writes, emptied IDs stay empty). AniList
	// is always locked at the title level — its value is user-set here.
	var locked []string
	lockIf := func(authoritative bool, key string) {
		if authoritative || !edit.AutoFill {
			locked = append(locked, key)
		}
	}
	lockIf(edit.TMDBID != nil, LockTMDB)
	lockIf(edit.IMDBID != nil, LockIMDB)
	lockIf(edit.TVDBID != nil, LockTVDB)
	lockIf(routeAniListToSeason || edit.AniListID != nil, LockAniList)

	payloadTMDB := int64(0)
	if edit.TMDBID != nil {
		payloadTMDB = *edit.TMDBID
	}
	payloadIMDB := ""
	if edit.IMDBID != nil {
		payloadIMDB = *edit.IMDBID
	}
	payloadTVDB := int64(0)
	if edit.TVDBID != nil {
		payloadTVDB = *edit.TVDBID
	}

	payload, err := json.Marshal(EnrichmentPayload{
		TitleID:       id,
		TitleName:     title.PrimaryName(),
		Year:          title.Year,
		TitleType:     title.Type,
		IsAnime:       title.IsAnime,
		IMDBID:        payloadIMDB,
		TMDBID:        payloadTMDB,
		TVDBID:        payloadTVDB,
		LockedIDs:     locked,
		PreserveMatch: true,
	})
	if err != nil {
		return fmt.Errorf("marshal enrichment payload: %w", err)
	}
	dedupKey := fmt.Sprintf("enrichment:%d", id)
	return database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		_, err := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeEnrichment, string(payload), &dedupKey)
		return err
	})
}

// ResolveURL identifies a title from an external URL.
func (s *TitleService) ResolveURL(ctx context.Context, url string) (*matching.MatchResult, error) {
	if s.pipeline == nil {
		return nil, fmt.Errorf("matching pipeline not available")
	}
	return s.pipeline.ResolveURL(ctx, url)
}

// Merge consolidates sourceID into destID. db must be the pool handle (*sql.DB),
// never a *sql.Tx — the method opens its own transaction via
// database.WithTxContext so a cancelled ctx aborts the write immediately.
func (s *TitleService) Merge(ctx context.Context, db *sql.DB, destID, sourceID int64, explicitOffset *int) error {
	source, err := s.titles.GetByID(sourceID)
	if err != nil {
		return err
	}
	dest, err := s.titles.GetByID(destID)
	if err != nil {
		return err
	}

	sourceName := source.PrimaryName()
	destName := dest.PrimaryName()

	seasonOffset := 0
	if explicitOffset != nil {
		seasonOffset = *explicitOffset
	} else if (source.IsAnime || dest.IsAnime) && s.pipeline != nil {
		if ident, err := s.pipeline.IdentifyAnimeSeason(ctx, sourceName, source.Year); err == nil && ident.IsSeason {
			log.Printf("fusion: Gemini identified sequel season %d for %q", ident.SeasonNumber, sourceName)
			seasonOffset = ident.SeasonNumber - 1
		} else if err != nil {
			log.Printf("fusion: Gemini season identification failed for %q: %v", sourceName, err)
		}
	}

	// Resolve AniList IDs for both dest and source independently.
	// destAniListID represents Season 1 (or existing dest seasons).
	// sourceAniListID represents the moved source season(s).
	var destAniListID, sourceAniListID int64
	if dest.AniListID != nil {
		destAniListID = *dest.AniListID
	} else if (source.IsAnime || dest.IsAnime) && s.pipeline != nil {
		if id, err := s.pipeline.SearchAniListByName(ctx, destName); err == nil && id != 0 {
			destAniListID = id
		}
	}

	if source.AniListID != nil {
		sourceAniListID = *source.AniListID
	} else if (source.IsAnime || dest.IsAnime) && s.pipeline != nil {
		if id, err := s.pipeline.SearchAniListByName(ctx, sourceName); err == nil && id != 0 {
			sourceAniListID = id
		}
	}

	return database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Merge(ctx, destID, sourceID, seasonOffset, destAniListID, sourceAniListID)
	})
}
