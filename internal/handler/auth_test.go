package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/stretchr/testify/assert"
)

func TestLogout_ClearsCookie(t *testing.T) {
	h := handler.NewAuthHandler("secret", "test@example.com", "client-id")
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	rr := httptest.NewRecorder()

	h.Logout(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	cookies := rr.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "token", cookies[0].Name)
	assert.Equal(t, "", cookies[0].Value)
	assert.True(t, cookies[0].MaxAge < 0)
}
