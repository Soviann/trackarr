package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type TitleHandler struct {
	serverCtx  context.Context // lifecycle ctx — cancelled on SIGTERM so fire-and-forget goroutines stop at shutdown
	db         *sql.DB
	titles     *repository.TitleRepository // writeDB — Create, Update, Merge, Rematch
	titlesRead *repository.TitleRepository // readDB — List, GetByID (non-blocking reads)
	seasons    *repository.SeasonRepository
	episodes   *repository.EpisodeRepository
	events     *repository.WatchEventRepository
	tasks      *repository.TaskRepository
	pipeline   *matching.Pipeline
	service    *service.TitleService
	bgSvc      *service.BackgroundService
	shutdownWG *sync.WaitGroup // optional — joined on shutdown so fire-and-forget refresh can finish
}

func NewTitleHandler(serverCtx context.Context, db *sql.DB, titles *repository.TitleRepository, titlesRead *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, tasks *repository.TaskRepository, pipeline *matching.Pipeline, svc *service.TitleService, bgSvc *service.BackgroundService) *TitleHandler {
	return &TitleHandler{
		serverCtx:  serverCtx,
		db:         db,
		titles:     titles,
		titlesRead: titlesRead,
		seasons:    seasons,
		episodes:   episodes,
		events:     events,
		tasks:      tasks,
		pipeline:   pipeline,
		service:    svc,
		bgSvc:      bgSvc,
	}
}

// SetShutdownWG registers a WaitGroup that RefreshOne goroutines increment on
// start and decrement on exit, so Serve() can wait for in-flight refresh before
// closing the database.
func (h *TitleHandler) SetShutdownWG(wg *sync.WaitGroup) {
	if h == nil {
		return
	}
	h.shutdownWG = wg
}

var allowedSorts = map[string]bool{
	"updated_at":      true,
	"original_title":  true,
	"year":            true,
	"my_rating":       true,
	"created_at":      true,
	"release_date":    true,
	"last_watched_at": true,
}

// findTitleByURL returns an existing title matching an external URL pasted in the
// search box (IMDB / AniList only). Returns nil when the query is not a
// recognized URL or no matching title exists.
func (h *TitleHandler) findTitleByURL(q string) *model.Title {
	ids := matching.ParseURL(q)
	if ids == nil {
		return nil
	}
	var (
		imdbPtr    *string
		anilistPtr *int64
	)
	if ids.IMDB != "" {
		imdbPtr = &ids.IMDB
	}
	if ids.AniList != 0 {
		anilistPtr = &ids.AniList
	}
	if imdbPtr == nil && anilistPtr == nil {
		return nil
	}
	t, err := h.titlesRead.FindByExternalID(imdbPtr, nil, nil, anilistPtr, nil)
	if err != nil {
		return nil
	}
	return t
}

func (h *TitleHandler) List(w http.ResponseWriter, r *http.Request) error {
	limit := httputil.ParseQueryInt(r, "limit", repository.DefaultPageSize)
	if limit > repository.MaxPageSize {
		limit = repository.MaxPageSize
	}
	filter := repository.TitleFilter{
		Limit:  limit,
		Offset: httputil.ParseQueryInt(r, "offset", 0),
	}
	if sortField := r.URL.Query().Get("sort"); allowedSorts[sortField] {
		filter.Sort = sortField
	}
	if order := r.URL.Query().Get("order"); order == "asc" || order == "desc" {
		filter.Order = order
	}
	if s := r.URL.Query().Get("status"); s != "" {
		switch s {
		case "up_to_date":
			filter.UpToDate = true
		case "watching_behind":
			filter.WatchingBehind = true
		default:
			status := model.TitleStatus(s)
			filter.Status = &status
		}
	}
	if t := r.URL.Query().Get("type"); t != "" {
		titleType := model.TitleType(t)
		filter.Type = &titleType
	}
	if ia := r.URL.Query().Get("is_anime"); ia != "" {
		isAnime := ia == "true"
		filter.IsAnime = &isAnime
	}
	if q := r.URL.Query().Get("search"); q != "" {
		if t := h.findTitleByURL(q); t != nil {
			httputil.WriteJSON(w, http.StatusOK, repository.PaginatedResult{
				Titles: []model.Title{*t},
				Total:  1,
			})
			return nil
		}
		filter.Search = &q
	}
	if m := r.URL.Query().Get("match_status"); m != "" {
		matchStatus := model.MatchStatus(m)
		filter.MatchStatus = &matchStatus
	}
	if ss := r.URL.Query().Get("series_status"); ss != "" {
		seriesStatus := model.SeriesStatus(ss)
		filter.SeriesStatus = &seriesStatus
	}
	if d := r.URL.Query().Get("decade"); d != "" {
		if decade := httputil.ParseQueryInt(r, "decade", 0); decade >= 1900 && decade <= 2100 {
			filter.Decade = &decade
		}
	}
	if rf := r.URL.Query().Get("release_from"); rf != "" {
		filter.ReleaseFrom = &rf
	}
	if rt := r.URL.Query().Get("release_to"); rt != "" {
		filter.ReleaseTo = &rt
	}
	filter.IncludeNoRelease = true // default: include titles without release date
	if r.URL.Query().Get("include_no_release") == "false" {
		filter.IncludeNoRelease = false
	}
	if genres := r.URL.Query()["genres"]; len(genres) > 0 {
		filter.Genres = genres
		if op := r.URL.Query().Get("genre_op"); op == "AND" {
			filter.GenreOp = "AND"
		} else {
			filter.GenreOp = "OR"
		}
	}

	if p := r.URL.Query().Get("person"); p != "" {
		filter.Person = &p
	}

	result, err := h.titlesRead.List(filter)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	if result.Titles == nil {
		result.Titles = []model.Title{}
	}

	// Include global counts on first page (for match review banner)
	if filter.Offset == 0 && filter.Search == nil {
		counts, err := h.titlesRead.GetStatusCounts()
		if err == nil {
			result.Counts = counts
		}
	}

	httputil.WriteJSON(w, http.StatusOK, result)
	return nil
}

