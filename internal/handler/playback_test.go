package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaybackHandler_GetActivePlayback_Nominal(t *testing.T) {
	hub := service.NewPlaybackHubService()
	h := handler.NewPlaybackHandler(hub)

	hub.SetSession(model.ActivePlaybackSession{
		Source:    "plex",
		Player:    "Apple TV",
		TitleID:   123,
		TitleName: "Severance",
		TitleType: model.TitleTypeSeries,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/playback/active", nil)
	rec := httptest.NewRecorder()

	err := h.GetActivePlayback(rec, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var session model.ActivePlaybackSession
	err = json.Unmarshal(rec.Body.Bytes(), &session)
	require.NoError(t, err)
	assert.Equal(t, int64(123), session.TitleID)
	assert.Equal(t, "Severance", session.TitleName)
}

func TestPlaybackHandler_GetActivePlayback_NoContent(t *testing.T) {
	hub := service.NewPlaybackHubService()
	h := handler.NewPlaybackHandler(hub)

	req := httptest.NewRequest(http.MethodGet, "/api/playback/active", nil)
	rec := httptest.NewRecorder()

	err := h.GetActivePlayback(rec, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.Bytes())
}

func TestPlaybackHandler_GetActivePlayback_NilHub(t *testing.T) {
	h := handler.NewPlaybackHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/playback/active", nil)
	rec := httptest.NewRecorder()

	err := h.GetActivePlayback(rec, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}
