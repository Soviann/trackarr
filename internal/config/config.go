package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	ListenAddr             string
	DataDir                string
	GoogleClientID         string
	GoogleAllowedEmail     string
	JWTSecret              string
	CookieSecure           bool
	TMDBAPIKey             string
	AniListClientID        string
	AniListClientSecret    string
	TVDBAPIKey             string
	GeminiAPIKeys          []string // Rotation pool
	VAPIDPublicKey         string
	VAPIDPrivateKey        string
	VAPIDSubject           string
	JellyfinWebhookSecret  string
	PlexWebhookSecret      string
	WebhookSecret          string
	RadarrURL              string
	RadarrAPIKey           string
	SonarrURL              string
	SonarrAPIKey           string
	ProwlarrURL            string
	ProwlarrAPIKey         string
	DisableBackgroundTasks bool
}

func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:             envOr("LISTEN_ADDR", ":8080"),
		DataDir:                envOr("DATA_DIR", "/data"),
		GoogleClientID:         os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleAllowedEmail:     os.Getenv("GOOGLE_ALLOWED_EMAIL"),
		JWTSecret:              os.Getenv("JWT_SECRET"),
		CookieSecure:           os.Getenv("COOKIE_SECURE") == "true",
		TMDBAPIKey:             os.Getenv("TMDB_API_KEY"),
		TVDBAPIKey:             os.Getenv("TVDB_API_KEY"),
		AniListClientID:        os.Getenv("ANILIST_CLIENT_ID"),
		AniListClientSecret:    os.Getenv("ANILIST_CLIENT_SECRET"),
		VAPIDPublicKey:         os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey:        os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:           os.Getenv("VAPID_SUBJECT"),
		JellyfinWebhookSecret:  os.Getenv("JELLYFIN_WEBHOOK_SECRET"),
		PlexWebhookSecret:      os.Getenv("PLEX_WEBHOOK_SECRET"),
		WebhookSecret:          os.Getenv("WEBHOOK_SECRET"),
		RadarrURL:              os.Getenv("RADARR_URL"),
		RadarrAPIKey:           os.Getenv("RADARR_API_KEY"),
		SonarrURL:              os.Getenv("SONARR_URL"),
		SonarrAPIKey:           os.Getenv("SONARR_API_KEY"),
		ProwlarrURL:            os.Getenv("PROWLARR_URL"),
		ProwlarrAPIKey:         os.Getenv("PROWLARR_API_KEY"),
		DisableBackgroundTasks: os.Getenv("DISABLE_BACKGROUND_TASKS") == "true",
	}

	if keys := os.Getenv("GEMINI_API_KEY"); keys != "" {
		cfg.GeminiAPIKeys = strings.Split(keys, ",")
	}

	if (cfg.GoogleClientID != "" && cfg.GoogleAllowedEmail == "") || (cfg.GoogleClientID == "" && cfg.GoogleAllowedEmail != "") {
		return nil, fmt.Errorf("both GOOGLE_CLIENT_ID and GOOGLE_ALLOWED_EMAIL must be provided if Google OAuth is configured")
	}

	// JWT_SECRET integrity is required if provided via env.
	// If not provided in env, it will be automatically loaded/generated from SQLite settings.
	// 32 bytes is the HMAC-SHA256 block size — shorter keys reduce signature strength.
	if cfg.JWTSecret != "" {
		if cfg.JWTSecret == "dev-secret-change-me" {
			return nil, fmt.Errorf("JWT_SECRET must be changed from the default value")
		}
		if len(cfg.JWTSecret) < 32 {
			return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters (HMAC-SHA256 strength)")
		}
	}

	if cfg.CookieSecure && cfg.JellyfinWebhookSecret == "" && cfg.PlexWebhookSecret == "" && cfg.WebhookSecret == "" {
		return nil, fmt.Errorf("a webhook secret (JELLYFIN_WEBHOOK_SECRET, PLEX_WEBHOOK_SECRET or WEBHOOK_SECRET) is required in production (COOKIE_SECURE=true)")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
