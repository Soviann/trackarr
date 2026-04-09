package handler

import (
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

const tmdbImageURL = "https://image.tmdb.org/t/p/w342"

type TMDBHandler struct {
	tmdb *matching.TMDBClient
}

func NewTMDBHandler(tmdb *matching.TMDBClient) *TMDBHandler {
	return &TMDBHandler{tmdb: tmdb}
}

type tmdbSearchResultDTO struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	Year      int     `json:"year"`
	PosterURL *string `json:"poster_url"`
}

func (h *TMDBHandler) Search(w http.ResponseWriter, r *http.Request) error {
	if h.tmdb == nil {
		return httputil.BadRequest("TMDB not configured")
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		return httputil.BadRequest("query is required")
	}

	mediaType := r.URL.Query().Get("type")
	if mediaType == "" {
		mediaType = "movie"
	}

	var results []matching.TMDBSearchResult
	var err error

	switch mediaType {
	case "movie":
		results, err = h.tmdb.SearchMovie(r.Context(), query, 0)
	case "tv":
		results, err = h.tmdb.SearchTV(r.Context(), query, 0)
	default:
		return httputil.BadRequest("type must be movie or tv")
	}

	if err != nil {
		return httputil.InternalError("TMDB search failed", err)
	}

	dto := make([]tmdbSearchResultDTO, 0, len(results))
	for _, r := range results {
		item := tmdbSearchResultDTO{
			ID:    r.ID,
			Title: r.DisplayTitle(),
			Year:  r.Year(),
		}
		if r.PosterPath != nil && *r.PosterPath != "" {
			url := tmdbImageURL + *r.PosterPath
			item.PosterURL = &url
		}
		dto = append(dto, item)
	}

	httputil.WriteJSON(w, http.StatusOK, dto)
	return nil
}
