package handler

import (
	"database/sql"
	"net/http"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
)

type EpisodeHandler struct {
	db         *sql.DB
	service    *service.LibraryService
	titlesRead *repository.TitleRepository // readDB — reload after post-commit backfill
}

func NewEpisodeHandler(db *sql.DB, svc *service.LibraryService, titlesRead *repository.TitleRepository) *EpisodeHandler {
	return &EpisodeHandler{
		db:         db,
		service:    svc,
		titlesRead: titlesRead,
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

	title, ep, prompt, err := h.service.ToggleEpisodeWatchedTx(r.Context(), titleID, episodeID)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}
	// Fire post-commit side effects sequentially AFTER the write tx releases
	// the sole SQLite writer. Running either of these inside the tx would
	// deadlock (backfill opens its own writeDB tx) or block the writer on
	// unrelated HTTP I/O (webpush).
	h.service.TriggerBackfillForEpisode(r.Context(), titleID, ep)
	h.service.SendRatingPrompt(r.Context(), prompt)

	// The backfill above marks previous episodes watched in its own post-commit
	// tx, so the `title` returned from ToggleEpisodeWatched predates the cascade.
	// Reload from readDB so the response reflects every auto-checked episode and
	// the client doesn't show stale season state.
	if ep != nil && ep.Watched {
		if reloaded, err := h.titlesRead.GetByID(titleID); err == nil && reloaded != nil {
			title = reloaded
		}
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

	title, prompt, err := h.service.MarkEpisodesWatchedTx(r.Context(), titleID, body.EpisodeIDs, nil, model.WatchEventSourceManual, nil)
	if err != nil {
		return httputil.InternalError("Internal error", err)
	}
	h.service.SendRatingPrompt(r.Context(), prompt)

	httputil.WriteJSON(w, http.StatusOK, title)
	return nil
}
