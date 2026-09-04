package handler

import (
	"context"
	"net/http"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/service/matching"
)

type aniListSearcher interface {
	SearchAnime(ctx context.Context, title string) ([]matching.AniListSearchResult, error)
}

type AniListSearchHandler struct {
	client aniListSearcher
}

func NewAniListSearchHandler(client aniListSearcher) *AniListSearchHandler {
	return &AniListSearchHandler{client: client}
}

type aniListSearchResultDTO struct {
	ID           int64   `json:"id"`
	RomajiTitle  string  `json:"romaji_title"`
	EnglishTitle string  `json:"english_title"`
	Title        string  `json:"title"`
	Year         *int    `json:"year"`
	Format       string  `json:"format"`
	Episodes     *int    `json:"episodes"`
	PosterURL    *string `json:"poster_url"`
}

func (h *AniListSearchHandler) Search(w http.ResponseWriter, r *http.Request) error {
	if h.client == nil {
		return httputil.BadRequest("AniList client not configured")
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		return httputil.BadRequest("query is required")
	}

	results, err := h.client.SearchAnime(r.Context(), query)
	if err != nil {
		return httputil.InternalError("AniList search failed", err)
	}

	dto := make([]aniListSearchResultDTO, 0, len(results))
	for _, res := range results {
		item := aniListSearchResultDTO{
			ID:           res.ID,
			RomajiTitle:  res.RomajiTitle,
			EnglishTitle: res.EnglishTitle,
			Title:        res.DisplayTitle(),
			Year:         res.SeasonYear,
			Format:       res.Format,
			Episodes:     res.Episodes,
		}
		if res.CoverURL != "" {
			cover := res.CoverURL
			item.PosterURL = &cover
		}
		dto = append(dto, item)
	}

	httputil.WriteJSON(w, http.StatusOK, dto)
	return nil
}
