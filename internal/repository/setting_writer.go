package repository

import (
	"context"
	"database/sql"
	"fmt"
)

// SettingWriter performs write operations on the settings key-value store
// within a caller-owned transaction. Accepting only *sql.Tx makes "write to
// the pool without a transaction" a compile-time error — consistent with the
// rest of the repository layer.
type SettingWriter struct {
	tx *sql.Tx
}

func NewSettingWriter(tx *sql.Tx) *SettingWriter {
	return &SettingWriter{tx: tx}
}

// Set inserts or updates a setting value for the given key.
func (w *SettingWriter) Set(ctx context.Context, key, value string) error {
	if _, err := w.tx.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	); err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}

// Delete removes the setting with the given key.
func (w *SettingWriter) Delete(ctx context.Context, key string) error {
	if _, err := w.tx.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key); err != nil {
		return fmt.Errorf("delete setting %s: %w", key, err)
	}
	return nil
}
