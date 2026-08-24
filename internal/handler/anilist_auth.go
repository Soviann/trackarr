package handler

import (
	"database/sql"
	"net/http"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
)

const anilistAuthorizeURL = "https://anilist.co/api/v2/oauth/authorize"

type AniListAuthHandler struct {
	writeDB  *sql.DB
	settings *repository.SettingRepository
	clientID string
}

func NewAniListAuthHandler(writeDB *sql.DB, settings *repository.SettingRepository, clientID string) *AniListAuthHandler {
	return &AniListAuthHandler{writeDB: writeDB, settings: settings, clientID: clientID}
}

// Authorize redirects to AniList OAuth page (implicit grant).
func (h *AniListAuthHandler) Authorize(w http.ResponseWriter, r *http.Request) error {
	if h.clientID == "" {
		return httputil.NewAPIError(http.StatusServiceUnavailable, "AniList not configured")
	}

	url := anilistAuthorizeURL + "?client_id=" + h.clientID + "&response_type=token"
	http.Redirect(w, r, url, http.StatusFound)
	return nil
}

// SaveToken stores the access token received from the frontend.
func (h *AniListAuthHandler) SaveToken(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Token string `json:"token"`
	}
	if err := httputil.ReadJSON(r, &body, 4096); err != nil || body.Token == "" {
		return httputil.BadRequest("Invalid token")
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		writer := repository.NewSettingWriter(tx)
		if err := writer.Set(r.Context(), repository.SettingKeyAniListToken, body.Token); err != nil {
			return err
		}
		// Un nouveau token est par définition valide : efface tout drapeau
		// d'invalidation laissé par un push 401.
		return writer.Delete(r.Context(), repository.SettingKeyAniListTokenInvalid)
	}); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Disconnect removes the stored AniList token.
func (h *AniListAuthHandler) Disconnect(w http.ResponseWriter, r *http.Request) error {
	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		return repository.NewSettingWriter(tx).Delete(r.Context(), repository.SettingKeyAniListToken)
	}); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
