package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

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
func (h *AniListAuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	if h.clientID == "" {
		http.Error(w, "AniList not configured", http.StatusServiceUnavailable)
		return
	}

	url := anilistAuthorizeURL + "?client_id=" + h.clientID + "&response_type=token"
	http.Redirect(w, r, url, http.StatusFound)
}

// SaveToken stores the access token received from the frontend.
func (h *AniListAuthHandler) SaveToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil || body.Token == "" {
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}

	if err := h.settings.Set(settingKeyAniListToken, body.Token); err != nil {
		log.Printf("anilist: save token: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Disconnect removes the stored AniList token.
func (h *AniListAuthHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	if err := h.settings.Delete(settingKeyAniListToken); err != nil {
		log.Printf("anilist: disconnect: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
