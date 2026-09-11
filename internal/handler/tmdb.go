package handler

import (
	"net/http"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/service/matching"
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
	Type      string  `json:"type,omitempty"`
	Overview  string  `json:"overview,omitempty"`
}

func toTMDBDTO(r matching.TMDBSearchResult, mediaType string) tmdbSearchResultDTO {
	item := tmdbSearchResultDTO{
		ID:       r.ID,
		Title:    r.DisplayTitle(),
		Year:     r.Year(),
		Type:     mediaType,
		Overview: r.Overview,
	}
	if r.PosterPath != nil && *r.PosterPath != "" {
		url := tmdbImageURL + *r.PosterPath
		item.PosterURL = &url
	}
	return item
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

	var dto []tmdbSearchResultDTO

	switch mediaType {
	case "movie":
		results, err := h.tmdb.SearchMovie(r.Context(), query, 0)
		if err != nil {
			return httputil.InternalError("TMDB search failed", err)
		}
		dto = make([]tmdbSearchResultDTO, 0, len(results))
		for _, r := range results {
			dto = append(dto, toTMDBDTO(r, "movie"))
		}
	case "tv":
		results, err := h.tmdb.SearchTV(r.Context(), query, 0)
		if err != nil {
			return httputil.InternalError("TMDB search failed", err)
		}
		dto = make([]tmdbSearchResultDTO, 0, len(results))
		for _, r := range results {
			dto = append(dto, toTMDBDTO(r, "tv"))
		}
	case "all", "multi":
		movies, _ := h.tmdb.SearchMovie(r.Context(), query, 0)
		tvs, _ := h.tmdb.SearchTV(r.Context(), query, 0)
		dto = make([]tmdbSearchResultDTO, 0, len(movies)+len(tvs))
		maxLen := len(movies)
		if len(tvs) > maxLen {
			maxLen = len(tvs)
		}
		for i := 0; i < maxLen; i++ {
			if i < len(movies) {
				dto = append(dto, toTMDBDTO(movies[i], "movie"))
			}
			if i < len(tvs) {
				dto = append(dto, toTMDBDTO(tvs[i], "tv"))
			}
		}
	default:
		return httputil.BadRequest("type must be movie or tv")
	}

	httputil.WriteJSON(w, http.StatusOK, dto)
	return nil
}
