package handler

import (
	"database/sql"
	"net/http"

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

	title, err := h.service.ToggleEpisodeWatched(h.db, titleID, episodeID)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

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

	title, err := h.service.MarkEpisodesWatched(h.db, titleID, body.EpisodeIDs, model.WatchEventSourceManual, nil)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}

	httputil.WriteJSON(w, http.StatusOK, title)
	return nil
}