func (h *TitleHandler) GetByID(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return httputil.BadRequest("Invalid ID")
	}

	title, err := h.titlesRead.GetByID(id)
	if err != nil {
		return httputil.NotFound("Not found")
	}

	httputil.WriteJSON(w, http.StatusOK, title)
	return nil
}

func (h *TitleHandler) Resolve(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query().Get("q")
	if q == "" {
		return httputil.BadRequest("Query is required")
	}

	result, err := h.service.ResolveURL(r.Context(), q)
	if err != nil {
		return httputil.BadRequest(fmt.Sprintf("Failed to resolve URL: %v", err))
	}

	httputil.WriteJSON(w, http.StatusOK, result)
	return nil
}

func (h *TitleHandler) Create(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Type        model.TitleType   `json:"type"`
		IsAnime     bool              `json:"is_anime"`
		Year        int               `json:"year"`
		Status      model.TitleStatus `json:"status"`
		MatchStatus model.MatchStatus `json:"match_status"`
		Names       []model.TitleName `json:"names"`
		CoverURL    *string           `json:"cover_url"`
		IMDBID      *string           `json:"imdb_id"`
		AniListID   *int64            `json:"anilist_id"`
		TMDBID      *int64            `json:"tmdb_id"`
		TVDBID      *int64            `json:"tvdb_id"`
	}

	if err := httputil.ReadJSON(r, &body, 65536); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	manualSource := "manual"
	title := &model.Title{
		Type:        body.Type,
		IsAnime:     body.IsAnime,
		Year:        body.Year,
		Status:      body.Status,
		MatchStatus: body.MatchStatus,
		MatchSource: &manualSource,
		CoverURL:    body.CoverURL,
		IMDBID:      body.IMDBID,
		AniListID:   body.AniListID,
		TMDBID:      body.TMDBID,
		TVDBID:      body.TVDBID,
	}

	var id int64
	if err := database.WithTxContext(r.Context(), h.db, func(tx *sql.Tx) error {
		newID, createErr := repository.NewTitleWriter(tx).Create(r.Context(), title, body.Names)
		if createErr != nil {
			return createErr
		}
		id = newID
		return nil
	}); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	created, _ := h.titles.GetByID(id)
	httputil.WriteJSON(w, http.StatusCreated, created)
	return nil
}

func (h *TitleHandler) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return httputil.BadRequest("Invalid ID")
	}

	var body struct {
		Status      *model.TitleStatus `json:"status"`
		MatchStatus *model.MatchStatus `json:"match_status"`
		MyRating    *int               `json:"my_rating"`
		Type        *model.TitleType   `json:"type"`
		IsAnime     *bool              `json:"is_anime"`
	}

	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	update := repository.TitleUpdate{
		Status:      body.Status,
		MatchStatus: body.MatchStatus,
		MyRating:    body.MyRating,
		Type:        body.Type,
		IsAnime:     body.IsAnime,
	}

	if err := database.WithTxContext(r.Context(), h.db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(r.Context(), id, update)
	}); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	title, _ := h.titles.GetByID(id)
	httputil.WriteJSON(w, http.StatusOK, title)
	return nil
}

