package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

type TitleHandler struct {
	db       *sql.DB
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
	tasks    *repository.TaskRepository
	pipeline *matching.Pipeline
	service  *service.TitleService
}

func NewTitleHandler(db *sql.DB, titles *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, tasks *repository.TaskRepository, pipeline *matching.Pipeline, svc *service.TitleService) *TitleHandler {
	return &TitleHandler{
		db:       db,
		titles:   titles,
		seasons:  seasons,
		episodes: episodes,
		events:   events,
		tasks:    tasks,
		pipeline: pipeline,
		service:  svc,
	}
}

var (
	reIMDB    = regexp.MustCompile(`imdb\.com/title/(tt\d+)`)
	reAniList = regexp.MustCompile(`anilist\.co/anime/(\d+)`)
)

var allowedSorts = map[string]bool{
	"updated_at":      true,
	"original_title":  true,
	"year":            true,
	"my_rating":       true,
	"created_at":      true,
	"release_date":    true,
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
	if ia := r.URL.Query().Get("is_anime"); ia != "" {
		isAnime := ia == "true"
		filter.IsAnime = &isAnime
	}
	if q := r.URL.Query().Get("search"); q != "" {
		// If search looks like a URL, try to find by external ID first
		if m := reIMDB.FindStringSubmatch(q); m != nil {
			id := m[1]
			if t, err := h.titles.FindByExternalID(&id, nil, nil, nil, nil); err == nil {
				httputil.WriteJSON(w, http.StatusOK, repository.PaginatedResult{
					Titles: []model.Title{*t},
					Total:  1,
				})
				return nil
			}
		}
		if m := reAniList.FindStringSubmatch(q); m != nil {
			idStr := m[1]
			if alID, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				if t, err := h.titles.FindByExternalID(nil, nil, nil, &alID, nil); err == nil {
					httputil.WriteJSON(w, http.StatusOK, repository.PaginatedResult{
						Titles: []model.Title{*t},
						Total:  1,
					})
					return nil
				}
			}
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

	if err := h.service.Rematch(h.db, id, body.IMDBID, body.TMDBID, body.AniListID); err != nil {
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
