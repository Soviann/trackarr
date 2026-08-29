package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/service/matching"
	"github.com/go-chi/chi/v5"
)

type AdminSettingsHandler struct {
	writeDB  *sql.DB
	settings *repository.SettingRepository
	reloader *service.DynamicConfigReloader
}

func NewAdminSettingsHandler(
	writeDB *sql.DB,
	settings *repository.SettingRepository,
	reloader *service.DynamicConfigReloader,
) *AdminSettingsHandler {
	return &AdminSettingsHandler{
		writeDB:  writeDB,
		settings: settings,
		reloader: reloader,
	}
}

type SystemSettingsResponse struct {
	TMDBAPIKey            string `json:"tmdb_api_key"`
	TMDBConfigured        bool   `json:"tmdb_configured"`
	TVDBAPIKey            string `json:"tvdb_api_key"`
	TVDBConfigured        bool   `json:"tvdb_configured"`
	GeminiAPIKeys         string `json:"gemini_api_keys"`
	GeminiConfigured      bool   `json:"gemini_configured"`
	AniListClientID       string `json:"anilist_client_id"`
	AniListClientSecret   string `json:"anilist_client_secret"`
	AniListConfigured     bool   `json:"anilist_configured"`
	JellyfinWebhookSecret string `json:"jellyfin_webhook_secret"`
	JellyfinWebhookURL    string `json:"jellyfin_webhook_url"`
	PlexWebhookSecret     string `json:"plex_webhook_secret"`
	PlexWebhookURL        string `json:"plex_webhook_url"`
	RadarrURL             string `json:"radarr_url"`
	RadarrAPIKey          string `json:"radarr_api_key"`
	RadarrConfigured      bool   `json:"radarr_configured"`
	SonarrURL             string `json:"sonarr_url"`
	SonarrAPIKey          string `json:"sonarr_api_key"`
	SonarrConfigured      bool   `json:"sonarr_configured"`
	ProwlarrURL           string `json:"prowlarr_url"`
	ProwlarrAPIKey        string `json:"prowlarr_api_key"`
	ProwlarrConfigured    bool   `json:"prowlarr_configured"`
	VAPIDPublicKey        string `json:"vapid_public_key"`
	VAPIDSubject          string `json:"vapid_subject"`
	VAPIDConfigured       bool   `json:"vapid_configured"`
	MetadataLanguage      string `json:"metadata_language"`
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "••••••••"
	}
	return "••••••••" + s[len(s)-4:]
}

