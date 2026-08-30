package handler

import (
	"net/http"
	"time"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

type SettingsHandler struct {
	settings           *repository.SettingRepository
	eventRepo          *repository.WatchEventRepository
	tvdbReady          bool
	jellyfinConfigured bool
	prowlarrSvc        prowlarrChecker
}

type prowlarrChecker interface {
	IsConfigured() bool
}

type SettingsResponse struct {
	AniListConnected       bool       `json:"anilist_connected"`
	AniListTokenInvalid    bool       `json:"anilist_token_invalid"`
	PushSubscribed         bool       `json:"push_subscribed"`
	TVDBConnected          bool       `json:"tvdb_connected"`
	JellyfinConfigured     bool       `json:"jellyfin_configured"`
	ProwlarrConfigured     bool       `json:"prowlarr_configured"`
	JellyfinLastScrobble   *time.Time `json:"jellyfin_last_scrobble_at"`
	EnabledWatchProviders string     `json:"enabled_watch_providers,omitempty"`
}

func NewSettingsHandler(settings *repository.SettingRepository, eventRepo *repository.WatchEventRepository, tvdbReady bool, jellyfinConfigured bool, prowlarrSvc ...prowlarrChecker) *SettingsHandler {
	var ps prowlarrChecker
	if len(prowlarrSvc) > 0 {
		ps = prowlarrSvc[0]
	}
	return &SettingsHandler{
		settings:           settings,
		eventRepo:          eventRepo,
		tvdbReady:          tvdbReady,
		jellyfinConfigured: jellyfinConfigured,
		prowlarrSvc:        ps,
	}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) error {
	_, anilistErr := h.settings.Get(repository.SettingKeyAniListToken)
	_, pushErr := h.settings.Get(repository.SettingKeyPushSubscription)
	tokenInvalid, _ := h.settings.Get(repository.SettingKeyAniListTokenInvalid)

	var lastScrobble *time.Time
	if h.eventRepo != nil {
		if t, err := h.eventRepo.GetLatestCreatedAtBySource(model.WatchEventSourceJellyfin); err == nil {
			lastScrobble = t
		}
	}

	prowlarrConfigured := false
	if h.prowlarrSvc != nil {
		prowlarrConfigured = h.prowlarrSvc.IsConfigured()
	}

	enabledWP := "netflix,prime,disney,apple,max,canal,crunchyroll,paramount,adn"
	if h.settings != nil {
		if wp, err := h.settings.Get(repository.SettingKeyEnabledWatchProviders); err == nil && wp != "" {
			enabledWP = wp
		}
	}

	httputil.WriteJSON(w, http.StatusOK, SettingsResponse{
		AniListConnected:       anilistErr == nil,
		AniListTokenInvalid:    tokenInvalid == "true",
		PushSubscribed:         pushErr == nil,
		TVDBConnected:          h.tvdbReady,
		JellyfinConfigured:     h.jellyfinConfigured,
		ProwlarrConfigured:     prowlarrConfigured,
		JellyfinLastScrobble:   lastScrobble,
		EnabledWatchProviders: enabledWP,
	})
	return nil
}
