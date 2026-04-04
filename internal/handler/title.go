package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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

func (h *TitleHandler) List(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if titles == nil {
		titles = []model.Title{}
	}

	writeJSON(w, titles)
}

func (h *TitleHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	title, err := h.titles.GetByID(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	writeJSON(w, title)
}

func (h *TitleHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	if err := json.NewDecoder(io.LimitReader(r.Body, 65536)).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	created, _ := h.titles.GetByID(id)
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, created)
}

func (h *TitleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Status      *model.TitleStatus  `json:"status"`
		MatchStatus *model.MatchStatus  `json:"match_status"`
		MyRating    *int                `json:"my_rating"`
		Type        *model.TitleType    `json:"type"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	update := repository.TitleUpdate{
		Status:      body.Status,
		MatchStatus: body.MatchStatus,
		MyRating:    body.MyRating,
	}

	if err := h.titles.Update(id, update); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	title, _ := h.titles.GetByID(id)
	writeJSON(w, title)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
