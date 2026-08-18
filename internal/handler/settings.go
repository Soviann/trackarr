package handler

import (
	"net/http"
	"time"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type SettingsHandler struct {
	settings           *repository.SettingRepository
	eventRepo          *repository.WatchEventRepository
	tvdbReady          bool
	jellyfinConfigured bool
}

type SettingsResponse struct {
	AniListConnected     bool       `json:"anilist_connected"`
	AniListTokenInvalid  bool       `json:"anilist_token_invalid"`
	PushSubscribed       bool       `json:"push_subscribed"`
	TVDBConnected        bool       `json:"tvdb_connected"`
	JellyfinConfigured   bool       `json:"jellyfin_configured"`
	JellyfinLastScrobble *time.Time `json:"jellyfin_last_scrobble_at"`
}

func NewSettingsHandler(settings *repository.SettingRepository, eventRepo *repository.WatchEventRepository, tvdbReady bool, jellyfinConfigured bool) *SettingsHandler {
	return &SettingsHandler{
		settings:           settings,
		eventRepo:          eventRepo,
		tvdbReady:          tvdbReady,
		jellyfinConfigured: jellyfinConfigured,
	}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) error {
	_, anilistErr := h.settings.Get("anilist_token")
	_, pushErr := h.settings.Get("push_subscription")
	tokenInvalid, _ := h.settings.Get("anilist_token_invalid")

	var lastScrobble *time.Time
	if h.eventRepo != nil {
		if t, err := h.eventRepo.GetLatestCreatedAtBySource(model.WatchEventSourceJellyfin); err == nil {
			lastScrobble = t
		}
	}

	httputil.WriteJSON(w, http.StatusOK, SettingsResponse{
		AniListConnected:     anilistErr == nil,
		AniListTokenInvalid:  tokenInvalid == "true",
		PushSubscribed:       pushErr == nil,
		TVDBConnected:        h.tvdbReady,
		JellyfinConfigured:   h.jellyfinConfigured,
		JellyfinLastScrobble: lastScrobble,
	})
	return nil
}
