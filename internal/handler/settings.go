package handler

import (
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/repository"
)

type SettingsHandler struct {
	settings *repository.SettingRepository
}

func NewSettingsHandler(settings *repository.SettingRepository) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	_, anilistErr := h.settings.Get("anilist_token")
	_, pushErr := h.settings.Get("push_subscription")

	writeJSON(w, map[string]bool{
		"anilist_connected": anilistErr == nil,
		"push_subscribed":   pushErr == nil,
	})
}
