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
	PlexWebhookSecret      string
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
		PlexWebhookSecret:      os.Getenv("PLEX_WEBHOOK_SECRET"),
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

	if cfg.JWTSecret == "dev-secret-change-me" && !cfg.DebugLogin {
		return nil, fmt.Errorf("JWT_SECRET must be changed from default for production use")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
