package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPushHandler(t *testing.T) *handler.PushHandler {
	t.Helper()
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	settings := repository.NewSettingRepository(db)
	pushSvc := service.NewPushService(settings, "pub", "priv", "mailto:test@test.com")
	return handler.NewPushHandler(pushSvc)
}

func TestPushHandler_Subscribe(t *testing.T) {
	h := setupPushHandler(t)

	body := `{"endpoint":"https://push.example.com/sub","keys":{"p256dh":"k","auth":"a"}}`
	req := httptest.NewRequest("POST", "/api/push/subscribe", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Subscribe(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestPushHandler_Subscribe_InvalidJSON(t *testing.T) {
	h := setupPushHandler(t)

	req := httptest.NewRequest("POST", "/api/push/subscribe", strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	h.Subscribe(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPushHandler_Unsubscribe(t *testing.T) {
	h := setupPushHandler(t)

	req := httptest.NewRequest("DELETE", "/api/push/subscribe", nil)
	rr := httptest.NewRecorder()
	h.Unsubscribe(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}
