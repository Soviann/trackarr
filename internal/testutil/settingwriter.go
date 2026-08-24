package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/stretchr/testify/require"
)

// SetSetting writes a settings key/value in a fresh transaction.
func SetSetting(t *testing.T, db *sql.DB, key, value string) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewSettingWriter(tx).Set(context.Background(), key, value)
	}))
}

// DeleteSetting removes a settings key in a fresh transaction.
func DeleteSetting(t *testing.T, db *sql.DB, key string) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewSettingWriter(tx).Delete(context.Background(), key)
	}))
}

// GetSetting reads the raw settings value; returns ("", error) when the key
// does not exist (mirrors SettingRepository.Get).
func GetSetting(t *testing.T, db *sql.DB, key string) (string, error) {
	t.Helper()
	return repository.NewSettingRepository(db).Get(key)
}
