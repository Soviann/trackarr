package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/model"
)

// APIKeyWriter performs transactional write operations on api_keys.
type APIKeyWriter struct {
	tx *sql.Tx
}

func NewAPIKeyWriter(tx *sql.Tx) *APIKeyWriter {
	return &APIKeyWriter{tx: tx}
}

// Create inserts a new API key record.
func (w *APIKeyWriter) Create(ctx context.Context, name, keyHash, keyPrefix string, scopes []string) (*model.APIKey, error) {
	scopesStr := strings.Join(scopes, ",")
	res, err := w.tx.ExecContext(ctx,
		`INSERT INTO api_keys (name, key_hash, key_prefix, scopes) VALUES (?, ?, ?, ?)`,
		name, keyHash, keyPrefix, scopesStr,
	)
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get api key last insert id: %w", err)
	}

	return &model.APIKey{
		ID:        id,
		Name:      name,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Scopes:    scopes,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// Revoke soft-deletes/revokes an API key by setting revoked_at.
func (w *APIKeyWriter) Revoke(ctx context.Context, id int64) error {
	res, err := w.tx.ExecContext(ctx,
		`UPDATE api_keys SET revoked_at = CURRENT_TIMESTAMP WHERE id = ? AND revoked_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected revoke api key: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("api key not found or already revoked")
	}
	return nil
}

// Delete permanently removes an API key record.
func (w *APIKeyWriter) Delete(ctx context.Context, id int64) error {
	res, err := w.tx.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected delete api key: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("api key not found")
	}
	return nil
}

// UpdateLastUsedAt updates the last_used_at timestamp.
func (w *APIKeyWriter) UpdateLastUsedAt(ctx context.Context, id int64, t time.Time) error {
	_, err := w.tx.ExecContext(ctx, `UPDATE api_keys SET last_used_at = ? WHERE id = ?`, t, id)
	if err != nil {
		return fmt.Errorf("update api key last_used_at: %w", err)
	}
	return nil
}
