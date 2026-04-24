package service_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPushService(t *testing.T) (*service.PushService, *sql.DB, *repository.SettingRepository) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	svc := service.NewPushService(db, settings, "test-public-key", "test-private-key", "mailto:test@example.com")
	return svc, db, settings
}

func TestPushService_Subscribe(t *testing.T) {
	svc, _, settings := setupPushService(t)

	sub := `{"endpoint":"https://push.example.com/sub1","keys":{"p256dh":"key1","auth":"auth1"}}`
	err := svc.Subscribe(context.Background(), sub)
	require.NoError(t, err)

	// Subscription stored in settings
	stored, err := settings.Get("push_subscription")
	require.NoError(t, err)
	assert.Equal(t, sub, stored)
}

func TestPushService_Unsubscribe(t *testing.T) {
	svc, db, settings := setupPushService(t)

	// Seed a subscription.
	testutil.SetSetting(t, db, "push_subscription", `{"endpoint":"https://push.example.com/sub1"}`)

	err := svc.Unsubscribe(context.Background())
	require.NoError(t, err)

	// Subscription removed
	_, err = settings.Get("push_subscription")
	assert.Error(t, err) // Not found
}

func TestPushService_HasSubscription(t *testing.T) {
	svc, db, _ := setupPushService(t)

	assert.False(t, svc.HasSubscription())

	testutil.SetSetting(t, db, "push_subscription", `{"endpoint":"https://push.example.com/sub1"}`)
	assert.True(t, svc.HasSubscription())
}

func TestPushService_Subscribe_ValidatesJSON(t *testing.T) {
	svc, _, _ := setupPushService(t)

	err := svc.Subscribe(context.Background(), "not json")
	assert.Error(t, err)
}

func TestPushService_Subscribe_RequiresEndpoint(t *testing.T) {
	svc, _, _ := setupPushService(t)

	err := svc.Subscribe(context.Background(), `{"keys":{"p256dh":"key","auth":"auth"}}`)
	assert.Error(t, err)
}

func TestNoopNotifier(t *testing.T) {
	noop := service.NewNoopNotifier()
	assert.False(t, noop.HasSubscription())
	assert.NoError(t, noop.SendNotification(context.Background(), "title", "body", ""))
	assert.Error(t, noop.Subscribe(context.Background(), `{"endpoint":"https://example.com"}`))
	assert.Error(t, noop.Unsubscribe(context.Background()))
}
