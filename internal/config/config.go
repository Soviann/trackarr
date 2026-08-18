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
	RadarrURL              string
	RadarrAPIKey           string
	SonarrURL              string
	SonarrAPIKey           string
	DebugLogin             bool
	DebugLoginUser         string
	DebugLoginPassword     string
	DisableBackgroundTasks bool
}

func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:             envOr("LISTEN_ADDR", ":8080"),
		DataDir:                envOr("DATA_DIR", "/data"),
		GoogleClientID:         os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleAllowedEmail:     os.Getenv("GOOGLE_ALLOWED_EMAIL"),
		JWTSecret:              os.Getenv("JWT_SECRET"),
		TMDBAPIKey:             os.Getenv("TMDB_API_KEY"),
		TVDBAPIKey:             os.Getenv("TVDB_API_KEY"),
		AniListClientID:        os.Getenv("ANILIST_CLIENT_ID"),
		AniListClientSecret:    os.Getenv("ANILIST_CLIENT_SECRET"),
		VAPIDPublicKey:         os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey:        os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:           os.Getenv("VAPID_SUBJECT"),
		JellyfinWebhookSecret:  os.Getenv("JELLYFIN_WEBHOOK_SECRET"),
		RadarrURL:              os.Getenv("RADARR_URL"),
		RadarrAPIKey:           os.Getenv("RADARR_API_KEY"),
		SonarrURL:              os.Getenv("SONARR_URL"),
		SonarrAPIKey:           os.Getenv("SONARR_API_KEY"),
		DisableBackgroundTasks: os.Getenv("DISABLE_BACKGROUND_TASKS") == "true",
	}

	if keys := os.Getenv("GEMINI_API_KEY"); keys != "" {
		cfg.GeminiAPIKeys = strings.Split(keys, ",")
	}

	cfg.DebugLogin = os.Getenv("DEBUG_LOGIN") == "true"
	cfg.DebugLoginUser = envOr("DEBUG_LOGIN_USER", "")
	cfg.DebugLoginPassword = envOr("DEBUG_LOGIN_PASSWORD", "")
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		cfg.CookieSecure = v == "true"
	} else {
		cfg.CookieSecure = !cfg.DebugLogin
	}

	if cfg.GoogleClientID == "" || cfg.GoogleAllowedEmail == "" || cfg.JWTSecret == "" {
		return nil, fmt.Errorf("required env vars: GOOGLE_CLIENT_ID, GOOGLE_ALLOWED_EMAIL, JWT_SECRET")
	}

	// JWT_SECRET integrity is required unconditionally. Previously the default-secret
	// check was gated on !DebugLogin, which meant a deploy that shipped DEBUG_LOGIN=true
	// (e.g. .env mis-copied) silently accepted "dev-secret-change-me" → trivial token forge.
	// 32 bytes is the HMAC-SHA256 block size — shorter keys reduce signature strength.
	if cfg.JWTSecret == "dev-secret-change-me" {
		return nil, fmt.Errorf("JWT_SECRET must be changed from the default value")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters (HMAC-SHA256 strength)")
	}

	// CookieSecure=true is the prod marker (HTTPS behind reverse proxy). DebugLogin=true
	// in that environment is a config copy-paste accident — refuse to boot rather than
	// expose /api/auth/dev with hardcoded creds in prod.
	if cfg.DebugLogin && cfg.CookieSecure {
		return nil, fmt.Errorf("DEBUG_LOGIN=true is incompatible with COOKIE_SECURE=true (prod env)")
	}

	if cfg.CookieSecure && cfg.JellyfinWebhookSecret == "" {
		return nil, fmt.Errorf("JELLYFIN_WEBHOOK_SECRET is required in production (COOKIE_SECURE=true)")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
