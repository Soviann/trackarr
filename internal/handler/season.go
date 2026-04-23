package handler

import (
	"database/sql"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type SeasonHandler struct {
	db *sql.DB
}

func NewSeasonHandler(db *sql.DB) *SeasonHandler {
	return &SeasonHandler{db: db}
}

func (h *SeasonHandler) UpdateRating(w http.ResponseWriter, r *http.Request) error {
	seasonID, err := httputil.ParseIDParam(r, "seasonID")
	if err != nil {
		return httputil.BadRequest("Invalid season ID")
	}

	var body struct {
		Rating int `json:"rating"`
	}
	if err := httputil.ReadJSON(r, &body, 1024); err != nil {
		return httputil.BadRequest("Invalid request")
	}

	if err := database.WithTxContext(r.Context(), h.db, func(tx *sql.Tx) error {
		return repository.NewSeasonWriter(tx).UpdateRating(r.Context(), seasonID, body.Rating)
	}); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
