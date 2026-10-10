package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
)

type TitleService struct {
	db       *sql.DB
	titles   *repository.TitleRepository
	tasks    *repository.TaskRepository
	pipeline *matching.Pipeline
	arrSvc   *ArrService
}

func NewTitleService(db *sql.DB, titles *repository.TitleRepository, tasks *repository.TaskRepository, pipeline *matching.Pipeline) *TitleService {
	return &TitleService{db: db, titles: titles, tasks: tasks, pipeline: pipeline}
}

func (s *TitleService) SetArrService(arrSvc *ArrService) {
	s.arrSvc = arrSvc
}

// CreateFromScrobble constructs a new title from scrobble metadata and starts matching.
// Caller owns the transaction so the create (or the "existing title found"
// update-in-place branch) shares atomicity with the surrounding webhook work.
func (s *TitleService) CreateFromScrobble(ctx context.Context, tx *sql.Tx, title string, year int, ids ExternalIDs, titleType model.TitleType, ratingKey string, guids []*url.URL, status model.TitleStatus) (int64, error) {
	titles := repository.NewTitleRepository(tx)
	writer := repository.NewTitleWriter(tx)

	// Step 0: Check if an existing title in our database matches by external IDs
	// or exact name and type before making online external API/LLM calls.
	var existingByLocal *model.Title
	if ids.IMDB != "" || ids.TMDB != 0 || ids.TVDB != 0 {
		var imdbPtr *string
		var tmdbPtr *int64
		var tvdbPtr *int64
		if ids.IMDB != "" {
			imdbPtr = &ids.IMDB
		}
		if ids.TMDB != 0 {
			tmdbPtr = &ids.TMDB
		}
		if ids.TVDB != 0 {
			tvdbPtr = &ids.TVDB
		}
		if found, err := titles.FindByExternalID(imdbPtr, tmdbPtr, nil, nil, tvdbPtr, &titleType); err == nil && found != nil {
			existingByLocal = found
		}
	}

	if existingByLocal == nil && title != "" {
		if found, err := titles.FindByNameAndType(title, titleType, year); err == nil && found != nil {
			existingByLocal = found
		}
	}

	if existingByLocal != nil {
		update := repository.TitleUpdate{
			ExternalSourceID: &ratingKey,
		}
		if err := writer.Update(ctx, existingByLocal.ID, update); err != nil {
			return 0, fmt.Errorf("update existing title with rating key: %w", err)
		}
		return existingByLocal.ID, nil
	}

	t := &model.Title{
		Type:             titleType,
		Year:             year,
		ExternalSourceID: &ratingKey,
		Status:           status,
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

			// Title already exists under these external IDs: update the external source key
			// and return the existing ID so scrobbles converge on one record.
			existing, err := titles.FindByExternalID(t.IMDBID, t.TMDBID, nil, t.AniListID, t.TVDBID, &t.Type)
			if err == nil && existing != nil {
				update := repository.TitleUpdate{
					ExternalSourceID: t.ExternalSourceID,
					Type:             &t.Type,
					IsAnime:          &t.IsAnime,
				}
				if err := writer.Update(ctx, existing.ID, update); err != nil {
					return 0, fmt.Errorf("update existing title with external source key: %w", err)
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

// Rematch updates a title's external IDs (and optionally title type) and enqueues
// an enrichment task. The service owns the transaction because handlers call it with the pool handle.
func (s *TitleService) Rematch(ctx context.Context, db *sql.DB, id int64, imdbID *string, tmdbID *int64, anilistID *int64, tvdbID *int64, titleType *model.TitleType, deleteFromArr bool) error {
	title, err := s.titles.GetByID(id)
	if err != nil {
		return err
	}

	matchStatus := model.MatchStatusConfirmed
	matchSource := matching.MatchSourceManual
	update := repository.TitleUpdate{
		MatchStatus:   &matchStatus,
		MatchSource:   &matchSource,
		ClearCoverURL: true,
	}
	if tmdbID != nil {
		update.TMDBID = tmdbID
		// When rematching via TMDB without an explicit TVDB ID, clear the old TVDB ID
		// so stale cross-references (or previous mismatches) do not taint the new match.
		if tvdbID == nil {
			update.ClearTVDBID = true
		}
		if imdbID == nil {
			update.ClearIMDBID = true
		}
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
	if titleType != nil && *titleType != "" {
		update.Type = titleType
	}

	// Arr synchronization:
	// If the title was linked to Radarr or Sonarr, a Rematch means its identity changed.
	// We unlink it from Trackarr, and optionally delete the old entry from Arr.
	hasArrLink := (title.RadarrID != nil && *title.RadarrID > 0) || (title.SonarrID != nil && *title.SonarrID > 0)
	if hasArrLink {
		update.ClearRadarrID = true
		update.ClearSonarrID = true
		update.ClearSonarrDeletedAt = true
		falseVal := false
		update.ArrIgnored = &falseVal
	}

	if err := database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(ctx, id, update)
	}); err != nil {
		return err
	}

	// Post-commit side effects: delete old entry from Sonarr/Radarr if requested
	if hasArrLink && deleteFromArr && s.arrSvc != nil {
		_ = s.arrSvc.DeleteTitleFromArr(ctx, title, true)
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
	} else if tmdbID == nil && title.IMDBID != nil {
		payloadIMDB = *title.IMDBID
	}
	payloadTVDB := int64(0)
	if tvdbID != nil {
		payloadTVDB = *tvdbID
	} else if tmdbID == nil && title.TVDBID != nil {
		payloadTVDB = *title.TVDBID
	}
	payloadType := title.Type
	if titleType != nil && *titleType != "" {
		payloadType = *titleType
	}

	payload, err := json.Marshal(EnrichmentPayload{
		TitleID:       id,
		TitleName:     title.PrimaryName(),
		Year:          title.Year,
		TitleType:     payloadType,
		IMDBID:        payloadIMDB,
		TMDBID:        payloadTMDB,
		TVDBID:        payloadTVDB,
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
	DeleteFromArr   bool
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
		isAnime := true
		update.IsAnime = &isAnime
	} else {
		update.ClearAniListID = true
	}
	routeAniListToSeason := edit.AniListSeasonID != nil
	var seasonID int64
	if edit.AniListSeasonID != nil {
		seasonID = *edit.AniListSeasonID
	} else if title.Type != model.TitleTypeMovie && edit.AniListID != nil {
		for _, s := range title.Seasons {
			if s.SeasonNumber == 1 {
				seasonID = s.ID
				routeAniListToSeason = true
				break
			}
		}
		if !routeAniListToSeason && len(title.Seasons) > 0 {
			seasonID = title.Seasons[0].ID
			routeAniListToSeason = true
		}
		if !routeAniListToSeason {
			routeAniListToSeason = true
			seasonID = 0
		}
	}

	// Arr synchronization:
	// If TMDB or TVDB ID changed and title had an Arr link, unlink it and optionally delete from Arr.
	tmdbChanged := (edit.TMDBID == nil && title.TMDBID != nil) || (edit.TMDBID != nil && (title.TMDBID == nil || *edit.TMDBID != *title.TMDBID))
	tvdbChanged := (edit.TVDBID == nil && title.TVDBID != nil) || (edit.TVDBID != nil && (title.TVDBID == nil || *edit.TVDBID != *title.TVDBID))
	hasArrLink := (title.RadarrID != nil && *title.RadarrID > 0) || (title.SonarrID != nil && *title.SonarrID > 0)
	if (tmdbChanged || tvdbChanged) && hasArrLink {
		update.ClearRadarrID = true
		update.ClearSonarrID = true
		update.ClearSonarrDeletedAt = true
		falseVal := false
		update.ArrIgnored = &falseVal
	}

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
			if seasonID == 0 {
				season, err := repository.NewSeasonWriter(tx).GetOrCreate(ctx, id, 1)
				if err != nil {
					return err
				}
				seasonID = season.ID
			}
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

	// Post-commit side effects: delete old entry from Sonarr/Radarr if requested
	if (tmdbChanged || tvdbChanged) && hasArrLink && edit.DeleteFromArr && s.arrSvc != nil {
		_ = s.arrSvc.DeleteTitleFromArr(ctx, title, true)
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

// Create constructs a title and its associated names in a managed transaction.
func (s *TitleService) Create(ctx context.Context, title *model.Title, names []model.TitleName) (int64, error) {
	var id int64
	err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		newID, err := repository.NewTitleWriter(tx).Create(ctx, title, names)
		if err != nil {
			return err
		}
		id = newID
		return nil
	})
	return id, err
}

// CreateAndEnrich constructs a title, its names, and optionally enqueues an enrichment task.
func (s *TitleService) CreateAndEnrich(ctx context.Context, title *model.Title, names []model.TitleName, enqueueEnrichment bool) (int64, error) {
	var newID int64
	err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		id, err := repository.NewTitleWriter(tx).Create(ctx, title, names)
		if err != nil {
			return err
		}
		newID = id

		if enqueueEnrichment && s.tasks != nil {
			primaryName := ""
			for _, n := range names {
				if n.IsPrimary {
					primaryName = n.Name
					break
				}
			}
			if primaryName == "" && len(names) > 0 {
				primaryName = names[0].Name
			}
			var tmdbID int64
			if title.TMDBID != nil {
				tmdbID = *title.TMDBID
			}
			var imdbID string
			if title.IMDBID != nil {
				imdbID = *title.IMDBID
			}
			var tvdbID int64
			if title.TVDBID != nil {
				tvdbID = *title.TVDBID
			}
			var anilistID int64
			if title.AniListID != nil {
				anilistID = *title.AniListID
			}

			payload := EnrichmentPayload{
				TitleID:       newID,
				TitleName:     primaryName,
				Year:          title.Year,
				TitleType:     title.Type,
				IsAnime:       title.IsAnime,
				IMDBID:        imdbID,
				TMDBID:        tmdbID,
				TVDBID:        tvdbID,
				AniListID:     anilistID,
				PreserveMatch: title.MatchStatus == model.MatchStatusConfirmed,
			}
			payloadJSON, _ := json.Marshal(payload)
			dedupKey := fmt.Sprintf("enrichment:%d", newID)
			_, _ = repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeEnrichment, string(payloadJSON), &dedupKey)
		}
		return nil
	})
	return newID, err
}

// Update modifies a title and enqueues any resulting AniList pushes or Sonarr deletions inside a managed transaction.
func (s *TitleService) Update(ctx context.Context, id int64, update repository.TitleUpdate, before *model.Title, newStatus *model.TitleStatus, newRating *int, deleteFromSonarr bool) error {
	return database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		if err := repository.NewTitleWriter(tx).Update(ctx, id, update); err != nil {
			return err
		}
		if before != nil {
			enqueueAniListPushesOnTitleUpdate(ctx, tx, before, newStatus, newRating)
			if deleteFromSonarr {
				enqueueSonarrDeleteOnTitleUpdate(ctx, tx, before)
			}
		}
		return nil
	})
}

