package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyService_Lifecycle(t *testing.T) {
	writeDB, _, err := database.Open(":memory:")
	require.NoError(t, err)
	defer writeDB.Close()
	require.NoError(t, database.Migrate(writeDB))

	readRepo := repository.NewAPIKeyRepository(writeDB)
	svc := service.NewAPIKeyService(writeDB, readRepo)
	ctx := context.Background()

	// 1. Nominal: CreateKey, ValidateTokenFormat, AuthenticateToken, TouchLastUsed
	key, rawToken, err := svc.CreateKey(ctx, "Home Assistant", []string{model.ScopeLibraryRead, model.ScopeArrRead})
	require.NoError(t, err)
	require.NotNil(t, key)
	assert.True(t, strings.HasPrefix(rawToken, "trck_live_"))
	assert.Len(t, rawToken, 74)
	assert.Equal(t, rawToken[:12], key.KeyPrefix)
	assert.True(t, svc.ValidateTokenFormat(rawToken))

	// AuthenticateToken
	authKey, err := svc.AuthenticateToken(ctx, rawToken)
	require.NoError(t, err)
	require.NotNil(t, authKey)
	assert.Equal(t, key.ID, authKey.ID)
	assert.Equal(t, "Home Assistant", authKey.Name)

	// ListKeys
	keys, err := svc.ListKeys(ctx)
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	// RevokeKey
	err = svc.RevokeKey(ctx, key.ID)
	require.NoError(t, err)

	// Authenticating with revoked key fails
	_, err = svc.AuthenticateToken(ctx, rawToken)
	assert.ErrorIs(t, err, service.ErrKeyNotFound)

	// 2. Boundary: Invalid token format, non-existent token, scope boundaries
	assert.False(t, svc.ValidateTokenFormat("trck_test_123"))
	assert.False(t, svc.ValidateTokenFormat("trck_live_short"))
	assert.False(t, svc.ValidateTokenFormat("trck_live_"+strings.Repeat("g", 64))) // non-hex

	_, err = svc.AuthenticateToken(ctx, "trck_live_"+strings.Repeat("a", 64))
	assert.ErrorIs(t, err, service.ErrKeyNotFound)

	// Empty name
	_, _, err = svc.CreateKey(ctx, "   ", []string{model.ScopeLibraryRead})
	assert.ErrorIs(t, err, service.ErrEmptyKeyName)

	// Name too long
	_, _, err = svc.CreateKey(ctx, strings.Repeat("x", 101), []string{model.ScopeLibraryRead})
	assert.Error(t, err)

	// 3. Error: Invalid scopes, empty scopes, delete
	_, _, err = svc.CreateKey(ctx, "Test", []string{"invalid:scope"})
	assert.ErrorIs(t, err, service.ErrInvalidScope)

	_, _, err = svc.CreateKey(ctx, "Test", []string{})
	assert.Error(t, err)

	// Delete key
	key2, _, err := svc.CreateKey(ctx, "Temporary Key", []string{model.ScopeLibraryWrite})
	require.NoError(t, err)
	err = svc.DeleteKey(ctx, key2.ID)
	require.NoError(t, err)

	// Deleting again fails
	err = svc.DeleteKey(ctx, key2.ID)
	assert.Error(t, err)

	// Throttling test: Multiple TouchLastUsed calls don't panic
	svc.TouchLastUsed(key.ID)
	svc.TouchLastUsed(key.ID)
	time.Sleep(50 * time.Millisecond)
}
