package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogout_ClearsCookie(t *testing.T) {
	h := handler.NewAuthHandler("secret", "test@example.com", "client-id", false)
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	rr := httptest.NewRecorder()

	require.NoError(t, h.Logout(rr, req))

	assert.Equal(t, http.StatusNoContent, rr.Code)
	cookies := rr.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "token", cookies[0].Name)
	assert.Equal(t, "", cookies[0].Value)
	assert.True(t, cookies[0].MaxAge < 0)
}
