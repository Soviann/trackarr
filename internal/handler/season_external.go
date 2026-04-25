package handler

import (
	"database/sql"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
)

// SeasonExternalHandler manages external-ID mappings for individual seasons
// (currently AniList only). PUT upserts the mapping and enqueues a push task;
// DELETE removes the mapping without enqueuing anything.
type SeasonExternalHandler struct {
	writeDB *sql.DB
}

func NewSeasonExternalHandler(writeDB *sql.DB) *SeasonExternalHandler {
	return &SeasonExternalHandler{writeDB: writeDB}
}

// SetAniListID upserts the AniList ID for a season and enqueues a push task.
// PUT /api/titles/{titleID}/seasons/{seasonID}/anilist
func (h *SeasonExternalHandler) SetAniListID(w http.ResponseWriter, r *http.Request) error {
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
		if err := repository.NewSeasonExternalIDWriter(tx).Upsert(
			r.Context(), seasonID, repository.ProviderAniList, body.AniListID,
		); err != nil {
			return err
		}
		service.EnqueueAniListSeasonPush(r.Context(), tx, seasonID)
		return nil
	}); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

// ClearAniListID removes the AniList ID mapping for a season.
// DELETE /api/titles/{titleID}/seasons/{seasonID}/anilist
func (h *SeasonExternalHandler) ClearAniListID(w http.ResponseWriter, r *http.Request) error {
	seasonID, err := httputil.ParseIDParam(r, "seasonID")
	if err != nil {
		return httputil.BadRequest("Invalid season ID")
	}

	// Pas de transaction : un DELETE seul n'a pas de side-effect à coordonner
	// avec un enqueue (au contraire du PUT). Une exécution sur le pool suffit.
	if err := repository.NewSeasonExternalIDRepository(h.writeDB).Delete(
		r.Context(), seasonID, repository.ProviderAniList,
	); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