func enqueueSonarrDeleteOnTitleUpdate(ctx context.Context, tx *sql.Tx, before *model.Title) {
	payload := SonarrDeletePayload{
		TitleID:                before.ID,
		SonarrID:               before.SonarrID,
		TVDBID:                 before.TVDBID,
		DeleteFiles:            true,
		AddImportListExclusion: true,
	}
	b, _ := json.Marshal(payload)
	dedup := fmt.Sprintf("sonarr_delete_%d", before.ID)
	_, _ = repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeSonarrDelete, string(b), &dedup)
}

// Delete removes a title by ID inside a managed transaction.
func (s *TitleService) Delete(ctx context.Context, id int64) error {
	return database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Delete(ctx, id)
	})
}

// BatchDelete removes multiple titles by ID inside a managed transaction.
func (s *TitleService) BatchDelete(ctx context.Context, ids []int64) error {
	return database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).BatchDelete(ctx, ids)
	})
}

// BatchUpdateStatus updates the status of multiple titles inside a managed transaction.
func (s *TitleService) BatchUpdateStatus(ctx context.Context, ids []int64, status string) error {
	return database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).BatchUpdateStatus(ctx, ids, status)
	})
}

type BatchCreateItem struct {
	Title     string            `json:"title"`
	Type      model.TitleType   `json:"type,omitempty"`
	Year      int               `json:"year,omitempty"`
	URL       string            `json:"url,omitempty"`
	IMDBID    *string           `json:"imdb_id,omitempty"`
	TMDBID    *int64            `json:"tmdb_id,omitempty"`
	TVDBID    *int64            `json:"tvdb_id,omitempty"`
	AniListID *int64            `json:"anilist_id,omitempty"`
	IsAnime   bool              `json:"is_anime,omitempty"`
	Status    model.TitleStatus `json:"status,omitempty"`
}

