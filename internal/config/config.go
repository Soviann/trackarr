package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	ListenAddr          string
	DataDir             string
	GoogleClientID      string
	GoogleAllowedEmail  string
	JWTSecret           string
	TMDBAPIKey          string
	AniListClientID     string
	AniListClientSecret string
	GeminiAPIKeys       []string // Rotation pool
	VAPIDPublicKey      string
	VAPIDPrivateKey     string
	VAPIDSubject        string
}

func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:          envOr("LISTEN_ADDR", ":8080"),
		DataDir:             envOr("DATA_DIR", "/data"),
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleAllowedEmail:  os.Getenv("GOOGLE_ALLOWED_EMAIL"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		TMDBAPIKey:          os.Getenv("TMDB_API_KEY"),
		AniListClientID:     os.Getenv("ANILIST_CLIENT_ID"),
		AniListClientSecret: os.Getenv("ANILIST_CLIENT_SECRET"),
		VAPIDPublicKey:      os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey:     os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:        os.Getenv("VAPID_SUBJECT"),
	}

	if keys := os.Getenv("GEMINI_API_KEY"); keys != "" {
		cfg.GeminiAPIKeys = strings.Split(keys, ",")
	}

	if cfg.GoogleClientID == "" || cfg.GoogleAllowedEmail == "" || cfg.JWTSecret == "" {
		return nil, fmt.Errorf("required env vars: GOOGLE_CLIENT_ID, GOOGLE_ALLOWED_EMAIL, JWT_SECRET")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
