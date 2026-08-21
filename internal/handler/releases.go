package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
)

type ReleasesHandler struct {
	db          *sql.DB
	prowlarrSvc *service.ProwlarrService
	titles      *repository.TitleRepository
	tasks       *repository.TaskRepository
}

func NewReleasesHandler(db *sql.DB, prowlarrSvc *service.ProwlarrService, titles *repository.TitleRepository, tasks *repository.TaskRepository) *ReleasesHandler {
	return &ReleasesHandler{
		db:          db,
		prowlarrSvc: prowlarrSvc,
		titles:      titles,
		tasks:       tasks,
	}
}

// List returns the latest releases from Prowlarr.
func (h *ReleasesHandler) List(w http.ResponseWriter, r *http.Request) error {
	if h.prowlarrSvc == nil {
		return httputil.BadRequest("Prowlarr service not initialized")
	}

	releaseType := r.URL.Query().Get("type")
	forceRefresh := r.URL.Query().Get("refresh") == "true"

	releases, err := h.prowlarrSvc.GetReleases(r.Context(), releaseType, forceRefresh)
	if err != nil {
		return httputil.InternalError("Failed to fetch releases from Prowlarr", err)
	}

	httputil.WriteJSON(w, http.StatusOK, releases)
	return nil
}

type AddReleasePayload struct {
	TMDBID    int64           `json:"tmdb_id"`
	Type      model.TitleType `json:"type"` // "movie" or "series"
	Title     string          `json:"title"`
	Year      int             `json:"year"`
	PosterURL *string         `json:"poster_url"`
	IMDBID    *string         `json:"imdb_id"`
}

// Add creates a new title in PlexTracker directly from a release.
func (h *ReleasesHandler) Add(w http.ResponseWriter, r *http.Request) error {
	var payload AddReleasePayload
	if err := httputil.ReadJSON(r, &payload, 65536); err != nil {
		return httputil.BadRequest("Invalid request body")
	}

	if payload.Title == "" {
		return httputil.BadRequest("Title is required")
	}
	if payload.Type == "" {
		payload.Type = model.TitleTypeMovie
	}

	// Check if title already exists by TMDB ID
	if payload.TMDBID > 0 {
		existing, _ := h.titles.FindByExternalID(nil, &payload.TMDBID, nil, nil, &payload.Type)
		if existing != nil {
			httputil.WriteJSON(w, http.StatusOK, existing)
			return nil
		}
	}

	source := "prowlarr"
	var tmdbIDPtr *int64
	if payload.TMDBID > 0 {
		tmdbIDPtr = &payload.TMDBID
	}

	matchStatus := model.MatchStatusPendingReview
	if payload.TMDBID > 0 {
		matchStatus = model.MatchStatusConfirmed
	}

	title := &model.Title{
		Type:        payload.Type,
		Year:        payload.Year,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: matchStatus,
		MatchSource: &source,
		CoverURL:    payload.PosterURL,
		IMDBID:      payload.IMDBID,
		TMDBID:      tmdbIDPtr,
		ArrIgnored:  true,
	}

	names := []model.TitleName{
		{
			Name:      payload.Title,
			Language:  "en",
			IsPrimary: true,
		},
	}

	var newID int64
	if err := database.WithTxContext(r.Context(), h.db, func(tx *sql.Tx) error {
		id, err := repository.NewTitleWriter(tx).Create(r.Context(), title, names)
		if err != nil {
			return err
		}
		newID = id

		// Enqueue enrichment task if TMDB ID is present
		if tmdbIDPtr != nil && h.tasks != nil {
			payloadJSON, _ := json.Marshal(map[string]any{"title_id": newID})
			dedupKey := fmt.Sprintf("enrich_%d", newID)
			_, _ = repository.NewTaskWriter(tx).Enqueue(r.Context(), model.TaskTypeEnrichment, string(payloadJSON), &dedupKey)
		}
		return nil
	}); err != nil {
		return httputil.InternalError("Failed to add title", err)
	}

	created, err := h.titles.GetByID(newID)
	if err != nil {
		return httputil.InternalError("Failed to load created title", err)
	}

	httputil.WriteJSON(w, http.StatusCreated, created)
	return nil
}
