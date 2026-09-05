package handler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCalendarHandler(t *testing.T) (*handler.CalendarHandler, *service.CalendarService, *sql.DB, *repository.TitleRepository, *repository.SettingRepository) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	calSvc := service.NewCalendarService(db, titleRepo, settingRepo)
	h := handler.NewCalendarHandler(calSvc)

	return h, calSvc, db, titleRepo, settingRepo
}

func TestCalendarHandler_ServeICS(t *testing.T) {
	h, calSvc, _, _, _ := setupCalendarHandler(t)

	// 1. Missing token -> 401
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/calendar.ics", nil)
	err := h.ServeICS(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 2. Invalid token -> 401
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/calendar.ics?token=wrongtoken", nil)
	err = h.ServeICS(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 3. Valid token -> 200 with text/calendar
	tok, err := calSvc.GetOrCreateToken(context.Background())
	require.NoError(t, err)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/calendar.ics?token="+tok, nil)
	err = h.ServeICS(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/calendar")
	assert.Contains(t, w.Body.String(), "BEGIN:VCALENDAR")
}

func TestCalendarHandler_GetEvents(t *testing.T) {
	h, _, db, titleRepo, _ := setupCalendarHandler(t)
	_ = titleRepo

	today := time.Now().UTC().Format("2006-01-02")
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")

	// Anime series
	seriesID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, IsAnime: true, Year: 2026, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Solo Leveling", Language: "en", IsPrimary: true}},
	)
	season := testutil.UpsertSeason(t, db, seriesID, 1, 12)
	testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
		{EpisodeNumber: 1, Name: "Episode 1", AirDate: today},
	})

	// Movie
	testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2026, Status: model.TitleStatusPlanToWatch, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &tomorrow},
		[]model.TitleName{{Name: "Interstellar 2", Language: "en", IsPrimary: true}},
	)

	// 1. Get all events
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/calendar/events", nil)
	err := h.GetEvents(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var events []repository.CalendarEventItem
	require.NoError(t, json.NewDecoder(w.Body).Decode(&events))
	assert.Len(t, events, 2)

	// 2. Filter type=anime
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/calendar/events?type=anime", nil)
	err = h.GetEvents(w, req)
	assert.NoError(t, err)
	var animeEvents []repository.CalendarEventItem
	require.NoError(t, json.NewDecoder(w.Body).Decode(&animeEvents))
	assert.Len(t, animeEvents, 1)
	assert.Equal(t, "Solo Leveling", animeEvents[0].TitleName)

	// 3. Filter type=movie
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/calendar/events?type=movie", nil)
	err = h.GetEvents(w, req)
	assert.NoError(t, err)
	var movieEvents []repository.CalendarEventItem
	require.NoError(t, json.NewDecoder(w.Body).Decode(&movieEvents))
	assert.Len(t, movieEvents, 1)
	assert.Equal(t, "Interstellar 2", movieEvents[0].TitleName)
}

func TestCalendarHandler_TokenEndpoints(t *testing.T) {
	h, _, _, _, _ := setupCalendarHandler(t)

	// Get Token
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/calendar/token", nil)
	req.Host = "trackarr.local:8080"
	err := h.GetToken(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var tokResp handler.CalendarTokenResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&tokResp))
	assert.NotEmpty(t, tokResp.Token)
	assert.Contains(t, tokResp.FeedURL, "/api/calendar.ics?token=")
	assert.Contains(t, tokResp.WebcalURL, "webcal://trackarr.local:8080/api/calendar.ics?token=")

	// Regenerate Token
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/calendar/token/regenerate", nil)
	req.Host = "trackarr.local:8080"
	err = h.RegenerateToken(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var regenResp handler.CalendarTokenResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&regenResp))
	assert.NotEmpty(t, regenResp.Token)
	assert.NotEqual(t, tokResp.Token, regenResp.Token)
}