type BatchCreateResultItem struct {
	Index   int    `json:"index"`
	TitleID *int64 `json:"title_id,omitempty"`
	Title   string `json:"title,omitempty"`
	Status  string `json:"status"` // "created", "existing", "error"
	Error   string `json:"error,omitempty"`
}

type BatchCreateResult struct {
	Total    int                     `json:"total"`
	Created  int                     `json:"created"`
	Existing int                     `json:"existing"`
	Failed   int                     `json:"failed"`
	Items    []BatchCreateResultItem `json:"items"`
}

func isPrivateOrLocalURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
	}
	return false
}

// BatchCreate inserts multiple titles in a single atomic transaction, deduplicating
// against existing titles and enqueuing background enrichment tasks.
func (s *TitleService) BatchCreate(ctx context.Context, items []BatchCreateItem) (*BatchCreateResult, error) {
	if len(items) == 0 {
		return nil, errors.New("batch must contain between 1 and 100 items")
	}
	if len(items) > 100 {
		return nil, errors.New("batch size exceeds maximum limit of 100 items")
	}

	res := &BatchCreateResult{
		Total: len(items),
		Items: make([]BatchCreateResultItem, len(items)),
	}

	err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		titles := repository.NewTitleRepository(tx)
		titleWriter := repository.NewTitleWriter(tx)
		taskWriter := repository.NewTaskWriter(tx)

		for i, item := range items {
			itemRes := BatchCreateResultItem{
				Index: i,
			}

			// Boundary checks
			if len(item.Title) > 500 {
				itemRes.Status = "error"
				itemRes.Error = "title exceeds maximum length of 500 characters"
				res.Failed++
				res.Items[i] = itemRes
				continue
			}
			if len(item.URL) > 2048 {
				itemRes.Status = "error"
				itemRes.Error = "url exceeds maximum length of 2048 characters"
				res.Failed++
				res.Items[i] = itemRes
				continue
			}

			// Synchronous URL parsing with SSRF protection
			if item.URL != "" {
				if isPrivateOrLocalURL(item.URL) {
					itemRes.Status = "error"
					itemRes.Error = "url points to a private or disallowed address"
					res.Failed++
					res.Items[i] = itemRes
					continue
				}
				parsed := matching.ParseURLFull(item.URL)
				if parsed != nil {
					if parsed.IMDB != "" && item.IMDBID == nil {
						item.IMDBID = &parsed.IMDB
					}
					if parsed.TMDBMovie != 0 && item.TMDBID == nil {
						item.TMDBID = &parsed.TMDBMovie
						if item.Type == "" {
							item.Type = model.TitleTypeMovie
						}
					}
					if parsed.TMDBTV != 0 && item.TMDBID == nil {
						item.TMDBID = &parsed.TMDBTV
						if item.Type == "" {
							item.Type = model.TitleTypeSeries
						}
					}
					if parsed.AniList != 0 && item.AniListID == nil {
						item.AniListID = &parsed.AniList
						item.IsAnime = true
					}
					if parsed.TVDB != 0 && item.TVDBID == nil {
						item.TVDBID = &parsed.TVDB
					}
					if parsed.TVDBSeriesSlug != "" && item.Type == "" {
						item.Type = model.TitleTypeSeries
					}
					if parsed.TVDBMovieSlug != "" && item.Type == "" {
						item.Type = model.TitleTypeMovie
					}
					if item.Title == "" {
						if parsed.TVDBSeriesSlug != "" {
							item.Title = parsed.TVDBSeriesSlug
						} else if parsed.TVDBMovieSlug != "" {
							item.Title = parsed.TVDBMovieSlug
						}
					}
				}
			}

			// Fallback title name if empty but ID present
			if item.Title == "" {
				switch {
				case item.IMDBID != nil && *item.IMDBID != "":
					item.Title = *item.IMDBID
				case item.TMDBID != nil && *item.TMDBID != 0:
					item.Title = fmt.Sprintf("TMDB %d", *item.TMDBID)
				case item.AniListID != nil && *item.AniListID != 0:
					item.Title = fmt.Sprintf("AniList %d", *item.AniListID)
				case item.TVDBID != nil && *item.TVDBID != 0:
					item.Title = fmt.Sprintf("TVDB %d", *item.TVDBID)
				}
			}

			if item.Title == "" {
				itemRes.Status = "error"
				itemRes.Error = "title or valid external identifier is required"
				res.Failed++
				res.Items[i] = itemRes
				continue
			}

			itemRes.Title = item.Title

			// Defaults
			if item.Type == "" {
				item.Type = model.TitleTypeMovie
			}
			if item.Status == "" {
				item.Status = model.TitleStatusPlanToWatch
			}
			if item.Year < 0 || item.Year > 2100 {
				item.Year = 0
			}

			// Check if title already exists in DB
			var typePtr *model.TitleType
			if item.Type != "" {
				typePtr = &item.Type
			}
			existing, err := titles.FindByExternalID(item.IMDBID, item.TMDBID, nil, item.AniListID, item.TVDBID, typePtr)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("lookup external id: %w", err)
			}
			if existing == nil && item.Title != "" {
				existing, err = titles.FindByNameAndType(item.Title, item.Type, item.Year)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("lookup name and type: %w", err)
				}
			}

			if existing != nil {
				res.Existing++
				itemRes.Status = "existing"
				itemRes.TitleID = &existing.ID
				if pName := existing.PrimaryName(); pName != "" {
					itemRes.Title = pName
				}
				res.Items[i] = itemRes
				continue
			}

			// Determine match status and source
			matchStatus := model.MatchStatusPendingReview
			if item.TMDBID != nil && *item.TMDBID > 0 {
				matchStatus = model.MatchStatusConfirmed
			} else if item.AniListID != nil && *item.AniListID > 0 {
				matchStatus = model.MatchStatusConfirmed
			}
			source := "batch"

			title := &model.Title{
				Type:        item.Type,
				IsAnime:     item.IsAnime,
				Year:        item.Year,
				Status:      item.Status,
				MatchStatus: matchStatus,
				MatchSource: &source,
				IMDBID:      item.IMDBID,
				TMDBID:      item.TMDBID,
				TVDBID:      item.TVDBID,
				AniListID:   item.AniListID,
				ArrIgnored:  true,
			}

			names := []model.TitleName{
				{
					Name:      item.Title,
					Language:  "en",
					IsPrimary: true,
				},
			}

			newID, err := titleWriter.Create(ctx, title, names)
			if err != nil {
				return fmt.Errorf("create title: %w", err)
			}

			// Enqueue background enrichment task if IDs are present
			if title.TMDBID != nil || title.IMDBID != nil || title.AniListID != nil || title.TVDBID != nil {
				var tmdbID int64
				if title.TMDBID != nil {
					tmdbID = *title.TMDBID
				}
				var imdbID string
				if title.IMDBID != nil {
					imdbID = *title.IMDBID
				}
				var tvdbID int64
				if title.TVDBID != nil {
					tvdbID = *title.TVDBID
				}
				var anilistID int64
				if title.AniListID != nil {
					anilistID = *title.AniListID
				}

				payload := EnrichmentPayload{
					TitleID:       newID,
					TitleName:     item.Title,
					Year:          title.Year,
					TitleType:     title.Type,
					IsAnime:       title.IsAnime,
					IMDBID:        imdbID,
					TMDBID:        tmdbID,
					TVDBID:        tvdbID,
					AniListID:     anilistID,
					PreserveMatch: title.MatchStatus == model.MatchStatusConfirmed,
				}
				payloadJSON, _ := json.Marshal(payload)
				dedupKey := fmt.Sprintf("enrichment:%d", newID)
				if _, err := taskWriter.Enqueue(ctx, model.TaskTypeEnrichment, string(payloadJSON), &dedupKey); err != nil {
					return fmt.Errorf("enqueue enrichment task: %w", err)
				}
			}

			res.Created++
			itemRes.Status = "created"
			itemRes.TitleID = &newID
			res.Items[i] = itemRes
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return res, nil
}

