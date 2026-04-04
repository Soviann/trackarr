package service_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPushService(t *testing.T) (*service.PushService, *repository.SettingRepository) {
	t.Helper()
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	svc := service.NewPushService(settings, "test-public-key", "test-private-key", "mailto:test@example.com")
	return svc, settings
}

func TestPushService_Subscribe(t *testing.T) {
	svc, settings := setupPushService(t)

	sub := `{"endpoint":"https://push.example.com/sub1","keys":{"p256dh":"key1","auth":"auth1"}}`
	err := svc.Subscribe(sub)
	require.NoError(t, err)

	// Subscription stored in settings
	stored, err := settings.Get("push_subscription")
	require.NoError(t, err)
	assert.Equal(t, sub, stored)
}

func TestPushService_Unsubscribe(t *testing.T) {
	svc, settings := setupPushService(t)

	// Store a subscription first
	_ = settings.Set("push_subscription", `{"endpoint":"https://push.example.com/sub1"}`)

	err := svc.Unsubscribe()
	require.NoError(t, err)

	// Subscription removed
	_, err = settings.Get("push_subscription")
	assert.Error(t, err) // Not found
}

func TestPushService_HasSubscription(t *testing.T) {
	svc, settings := setupPushService(t)

	assert.False(t, svc.HasSubscription())

	_ = settings.Set("push_subscription", `{"endpoint":"https://push.example.com/sub1"}`)
	assert.True(t, svc.HasSubscription())
}

func TestPushService_Subscribe_ValidatesJSON(t *testing.T) {
	svc, _ := setupPushService(t)

	err := svc.Subscribe("not json")
	assert.Error(t, err)
}

func TestPushService_Subscribe_RequiresEndpoint(t *testing.T) {
	svc, _ := setupPushService(t)

	err := svc.Subscribe(`{"keys":{"p256dh":"key","auth":"auth"}}`)
	assert.Error(t, err)
}

func TestPushService_NilSafe(t *testing.T) {
	// Nil push service should not panic
	var svc *service.PushService
	assert.False(t, svc.HasSubscription())
	assert.NoError(t, svc.SendNotification("title", "body", ""))
}
