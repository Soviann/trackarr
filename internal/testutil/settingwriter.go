package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
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
