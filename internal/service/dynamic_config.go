package service

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"sync"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
)

// DynamicConfigReloader manages thread-safe reloading of external service clients
// and webhooks when settings are modified dynamically via the admin UI.
type DynamicConfigReloader struct {
	mu             sync.RWMutex
	cfg            *config.Config
	writeDB        *sql.DB
	settings       *repository.SettingRepository
	pipeline       *matching.Pipeline
	bgSvc          *BackgroundService
	coverSvc       *CoverService
	worker         *TaskQueueWorker
	pushSvc        *PushService
	webhookSecrets func(jellyfin, plex, fallback string)
}

func NewDynamicConfigReloader(
	cfg *config.Config,
	writeDB *sql.DB,
	settings *repository.SettingRepository,
	pipeline *matching.Pipeline,
	bgSvc *BackgroundService,
	coverSvc *CoverService,
	worker *TaskQueueWorker,
	pushSvc *PushService,
	webhookSecrets func(jellyfin, plex, fallback string),
) *DynamicConfigReloader {
	return &DynamicConfigReloader{
		cfg:            cfg,
		writeDB:        writeDB,
		settings:       settings,
		pipeline:       pipeline,
		bgSvc:          bgSvc,
		coverSvc:       coverSvc,
		worker:         worker,
		pushSvc:        pushSvc,
		webhookSecrets: webhookSecrets,
	}
}

// Get returns the effective configuration value for a key (SQLite settings priority, env fallback).
func (d *DynamicConfigReloader) Get(key string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.settings != nil {
		if val, err := d.settings.Get(key); err == nil && val != "" {
			return val
		}
	}

	if d.cfg == nil {
		return ""
	}

	switch key {
	case "tmdb_api_key":
		return d.cfg.TMDBAPIKey
	case "tvdb_api_key":
		return d.cfg.TVDBAPIKey
	case "gemini_api_keys":
		return strings.Join(d.cfg.GeminiAPIKeys, ",")
	case "anilist_client_id":
		return d.cfg.AniListClientID
	case "anilist_client_secret":
		return d.cfg.AniListClientSecret
	case "jellyfin_webhook_secret":
		return d.cfg.JellyfinWebhookSecret
	case "plex_webhook_secret":
		return d.cfg.PlexWebhookSecret
	case "webhook_secret":
		return d.cfg.WebhookSecret
	case "radarr_url":
		return d.cfg.RadarrURL
	case "radarr_api_key":
		return d.cfg.RadarrAPIKey
	case "sonarr_url":
		return d.cfg.SonarrURL
	case "sonarr_api_key":
		return d.cfg.SonarrAPIKey
	case "prowlarr_url":
		return d.cfg.ProwlarrURL
	case "prowlarr_api_key":
		return d.cfg.ProwlarrAPIKey
	case "vapid_public_key":
		return d.cfg.VAPIDPublicKey
	case "vapid_private_key":
		return d.cfg.VAPIDPrivateKey
	case "vapid_subject":
		return d.cfg.VAPIDSubject
	default:
		return ""
	}
}

// Reload re-instantiates and injects updated external clients into all services at runtime.
func (d *DynamicConfigReloader) Reload(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tmdbKey := d.getUnsafe("tmdb_api_key")
	tvdbKey := d.getUnsafe("tvdb_api_key")
	geminiKeysRaw := d.getUnsafe("gemini_api_keys")
	jellyfinSecret := d.getUnsafe("jellyfin_webhook_secret")
	plexSecret := d.getUnsafe("plex_webhook_secret")
	fallbackSecret := d.getUnsafe("webhook_secret")

	// 1. TMDB Client
	var tmdbClient *matching.TMDBClient
	if tmdbKey != "" {
		tmdbClient = matching.NewTMDBClient(tmdbKey)
	}

	// 2. TVDB Client
	var tvdbClient *matching.TVDBClient
	if tvdbKey != "" {
		tvdbClient = matching.NewTVDBClient(tvdbKey)
		if err := tvdbClient.Login(ctx); err != nil {
			log.Printf("dynamic config: TVDB login failed on reload: %v", err)
		}
	}

	// 3. Gemini Client
	var geminiClient *matching.GeminiClient
	if geminiKeysRaw != "" {
		keys := strings.Split(geminiKeysRaw, ",")
		var cleanKeys []string
		for _, k := range keys {
			k = strings.TrimSpace(k)
			if k != "" {
				cleanKeys = append(cleanKeys, k)
			}
		}
		if len(cleanKeys) > 0 {
			geminiClient = matching.NewGeminiClient(cleanKeys)
		}
	}

	// 4. Update Pipeline
	if d.pipeline != nil {
		d.pipeline.SetTMDB(tmdbClient)
		d.pipeline.SetTVDB(tvdbClient)
		d.pipeline.SetGemini(geminiClient)
	}

	// 5. Update BackgroundService
	if d.bgSvc != nil {
		d.bgSvc.SetTMDB(tmdbClient)
		d.bgSvc.SetTVDB(tvdbClient)
	}

	// 6. Update CoverService
	if d.coverSvc != nil {
		d.coverSvc.SetTMDB(tmdbClient)
	}

	// 7. Update Worker
	if d.worker != nil {
		d.worker.SetTMDB(tmdbClient)
	}

	// 8. Update Webhook Secrets
	if d.webhookSecrets != nil {
		d.webhookSecrets(jellyfinSecret, plexSecret, fallbackSecret)
	}

	// 9. Update PushService Keys
	if d.pushSvc != nil {
		vapidPub := d.getUnsafe("vapid_public_key")
		vapidPriv := d.getUnsafe("vapid_private_key")
		vapidSub := d.getUnsafe("vapid_subject")
		d.pushSvc.SetKeys(vapidPub, vapidPriv, vapidSub)
	}

	log.Println("✅ DynamicConfig: Successfully reloaded external clients and secrets")
	return nil
}

func (d *DynamicConfigReloader) getUnsafe(key string) string {
	if d.settings != nil {
		if val, err := d.settings.Get(key); err == nil && val != "" {
			return val
		}
	}
	if d.cfg == nil {
		return ""
	}
	switch key {
	case "tmdb_api_key":
		return d.cfg.TMDBAPIKey
	case "tvdb_api_key":
		return d.cfg.TVDBAPIKey
	case "gemini_api_keys":
		return strings.Join(d.cfg.GeminiAPIKeys, ",")
	case "anilist_client_id":
		return d.cfg.AniListClientID
	case "anilist_client_secret":
		return d.cfg.AniListClientSecret
	case "jellyfin_webhook_secret":
		return d.cfg.JellyfinWebhookSecret
	case "plex_webhook_secret":
		return d.cfg.PlexWebhookSecret
	case "webhook_secret":
		return d.cfg.WebhookSecret
	case "radarr_url":
		return d.cfg.RadarrURL
	case "radarr_api_key":
		return d.cfg.RadarrAPIKey
	case "sonarr_url":
		return d.cfg.SonarrURL
	case "sonarr_api_key":
		return d.cfg.SonarrAPIKey
	case "prowlarr_url":
		return d.cfg.ProwlarrURL
	case "prowlarr_api_key":
		return d.cfg.ProwlarrAPIKey
	case "vapid_public_key":
		return d.cfg.VAPIDPublicKey
	case "vapid_private_key":
		return d.cfg.VAPIDPrivateKey
	case "vapid_subject":
		return d.cfg.VAPIDSubject
	default:
		return ""
	}
}
