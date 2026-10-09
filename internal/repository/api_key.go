package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
)

type APIKeyRepository struct {
	db database.DBTX
}

func NewAPIKeyRepository(db database.DBTX) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func parseScopes(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	scopes := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			scopes = append(scopes, trimmed)
		}
	}
	return scopes
}

// List returns all API keys ordered by creation time descending.
func (r *APIKeyRepository) List(ctx context.Context) ([]*model.APIKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, key_hash, key_prefix, scopes, created_at, last_used_at, revoked_at
		 FROM api_keys
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*model.APIKey
	for rows.Next() {
		var k model.APIKey
		var scopesStr string
		var lastUsedAt, revokedAt sql.NullTime
		if err := rows.Scan(
			&k.ID, &k.Name, &k.KeyHash, &k.KeyPrefix, &scopesStr,
			&k.CreatedAt, &lastUsedAt, &revokedAt,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		k.Scopes = parseScopes(scopesStr)
		if lastUsedAt.Valid {
			k.LastUsedAt = &lastUsedAt.Time
		}
		if revokedAt.Valid {
			k.RevokedAt = &revokedAt.Time
		}
		keys = append(keys, &k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows api keys: %w", err)
	}
	return keys, nil
}

// GetByHash retrieves an active (non-revoked) API key by its SHA-256 hash.
func (r *APIKeyRepository) GetByHash(ctx context.Context, keyHash string) (*model.APIKey, error) {
	var k model.APIKey
	var scopesStr string
	var lastUsedAt, revokedAt sql.NullTime

	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, key_hash, key_prefix, scopes, created_at, last_used_at, revoked_at
		 FROM api_keys
		 WHERE key_hash = ? AND revoked_at IS NULL`,
		keyHash,
	).Scan(
		&k.ID, &k.Name, &k.KeyHash, &k.KeyPrefix, &scopesStr,
		&k.CreatedAt, &lastUsedAt, &revokedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}
	k.Scopes = parseScopes(scopesStr)
	if lastUsedAt.Valid {
		k.LastUsedAt = &lastUsedAt.Time
	}
	if revokedAt.Valid {
		k.RevokedAt = &revokedAt.Time
	}
	return &k, nil
}

// GetByID retrieves an API key by ID (including revoked keys).
func (r *APIKeyRepository) GetByID(ctx context.Context, id int64) (*model.APIKey, error) {
	var k model.APIKey
	var scopesStr string
	var lastUsedAt, revokedAt sql.NullTime

	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, key_hash, key_prefix, scopes, created_at, last_used_at, revoked_at
		 FROM api_keys
		 WHERE id = ?`,
		id,
	).Scan(
		&k.ID, &k.Name, &k.KeyHash, &k.KeyPrefix, &scopesStr,
		&k.CreatedAt, &lastUsedAt, &revokedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get api key by id: %w", err)
	}
	k.Scopes = parseScopes(scopesStr)
	if lastUsedAt.Valid {
		k.LastUsedAt = &lastUsedAt.Time
	}
	if revokedAt.Valid {
		k.RevokedAt = &revokedAt.Time
	}
	return &k, nil
}