// GetSystemSettings returns the system configuration with masked secrets.
func (h *AdminSettingsHandler) GetSystemSettings(w http.ResponseWriter, r *http.Request) error {
	get := func(key string) string {
		if h.reloader != nil {
			return h.reloader.Get(key)
		}
		if h.settings != nil {
			val, _ := h.settings.Get(key)
			return val
		}
		return ""
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}

	jellyfinSecret := get("jellyfin_webhook_secret")
	plexSecret := get("plex_webhook_secret")

	jellyfinURL := ""
	if jellyfinSecret != "" {
		jellyfinURL = fmt.Sprintf("%s://%s/api/webhook/jellyfin/%s", scheme, host, jellyfinSecret)
	}

	plexURL := ""
	if plexSecret != "" {
		plexURL = fmt.Sprintf("%s://%s/api/webhook/plex/%s", scheme, host, plexSecret)
	}

	metadataLang := get("metadata_language")
	if metadataLang == "" {
		metadataLang = "fr"
	}

	resp := SystemSettingsResponse{
		TMDBAPIKey:            maskSecret(get("tmdb_api_key")),
		TMDBConfigured:        get("tmdb_api_key") != "",
		TVDBAPIKey:            maskSecret(get("tvdb_api_key")),
		TVDBConfigured:        get("tvdb_api_key") != "",
		GeminiAPIKeys:         maskSecret(get("gemini_api_keys")),
		GeminiConfigured:      get("gemini_api_keys") != "",
		AniListClientID:       get("anilist_client_id"),
		AniListClientSecret:   maskSecret(get("anilist_client_secret")),
		AniListConfigured:     get("anilist_client_id") != "",
		JellyfinWebhookSecret: jellyfinSecret,
		JellyfinWebhookURL:    jellyfinURL,
		PlexWebhookSecret:     plexSecret,
		PlexWebhookURL:        plexURL,
		RadarrURL:             get("radarr_url"),
		RadarrAPIKey:          maskSecret(get("radarr_api_key")),
		RadarrConfigured:      get("radarr_url") != "" && get("radarr_api_key") != "",
		SonarrURL:             get("sonarr_url"),
		SonarrAPIKey:          maskSecret(get("sonarr_api_key")),
		SonarrConfigured:      get("sonarr_url") != "" && get("sonarr_api_key") != "",
		ProwlarrURL:           get("prowlarr_url"),
		ProwlarrAPIKey:        maskSecret(get("prowlarr_api_key")),
		ProwlarrConfigured:    get("prowlarr_url") != "" && get("prowlarr_api_key") != "",
		VAPIDPublicKey:        get("vapid_public_key"),
		VAPIDSubject:          get("vapid_subject"),
		VAPIDConfigured:       get("vapid_public_key") != "",
		MetadataLanguage:      metadataLang,
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// UpdateSystemSettings updates system configuration in SQLite settings table and triggers hot reload.
func (h *AdminSettingsHandler) UpdateSystemSettings(w http.ResponseWriter, r *http.Request) error {
	if h.writeDB == nil {
		return httputil.InternalError("writeDB not configured", nil)
	}

	var req map[string]string
	if err := httputil.ReadJSON(r, &req, 64*1024); err != nil {
		return httputil.BadRequest("Invalid JSON body")
	}

	allowedKeys := map[string]bool{
		"tmdb_api_key":            true,
		"tvdb_api_key":            true,
		"gemini_api_keys":         true,
		"anilist_client_id":       true,
		"anilist_client_secret":   true,
		"jellyfin_webhook_secret": true,
		"plex_webhook_secret":     true,
		"webhook_secret":          true,
		"radarr_url":              true,
		"radarr_api_key":          true,
		"sonarr_url":              true,
		"sonarr_api_key":          true,
		"prowlarr_url":            true,
		"prowlarr_api_key":        true,
		"vapid_public_key":        true,
		"vapid_private_key":       true,
		"vapid_subject":           true,
		"metadata_language":       true,
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		writer := repository.NewSettingWriter(tx)
		for k, v := range req {
			if !allowedKeys[k] {
				continue
			}
			// Skip masked values (user did not modify the secret)
			if strings.HasPrefix(v, "••••") {
				continue
			}
			trimmed := strings.TrimSpace(v)
			if err := writer.Set(r.Context(), k, trimmed); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return httputil.InternalError("save settings", err)
	}

	// Trigger hot reload
	if h.reloader != nil {
		if err := h.reloader.Reload(r.Context()); err != nil {
			return httputil.InternalError("reload clients", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// TestTMDB validates the TMDB API key by calling the configuration endpoint.
func (h *AdminSettingsHandler) TestTMDB(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		APIKey string `json:"api_key"`
	}
	_ = httputil.ReadJSON(r, &body, 4096)

	key := strings.TrimSpace(body.APIKey)
	if key == "" || strings.HasPrefix(key, "••••") {
		if h.reloader != nil {
			key = h.reloader.Get("tmdb_api_key")
		}
	}
	if key == "" {
		return httputil.BadRequest("Clé d'API TMDB manquante")
	}

	client := matching.NewTMDBClient(key)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	results, err := client.SearchMovie(ctx, "Inception", 2010)
	if err != nil || len(results) == 0 {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("Échec de connexion TMDB: %v", err),
		})
		return nil
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Connexion TMDB réussie (clé valide)",
	})
	return nil
}

// TestTVDB validates the TVDB API key by attempting a v4 login.
func (h *AdminSettingsHandler) TestTVDB(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		APIKey string `json:"api_key"`
	}
	_ = httputil.ReadJSON(r, &body, 4096)

	key := strings.TrimSpace(body.APIKey)
	if key == "" || strings.HasPrefix(key, "••••") {
		if h.reloader != nil {
			key = h.reloader.Get("tvdb_api_key")
		}
	}
	if key == "" {
		return httputil.BadRequest("Clé d'API TheTVDB manquante")
	}

	client := matching.NewTVDBClient(key)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := client.Login(ctx); err != nil {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("Échec de connexion TheTVDB: %v", err),
		})
		return nil
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Connexion TheTVDB v4 réussie",
	})
	return nil
}