func (h *TitleHandler) Rematch(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return httputil.BadRequest("Invalid ID")
	}

	var body struct {
		TMDBID    *int64  `json:"tmdb_id"`
		IMDBID    *string `json:"imdb_id"`
		AniListID *int64  `json:"anilist_id"`
		TVDBID    *int64  `json:"tvdb_id"`
	}

	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	if body.TMDBID == nil && body.IMDBID == nil && body.AniListID == nil && body.TVDBID == nil {
		return httputil.BadRequest("At least one ID is required")
	}

	if err := h.service.Rematch(r.Context(), h.db, id, body.IMDBID, body.TMDBID, body.AniListID, body.TVDBID); err != nil {
		return httputil.InternalError("Failed to rematch", err)
	}

	updated, _ := h.titles.GetByID(id)
	httputil.WriteJSON(w, http.StatusOK, updated)
	return nil
}

func (h *TitleHandler) Merge(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return httputil.BadRequest("Invalid ID")
	}

	var body struct {
		TargetID     int64 `json:"target_id"`
		SeasonOffset *int  `json:"season_offset"`
	}
	if err := httputil.ReadJSON(r, &body, 1<<20); err != nil {
		return httputil.BadRequest("Invalid JSON")
	}

	if body.TargetID == 0 {
		return httputil.BadRequest("Missing target_id")
	}

	if id == body.TargetID {
		return httputil.BadRequest("Cannot merge a title with itself")
	}

	if err := h.service.Merge(r.Context(), h.db, body.TargetID, id, body.SeasonOffset); err != nil {
		return httputil.InternalError("Failed to merge titles", err)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}

// RefreshOne triggers a metadata refresh for a single title.
func (h *TitleHandler) RefreshOne(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return httputil.BadRequest("invalid title id")
	}

	if h.bgSvc == nil {
		return httputil.InternalError("refresh title", fmt.Errorf("background service not available"))
	}

	// Intentional fire-and-forget: 202 Accepted. The refresh runs in background
	// with a 2-minute timeout; errors are logged. The caller does not wait for
	// completion. Parent ctx is the server lifecycle so SIGTERM cancels the
	// goroutine instead of leaking it until its own timeout. The shutdown WG
	// ensures Serve() waits for the goroutine before closing the database.
	ctx, cancel := context.WithTimeout(h.serverCtx, 2*time.Minute)
	if h.shutdownWG != nil {
		h.shutdownWG.Add(1)
	}
	go func() {
		if h.shutdownWG != nil {
			defer h.shutdownWG.Done()
		}
		defer cancel()
		if err := h.bgSvc.RefreshByID(ctx, id); err != nil {
			log.Printf("refresh title %d: %v", id, err)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	return nil
}

// Delete removes a title by ID.
func (h *TitleHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return httputil.BadRequest("Invalid ID")
	}
	if err := database.WithTxContext(r.Context(), h.db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Delete(r.Context(), id)
	}); err != nil {
		return fmt.Errorf("title: delete: %w", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// BatchDelete removes multiple titles by ID.
func (h *TitleHandler) BatchDelete(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := httputil.ReadJSON(r, &body, 1<<20); err != nil {
		return httputil.BadRequest("Invalid body")
	}
	if len(body.IDs) == 0 {
		return httputil.BadRequest("ids is required")
	}
	if err := database.WithTxContext(r.Context(), h.db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).BatchDelete(r.Context(), body.IDs)
	}); err != nil {
		return fmt.Errorf("title: batch delete: %w", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// BatchStatus updates the status of multiple titles.
func (h *TitleHandler) BatchStatus(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		IDs    []int64 `json:"ids"`
		Status string  `json:"status"`
	}
	if err := httputil.ReadJSON(r, &body, 1<<20); err != nil {
		return httputil.BadRequest("Invalid body")
	}
	if len(body.IDs) == 0 || body.Status == "" {
		return httputil.BadRequest("ids and status are required")
	}
	validStatuses := map[string]bool{"watching": true, "completed": true, "dropped": true, "plan_to_watch": true}
	if !validStatuses[body.Status] {
		return httputil.BadRequest("Invalid status")
	}
	if err := database.WithTxContext(r.Context(), h.db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).BatchUpdateStatus(r.Context(), body.IDs, body.Status)
	}); err != nil {
		return fmt.Errorf("title: batch status: %w", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ReviewCount handles GET /api/titles/review-count.
// Returns the number of titles needing match review (pending_review + unconfirmed).
func (h *TitleHandler) ReviewCount(w http.ResponseWriter, r *http.Request) error {
	count, err := h.titlesRead.ReviewCount(r.Context())
	if err != nil {
		return fmt.Errorf("review count: %w", err)
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]int{"count": count})
	return nil
}
