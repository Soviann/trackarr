package repository

import (
	"fmt"

	"github.com/Soviann/trackarr/internal/database"
)

// Canonical settings keys.
const (
	SettingKeyAniListToken          = "anilist_token"
	SettingKeyAniListTokenInvalid   = "anilist_token_invalid"
	SettingKeyPushSubscription      = "push_subscription"
	SettingKeyAdminUsername         = "admin_username"
	SettingKeyAdminPasswordHash     = "admin_password_hash"
	SettingKeyAdminRecoveryKeyHash  = "admin_recovery_key_hash"
	SettingKeyAuthMode              = "auth_mode"
	SettingKeyJWTSecret             = "jwt_secret"
	SettingKeyVAPIDPublicKey        = "vapid_public_key"
	SettingKeyVAPIDPrivateKey       = "vapid_private_key"
	SettingKeyVAPIDSubject          = "vapid_subject"
	SettingKeyTMDBAPIKey            = "tmdb_api_key"
	SettingKeyTVDBAPIKey            = "tvdb_api_key"
	SettingKeyGeminiAPIKeys         = "gemini_api_keys"
	SettingKeyAniListClientID       = "anilist_client_id"
	SettingKeyAniListClientSecret   = "anilist_client_secret"
	SettingKeyJellyfinWebhookSecret = "jellyfin_webhook_secret"
	SettingKeyPlexWebhookSecret     = "plex_webhook_secret"
	SettingKeyWebhookSecret         = "webhook_secret"
	SettingKeyRadarrURL             = "radarr_url"
	SettingKeyRadarrAPIKey          = "radarr_api_key"
	SettingKeySonarrURL             = "sonarr_url"
	SettingKeySonarrAPIKey          = "sonarr_api_key"
	SettingKeyProwlarrURL           = "prowlarr_url"
	SettingKeyProwlarrAPIKey        = "prowlarr_api_key"
	SettingKeyMetadataLanguage      = "metadata_language"
	SettingKeyEnabledWatchProviders = "enabled_watch_providers"
)

// SettingRepository reads the settings key-value store. Writes live on
// SettingWriter, which requires a *sql.Tx.
type SettingRepository struct {
	db database.DBTX
}

func NewSettingRepository(db database.DBTX) *SettingRepository {
	return &SettingRepository{db: db}
}

func (r *SettingRepository) Get(key string) (string, error) {
	var value string
	err := r.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("get setting %s: %w", key, err)
	}
	return value, nil
}