// TestGemini validates the Gemini API key(s).
func (h *AdminSettingsHandler) TestGemini(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		APIKeys string `json:"api_keys"`
	}
	_ = httputil.ReadJSON(r, &body, 4096)

	raw := strings.TrimSpace(body.APIKeys)
	if raw == "" || strings.HasPrefix(raw, "••••") {
		if h.reloader != nil {
			raw = h.reloader.Get("gemini_api_keys")
		}
	}
	if raw == "" {
		return httputil.BadRequest("Clé d'API Gemini manquante")
	}

	keys := strings.Split(raw, ",")
	var cleanKeys []string
	for _, k := range keys {
		if t := strings.TrimSpace(k); t != "" && !strings.HasPrefix(t, "••••") {
			cleanKeys = append(cleanKeys, t)
		}
	}
	if len(cleanKeys) == 0 {
		return httputil.BadRequest("Aucune clé valide fournie")
	}

	client := matching.NewGeminiClient(cleanKeys)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Lightweight verification test
	_, err := client.VerifyMatch(ctx,
		matching.PlexInfo{Title: "Inception", Year: 2010, Type: "movie"},
		matching.MatchCandidate{Title: "Inception", Year: 2010, TMDBID: 27205, IMDBID: "tt1375666"},
	)
	if err != nil {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("Échec de validation Gemini: %v", err),
		})
		return nil
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": fmt.Sprintf("Clé(s) Gemini validée(s) (%d clé(s) active(s))", len(cleanKeys)),
	})
	return nil
}

// TestArr validates connection to Radarr, Sonarr, or Prowlarr via /api/v3/system/status.
func (h *AdminSettingsHandler) TestArr(w http.ResponseWriter, r *http.Request) error {
	app := chi.URLParam(r, "app")
	if app != "radarr" && app != "sonarr" && app != "prowlarr" {
		return httputil.BadRequest("App inconnue (attendu radarr, sonarr ou prowlarr)")
	}

	var body struct {
		URL    string `json:"url"`
		APIKey string `json:"api_key"`
	}
	_ = httputil.ReadJSON(r, &body, 4096)

	baseURL := strings.TrimRight(strings.TrimSpace(body.URL), "/")
	apiKey := strings.TrimSpace(body.APIKey)

	if baseURL == "" && h.reloader != nil {
		baseURL = strings.TrimRight(strings.TrimSpace(h.reloader.Get(app+"_url")), "/")
	}
	if (apiKey == "" || strings.HasPrefix(apiKey, "••••")) && h.reloader != nil {
		apiKey = strings.TrimSpace(h.reloader.Get(app + "_api_key"))
	}

	if baseURL == "" || apiKey == "" {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("URL et clé d'API requises pour %s", app),
		})
		return nil
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	statusURL := fmt.Sprintf("%s/api/v3/system/status", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("URL invalide: %v", err),
		})
		return nil
	}
	req.Header.Set("X-Api-Key", apiKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("Échec de connexion à %s: %v", app, err),
		})
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("%s a retourné le code HTTP %d", app, resp.StatusCode),
		})
		return nil
	}

	var statusResp struct {
		AppName string `json:"appName"`
		Version string `json:"version"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&statusResp)

	msg := fmt.Sprintf("Connexion réussie à %s", app)
	if statusResp.Version != "" {
		msg += fmt.Sprintf(" (version %s)", statusResp.Version)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": msg,
		"version": statusResp.Version,
	})
	return nil
}

// GenerateVAPIDKeys generates a new pair of VAPID keys, saves them to SQLite, and triggers hot reload.
func (h *AdminSettingsHandler) GenerateVAPIDKeys(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Subject string `json:"subject"`
	}
	_ = httputil.ReadJSON(r, &body, 4096)

	subject := strings.TrimSpace(body.Subject)
	if subject == "" && h.reloader != nil {
		subject = h.reloader.Get("vapid_subject")
	}

	pubKey, _, sub, err := service.GenerateAndSaveVAPIDKeys(r.Context(), h.writeDB, subject)
	if err != nil {
		return httputil.InternalError("generate vapid keys", err)
	}

	if h.reloader != nil {
		_ = h.reloader.Reload(r.Context())
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"message":          "Nouvelles clés VAPID générées avec succès !",
		"vapid_public_key": pubKey,
		"vapid_subject":    sub,
	})
	return nil
}
