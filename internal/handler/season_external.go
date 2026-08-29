package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/go-chi/chi/v5"
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

func cleanAniListID(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "anilist.co/anime/"); idx != -1 {
		raw = raw[idx+len("anilist.co/anime/"):]
		if slashIdx := strings.Index(raw, "/"); slashIdx != -1 {
			raw = raw[:slashIdx]
		}
	}
	return strings.TrimSpace(raw)
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
	aniListID := cleanAniListID(body.AniListID)
	if aniListID == "" {
		return httputil.BadRequest("anilist_id required")
	}
	if _, err := strconv.ParseInt(aniListID, 10, 64); err != nil {
		return httputil.BadRequest("invalid anilist_id format")
	}
	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		if err := repository.NewSeasonExternalIDWriter(tx).Add(
			r.Context(), seasonID, repository.ProviderAniList, aniListID); err != nil {
			return err
		}
		// Ensure parent title is marked as anime
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE titles SET is_anime = 1 WHERE id = (SELECT title_id FROM seasons WHERE id = ?) AND is_anime = 0`,
			seasonID,
		); err != nil {
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
	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		return repository.NewSeasonExternalIDWriter(tx).DeletePart(
			r.Context(), seasonID, repository.ProviderAniList, externalID)
	}); err != nil {
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
	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		return repository.NewSeasonExternalIDWriter(tx).Reorder(
			r.Context(), seasonID, repository.ProviderAniList, body.OrderedIDs)
	}); err != nil {
		return httputil.InternalError("Internal error", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
