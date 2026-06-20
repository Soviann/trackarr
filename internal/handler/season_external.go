package handler

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
)

// SeasonExternalHandler manages external-ID mappings for individual seasons
// (currently AniList only). Supports adding parts, removing a specific part,
// and reordering parts.
type SeasonExternalHandler struct {
	writeDB *sql.DB
}

func NewSeasonExternalHandler(writeDB *sql.DB) *SeasonExternalHandler {
	return &SeasonExternalHandler{writeDB: writeDB}
}

// AddAniListID appends an AniList part to a season and enqueues a push.
// POST /api/titles/{titleID}/seasons/{seasonID}/anilist
func (h *SeasonExternalHandler) AddAniListID(w http.ResponseWriter, r *http.Request) error {
	seasonID, err := httputil.ParseIDParam(r, "seasonID")
	if err != nil {
		return httputil.BadRequest("Invalid season ID")
	}
	var body struct {
		AniListID string `json:"anilist_id"`
	}
	if err := httputil.ReadJSON(r, &body, 1024); err != nil {
		return httputil.BadRequest("Invalid request")
	}
	if body.AniListID == "" {
		return httputil.BadRequest("anilist_id required")
	}
	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		if err := repository.NewSeasonExternalIDWriter(tx).Add(
			r.Context(), seasonID, repository.ProviderAniList, body.AniListID); err != nil {
			return err
		}
		service.EnqueueAniListSeasonPush(r.Context(), tx, seasonID)
		return nil
	}); err != nil {
		return httputil.InternalError("Internal error", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// RemoveAniListID removes one AniList part from a season.
// DELETE /api/titles/{titleID}/seasons/{seasonID}/anilist/{externalID}
func (h *SeasonExternalHandler) RemoveAniListID(w http.ResponseWriter, r *http.Request) error {
	seasonID, err := httputil.ParseIDParam(r, "seasonID")
	if err != nil {
		return httputil.BadRequest("Invalid season ID")
	}
	externalID := chi.URLParam(r, "externalID")
	if externalID == "" {
		return httputil.BadRequest("external id required")
	}
	if err := repository.NewSeasonExternalIDRepository(h.writeDB).DeletePart(
		r.Context(), seasonID, repository.ProviderAniList, externalID); err != nil {
		return httputil.InternalError("Internal error", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ReorderAniList sets the explicit part order for a season.
// PUT /api/titles/{titleID}/seasons/{seasonID}/anilist/order
func (h *SeasonExternalHandler) ReorderAniList(w http.ResponseWriter, r *http.Request) error {
	seasonID, err := httputil.ParseIDParam(r, "seasonID")
	if err != nil {
		return httputil.BadRequest("Invalid season ID")
	}
	var body struct {
		OrderedIDs []string `json:"ordered_ids"`
	}
	if err := httputil.ReadJSON(r, &body, 4096); err != nil {
		return httputil.BadRequest("Invalid request")
	}
	if err := repository.NewSeasonExternalIDRepository(h.writeDB).Reorder(
		r.Context(), seasonID, repository.ProviderAniList, body.OrderedIDs); err != nil {
		return httputil.InternalError("Internal error", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
