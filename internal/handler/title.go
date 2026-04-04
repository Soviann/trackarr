package handler

import (
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type TitleHandler struct {
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
}

func NewTitleHandler(titles *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository) *TitleHandler {
	return &TitleHandler{titles: titles, seasons: seasons, episodes: episodes, events: events}
}

func (h *TitleHandler) List(w http.ResponseWriter, r *http.Request) error {
	filter := repository.TitleFilter{}
	if s := r.URL.Query().Get("status"); s != "" {
		status := model.TitleStatus(s)
		filter.Status = &status
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

	titles, err := h.titles.List(filter)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	if titles == nil {
		titles = []model.Title{}
	}

	httputil.WriteJSON(w, http.StatusOK, titles)
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
		Status      *model.TitleStatus  `json:"status"`
		MatchStatus *model.MatchStatus  `json:"match_status"`
		MyRating    *int                `json:"my_rating"`
		Type        *model.TitleType    `json:"type"`
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