func enqueueAniListPushesOnTitleUpdate(ctx context.Context, tx *sql.Tx, before *model.Title, newStatus *model.TitleStatus, newRating *int) {
	statusChanged := newStatus != nil && *newStatus != before.Status
	ratingChanged := newRating != nil && !intPtrEq(newRating, before.MyRating)
	if !statusChanged && !ratingChanged {
		return
	}

	if before.Type == model.TitleTypeMovie {
		if before.IsAnime && before.AniListID != nil && *before.AniListID != 0 {
			EnqueueAniListMoviePush(ctx, tx, before.ID)
		}
		return
	}

	effectiveStatus := before.Status
	if newStatus != nil {
		effectiveStatus = *newStatus
	}
	ratingOnly := !statusChanged && ratingChanged

	seen := map[int64]bool{}
	for _, season := range before.Seasons {
		if seen[season.ID] {
			continue
		}
		if ratingOnly {
			total, watched := seasonWatchCounts(season)
			derived, _ := DeriveSeasonState(string(effectiveStatus), total, watched)
			if !ShouldPushRating(derived) {
				continue
			}
		}
		EnqueueAniListSeasonPush(ctx, tx, season.ID)
		seen[season.ID] = true
	}
}

func seasonWatchCounts(s model.Season) (total, watched int) {
	if s.TotalEpisodes != nil {
		total = *s.TotalEpisodes
	}
	if total == 0 {
		total = len(s.Episodes)
	}
	for _, ep := range s.Episodes {
		if ep.Watched {
			watched++
		}
	}
	return total, watched
}

func intPtrEq(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
