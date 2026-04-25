package handler

import (
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type SettingsHandler struct {
	settings  *repository.SettingRepository
	tvdbReady bool
}

func NewSettingsHandler(settings *repository.SettingRepository, tvdbReady bool) *SettingsHandler {
	return &SettingsHandler{settings: settings, tvdbReady: tvdbReady}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) error {
	_, anilistErr := h.settings.Get("anilist_token")
	_, pushErr := h.settings.Get("push_subscription")
	tokenInvalid, _ := h.settings.Get("anilist_token_invalid")

	httputil.WriteJSON(w, http.StatusOK, map[string]bool{
		"anilist_connected":     anilistErr == nil,
		"anilist_token_invalid": tokenInvalid == "true",
		"push_subscribed":       pushErr == nil,
		"tvdb_connected":        h.tvdbReady,
	})
	return nil
}
