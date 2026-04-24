package handler

import (
	"database/sql"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/service"
)

type EpisodeHandler struct {
	db      *sql.DB
	service *service.LibraryService
}

func NewEpisodeHandler(db *sql.DB, svc *service.LibraryService) *EpisodeHandler {
	return &EpisodeHandler{
		db:      db,
		service: svc,
	}
}

func (h *EpisodeHandler) ToggleWatched(w http.ResponseWriter, r *http.Request) error {
	titleID, err := httputil.ParseIDParam(r, "titleID")
	if err != nil {
		return httputil.BadRequest("Invalid title ID")
	}

	episodeID, err := httputil.ParseIDParam(r, "episodeID")
	if err != nil {
		return httputil.BadRequest("Invalid episode ID")
	}

	var (
		title  *model.Title
		prompt *service.RatingPrompt
	)
	if err := database.WithTxContext(r.Context(), h.db, func(tx *sql.Tx) error {
		t, p, e := h.service.ToggleEpisodeWatched(r.Context(), tx, titleID, episodeID)
		if e != nil {
			return e
		}
		title, prompt = t, p
		return nil
	}); err != nil {
		return httputil.InternalError("Internal error", err)
	}
	// Prompt delivery runs after the tx commits so a slow webpush endpoint
	// cannot tie up the sole write connection.
	h.service.SendRatingPrompt(r.Context(), prompt)

	httputil.WriteJSON(w, http.StatusOK, title)
	return nil
}

func (h *EpisodeHandler) BatchMarkWatched(w http.ResponseWriter, r *http.Request) error {
	titleID, err := httputil.ParseIDParam(r, "titleID")
	if err != nil {
		return httputil.BadRequest("Invalid title ID")
	}

	var body struct {
		EpisodeIDs []int64 `json:"episode_ids"`
	}
	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	var (
		title  *model.Title
		prompt *service.RatingPrompt
	)
	if err := database.WithTxContext(r.Context(), h.db, func(tx *sql.Tx) error {
		t, p, e := h.service.MarkEpisodesWatched(r.Context(), tx, titleID, body.EpisodeIDs, model.WatchEventSourceManual, nil)
		if e != nil {
			return e
		}
		title, prompt = t, p
		return nil
	}); err != nil {
		return httputil.InternalError("Internal error", err)
	}
	h.service.SendRatingPrompt(r.Context(), prompt)

	httputil.WriteJSON(w, http.StatusOK, title)
	return nil
}
