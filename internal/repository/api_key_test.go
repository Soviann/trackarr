package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyRepository_And_Writer(t *testing.T) {
	writeDB, _, err := database.Open(":memory:")
	require.NoError(t, err)
	defer writeDB.Close()
	require.NoError(t, database.Migrate(writeDB))

	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(writeDB)

	// 1. Nominal: Create, List, GetByHash, GetByID
	var key1 *model.APIKey
	err = database.WithTx(writeDB, func(tx *sql.Tx) error {
		w := repository.NewAPIKeyWriter(tx)
		var err error
		key1, err = w.Create(ctx, "Test Key 1", "hash1", "trck_live_01", []string{model.ScopeLibraryRead, model.ScopeLibraryWrite})
		return err
	})
	require.NoError(t, err)
	require.NotNil(t, key1)
	assert.Equal(t, "Test Key 1", key1.Name)
	assert.Equal(t, "trck_live_01", key1.KeyPrefix)
	assert.Equal(t, []string{model.ScopeLibraryRead, model.ScopeLibraryWrite}, key1.Scopes)
	assert.True(t, key1.HasScope(model.ScopeLibraryRead))
	assert.True(t, key1.HasScope(model.ScopeLibraryWrite))
	assert.False(t, key1.HasScope(model.ScopeLibraryDelete))

	// GetByHash
	found, err := repo.GetByHash(ctx, "hash1")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, key1.ID, found.ID)
	assert.Equal(t, "Test Key 1", found.Name)

	// GetByID
	foundByID, err := repo.GetByID(ctx, key1.ID)
	require.NoError(t, err)
	require.NotNil(t, foundByID)
	assert.Equal(t, key1.ID, foundByID.ID)

	// UpdateLastUsedAt
	now := time.Now().UTC().Truncate(time.Second)
	err = database.WithTx(writeDB, func(tx *sql.Tx) error {
		return repository.NewAPIKeyWriter(tx).UpdateLastUsedAt(ctx, key1.ID, now)
	})
	require.NoError(t, err)

	foundAfterTouch, err := repo.GetByID(ctx, key1.ID)
	require.NoError(t, err)
	require.NotNil(t, foundAfterTouch.LastUsedAt)

	// 2. Boundary: Empty scopes, non-existent hash, non-existent ID
	var key2 *model.APIKey
	err = database.WithTx(writeDB, func(tx *sql.Tx) error {
		w := repository.NewAPIKeyWriter(tx)
		var err error
		key2, err = w.Create(ctx, "Key No Scopes", "hash2", "trck_live_02", []string{})
		return err
	})
	require.NoError(t, err)
	assert.Empty(t, key2.Scopes)

	notFoundByHash, err := repo.GetByHash(ctx, "non_existent_hash")
	require.NoError(t, err)
	assert.Nil(t, notFoundByHash)

	notFoundByID, err := repo.GetByID(ctx, 999999)
	require.NoError(t, err)
	assert.Nil(t, notFoundByID)

	// List
	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// 3. Error / Revoke & Delete: Duplicate hash error, Revoke active key, Deleted key
	err = database.WithTx(writeDB, func(tx *sql.Tx) error {
		w := repository.NewAPIKeyWriter(tx)
		_, err := w.Create(ctx, "Duplicate Key", "hash1", "trck_live_01", []string{model.ScopeLibraryRead})
		return err
	})
	assert.Error(t, err, "duplicate key_hash must violate unique constraint")

	// Revoke key1
	err = database.WithTx(writeDB, func(tx *sql.Tx) error {
		return repository.NewAPIKeyWriter(tx).Revoke(ctx, key1.ID)
	})
	require.NoError(t, err)

	// Revoked key cannot be found by GetByHash
	revokedLookup, err := repo.GetByHash(ctx, "hash1")
	require.NoError(t, err)
	assert.Nil(t, revokedLookup)

	// Revoking again fails
	err = database.WithTx(writeDB, func(tx *sql.Tx) error {
		return repository.NewAPIKeyWriter(tx).Revoke(ctx, key1.ID)
	})
	assert.Error(t, err)

	// Delete key2
	err = database.WithTx(writeDB, func(tx *sql.Tx) error {
		return repository.NewAPIKeyWriter(tx).Delete(ctx, key2.ID)
	})
	require.NoError(t, err)

	deletedLookup, err := repo.GetByID(ctx, key2.ID)
	require.NoError(t, err)
	assert.Nil(t, deletedLookup)

	// Deleting non-existent key errors
	err = database.WithTx(writeDB, func(tx *sql.Tx) error {
		return repository.NewAPIKeyWriter(tx).Delete(ctx, 999999)
	})
	assert.Error(t, err)
}
