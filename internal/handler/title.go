package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
)

type TitleHandler struct {
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
	tasks    *repository.TaskRepository
}

func NewTitleHandler(titles *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, tasks *repository.TaskRepository) *TitleHandler {
	return &TitleHandler{titles: titles, seasons: seasons, episodes: episodes, events: events, tasks: tasks}
}

var allowedSorts = map[string]bool{
	"updated_at":     true,
	"original_title": true,
	"year":           true,
	"my_rating":      true,
	"created_at":     true,
	"release_date":   true,
	"last_watched_at": true,
}

func (h *TitleHandler) List(w http.ResponseWriter, r *http.Request) error {
	filter := repository.TitleFilter{
		Limit:  httputil.ParseQueryInt(r, "limit", repository.DefaultPageSize),
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
	if q := r.URL.Query().Get("search"); q != "" {
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

	result, err := h.titles.List(filter)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	if result.Titles == nil {
		result.Titles = []model.Title{}
	}

	// Include global counts on first page (for match review banner)
	if filter.Offset == 0 && filter.Search == nil {
		counts, err := h.titles.GetStatusCounts()
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

	title, err := h.titles.GetByID(id)
	if err != nil {
		return httputil.NotFound("Not found")
	}

	httputil.WriteJSON(w, http.StatusOK, title)
	return nil
}

func (h *TitleHandler) Create(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Type        model.TitleType   `json:"type"`
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

	id, err := h.titles.Create(title, body.Names)
	if err != nil {
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
	}

	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	update := repository.TitleUpdate{
		Status:      body.Status,
		MatchStatus: body.MatchStatus,
		MyRating:    body.MyRating,
	}

	if err := h.titles.Update(id, update); err != nil {
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

	title, err := h.titles.GetByID(id)
	if err != nil {
		return httputil.NotFound("Not found")
	}

	var body struct {
		TMDBID    *int64  `json:"tmdb_id"`
		IMDBID    *string `json:"imdb_id"`
		AniListID *int64  `json:"anilist_id"`
	}

	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	if body.TMDBID == nil && body.IMDBID == nil && body.AniListID == nil {
		return httputil.BadRequest("At least one ID is required")
	}

	// Update IDs + match status
	matchStatus := model.MatchStatusConfirmed
	matchSource := "manual"
	update := repository.TitleUpdate{
		MatchStatus: &matchStatus,
		MatchSource: &matchSource,
	}
	if body.TMDBID != nil {
		update.TMDBID = body.TMDBID
	}
	if body.IMDBID != nil {
		update.IMDBID = body.IMDBID
	}
	if body.AniListID != nil {
		update.AniListID = body.AniListID
	}

	if err := h.titles.Update(id, update); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	// Enqueue enrichment task with the new IDs
	tmdbID := int64(0)
	if body.TMDBID != nil {
		tmdbID = *body.TMDBID
	} else if title.TMDBID != nil {
		tmdbID = *title.TMDBID
	}
	imdbID := ""
	if body.IMDBID != nil {
		imdbID = *body.IMDBID
	} else if title.IMDBID != nil {
		imdbID = *title.IMDBID
	}

	payload, _ := json.Marshal(service.EnrichmentPayload{
		TitleID:   id,
		TitleName: title.PrimaryName(),
		Year:      title.Year,
		TitleType: title.Type,
		IMDBID:    imdbID,
		TMDBID:    tmdbID,
	})
	dedupKey := fmt.Sprintf("enrichment:%d", id)
	if _, err := h.tasks.Enqueue(model.TaskTypeEnrichment, string(payload), &dedupKey); err != nil {
		return httputil.InternalError("Failed to enqueue enrichment", err)
	}

	updated, _ := h.titles.GetByID(id)
	httputil.WriteJSON(w, http.StatusOK, updated)
	return nil
}
