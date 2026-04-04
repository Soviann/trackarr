package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

const (
	settingKeyAniListToken = "anilist_token"
	anilistAuthorizeURL    = "https://anilist.co/api/v2/oauth/authorize"
)

type AniListAuthHandler struct {
	settings *repository.SettingRepository
	clientID string
}

func NewAniListAuthHandler(settings *repository.SettingRepository, clientID string) *AniListAuthHandler {
	return &AniListAuthHandler{settings: settings, clientID: clientID}
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
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil || body.Token == "" {
		return httputil.BadRequest("Invalid token")
	}

	if err := h.settings.Set(settingKeyAniListToken, body.Token); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Disconnect removes the stored AniList token.
func (h *AniListAuthHandler) Disconnect(w http.ResponseWriter, r *http.Request) error {
	if err := h.settings.Delete(settingKeyAniListToken); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
