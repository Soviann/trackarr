package handler

import (
	"database/sql"
	"net/http"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
)

type ReleasesHandler struct {
	db          *sql.DB
	prowlarrSvc *service.ProwlarrService
	titles      *repository.TitleRepository
	tasks       *repository.TaskRepository
	titleSvc    *service.TitleService
}

func NewReleasesHandler(db *sql.DB, prowlarrSvc *service.ProwlarrService, titles *repository.TitleRepository, tasks *repository.TaskRepository, titleSvc ...*service.TitleService) *ReleasesHandler {
	var svc *service.TitleService
	if len(titleSvc) > 0 && titleSvc[0] != nil {
		svc = titleSvc[0]
	} else if db != nil {
		svc = service.NewTitleService(db, titles, tasks, nil)
	}
	return &ReleasesHandler{
		db:          db,
		prowlarrSvc: prowlarrSvc,
		titles:      titles,
		tasks:       tasks,
		titleSvc:    svc,
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

// Add creates a new title in Trackarr directly from a release.
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
		existing, _ := h.titles.FindByExternalID(nil, &payload.TMDBID, nil, nil, nil, &payload.Type)
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

	newID, err := h.titleSvc.CreateAndEnrich(r.Context(), title, names, tmdbIDPtr != nil)
	if err != nil {
		return httputil.InternalError("Failed to add title", err)
	}

	created, err := h.titles.GetByID(newID)
	if err != nil {
		return httputil.InternalError("Failed to load created title", err)
	}

	httputil.WriteJSON(w, http.StatusCreated, created)
	return nil
}
