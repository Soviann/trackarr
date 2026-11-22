package handler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/handler/httputil"
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

	// HTTPS detection via proxy / origin / referer headers
	for _, tc := range []struct {
		name   string
		header string
		value  string
		want   string
	}{
		{name: "default http", want: "http://trackarr.example.com/api/calendar.ics?token="},
		{name: "X-Forwarded-Proto", header: "X-Forwarded-Proto", value: "https", want: "https://trackarr.example.com/api/calendar.ics?token="},
		{name: "X-Forwarded-Proto chained", header: "X-Forwarded-Proto", value: "https, http", want: "https://trackarr.example.com/api/calendar.ics?token="},
		{name: "X-Forwarded-Ssl", header: "X-Forwarded-Ssl", value: "on", want: "https://trackarr.example.com/api/calendar.ics?token="},
		{name: "Forwarded", header: "Forwarded", value: "for=1.2.3.4;proto=https", want: "https://trackarr.example.com/api/calendar.ics?token="},
		{name: "Forwarded quoted", header: "Forwarded", value: `for=1.2.3.4;proto="https"`, want: "https://trackarr.example.com/api/calendar.ics?token="},
		{name: "Origin", header: "Origin", value: "https://trackarr.example.com", want: "https://trackarr.example.com/api/calendar.ics?token="},
		{name: "Referer", header: "Referer", value: "https://trackarr.example.com/calendar", want: "https://trackarr.example.com/api/calendar.ics?token="},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/calendar/token", nil)
			req.Host = "trackarr.example.com"
			if tc.header != "" {
				req.Header.Set(tc.header, tc.value)
			}
			require.NoError(t, h.GetToken(w, req))
			var resp handler.CalendarTokenResponse
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
			assert.True(t, len(resp.HTTPURL) > len(tc.want) && resp.HTTPURL[:len(tc.want)] == tc.want, "got %s", resp.HTTPURL)
		})
	}
}

func TestCalendarHandler_ServeICS_GroupingToggle(t *testing.T) {
	h, calSvc, db, _, settingRepo := setupCalendarHandler(t)
	adminH := handler.NewAdminSettingsHandler(db, settingRepo, nil)

	airTime := time.Now().UTC()
	airDate := airTime.Format("2006-01-02")
	wantDate := airTime.Format("20060102")

	seriesID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, IsAnime: true, Year: 2026, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Frieren", Language: "en", IsPrimary: true}},
	)
	season := testutil.UpsertSeason(t, db, seriesID, 1, 12)
	testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
		{EpisodeNumber: 1, Name: "Ep One", AirDate: airDate},
		{EpisodeNumber: 2, Name: "Ep Two", AirDate: airDate},
		{EpisodeNumber: 3, Name: "Ep Three", AirDate: airDate},
	})
	movieID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2026, Status: model.TitleStatusPlanToWatch, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &airDate},
		[]model.TitleName{{Name: "Movie Same Day", Language: "en", IsPrimary: true}},
	)

	tok, err := calSvc.GetOrCreateToken(context.Background())
	require.NoError(t, err)

	fetchICS := func() string {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/calendar.ics?token="+tok, nil)
		req.Host = "trackarr.example.com"
		httputil.WrapHandler(h.ServeICS)(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		return strings.ReplaceAll(w.Body.String(), "\r\n ", "")
	}

	// 1. Default (unset) groups same-day episodes while keeping movie separate.
	defaultICS := fetchICS()
	assert.Equal(t, 2, strings.Count(defaultICS, "BEGIN:VEVENT"))
	assert.Contains(t, defaultICS, fmt.Sprintf("UID:trackarr-group-%d-%s@trackarr", seriesID, wantDate))
	assert.Contains(t, defaultICS, "SUMMARY:Frieren - S01E01-E03")
	assert.Contains(t, defaultICS, fmt.Sprintf("UID:trackarr-title-%d-%s@trackarr", movieID, airDate))

	// 2. Disable via PUT /api/admin/system-settings -> GET reports false and ICS emits individual VEVENTs.
	putFalseReq := httptest.NewRequest(http.MethodPut, "/api/admin/system-settings", strings.NewReader(`{"ical_group_same_day":"false"}`))
	putFalseRR := httptest.NewRecorder()
	httputil.WrapHandler(adminH.UpdateSystemSettings)(putFalseRR, putFalseReq)
	require.Equal(t, http.StatusNoContent, putFalseRR.Code)

	getSettingsReq := httptest.NewRequest(http.MethodGet, "/api/admin/system-settings", nil)
	getSettingsRR := httptest.NewRecorder()
	httputil.WrapHandler(adminH.GetSystemSettings)(getSettingsRR, getSettingsReq)
	require.Equal(t, http.StatusOK, getSettingsRR.Code)
	var settingsResp handler.SystemSettingsResponse
	require.NoError(t, json.NewDecoder(getSettingsRR.Body).Decode(&settingsResp))
	assert.False(t, settingsResp.ICalGroupSameDay)

	ungroupedICS := fetchICS()
	assert.Equal(t, 4, strings.Count(ungroupedICS, "BEGIN:VEVENT"))
	assert.NotContains(t, ungroupedICS, "trackarr-group-")
	assert.Contains(t, ungroupedICS, "SUMMARY:Frieren - S01E01 - Ep One")
	assert.Contains(t, ungroupedICS, "SUMMARY:Frieren - S01E02 - Ep Two")
	assert.Contains(t, ungroupedICS, "SUMMARY:Frieren - S01E03 - Ep Three")

	// 3. Re-enable via PUT /api/admin/system-settings -> ICS returns to grouped output.
	putTrueReq := httptest.NewRequest(http.MethodPut, "/api/admin/system-settings", strings.NewReader(`{"ical_group_same_day":"true"}`))
	putTrueRR := httptest.NewRecorder()
	httputil.WrapHandler(adminH.UpdateSystemSettings)(putTrueRR, putTrueReq)
	require.Equal(t, http.StatusNoContent, putTrueRR.Code)

	regroupedICS := fetchICS()
	assert.Equal(t, 2, strings.Count(regroupedICS, "BEGIN:VEVENT"))
	assert.Contains(t, regroupedICS, fmt.Sprintf("UID:trackarr-group-%d-%s@trackarr", seriesID, wantDate))
}
