package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Soviann/trackarr/internal/cache"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
)

type CalendarHandler struct {
	calendarSvc *service.CalendarService
	cache       *cache.Cache
}

func NewCalendarHandler(calendarSvc *service.CalendarService, c ...*cache.Cache) *CalendarHandler {
	h := &CalendarHandler{
		calendarSvc: calendarSvc,
	}
	if len(c) > 0 {
		h.cache = c[0]
	}
	return h
}

// SetCache injects the in-memory read cache.
func (h *CalendarHandler) SetCache(c *cache.Cache) {
	h.cache = c
}

type CalendarTokenResponse struct {
	Token     string `json:"token"`
	FeedURL   string `json:"feed_url"`
	HTTPURL   string `json:"http_url"`
	WebcalURL string `json:"webcal_url"`
}

func buildURLs(r *http.Request, token string) CalendarTokenResponse {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	feedURL := fmt.Sprintf("/api/calendar.ics?token=%s", token)
	httpURL := fmt.Sprintf("%s://%s%s", scheme, host, feedURL)
	webcalURL := fmt.Sprintf("webcal://%s%s", host, feedURL)

	return CalendarTokenResponse{
		Token:     token,
		FeedURL:   feedURL,
		HTTPURL:   httpURL,
		WebcalURL: webcalURL,
	}
}

// ServeICS serves the unauthenticated RFC 5545 iCalendar feed secured by secret token.
func (h *CalendarHandler) ServeICS(w http.ResponseWriter, r *http.Request) error {
	token := r.URL.Query().Get("token")
	if token == "" || !h.calendarSvc.ValidateToken(r.Context(), token) {
		http.Error(w, "Unauthorized: invalid or missing calendar token", http.StatusUnauthorized)
		return nil
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

	ics, err := h.calendarSvc.GenerateICS(r.Context(), baseURL)
	if err != nil {
		return fmt.Errorf("calendar: generate ics: %w", err)
	}

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="trackarr.ics"`)
	w.Header().Set("Cache-Control", "private, max-age=1800")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(ics)
	return nil
}

// GetEvents returns calendar events for the in-app calendar.
func (h *CalendarHandler) GetEvents(w http.ResponseWriter, r *http.Request) error {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	typeFilter := r.URL.Query().Get("type")

	if start == "" {
		start = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if end == "" {
		end = time.Now().AddDate(0, 0, 90).Format("2006-01-02")
	}

	cacheKey := fmt.Sprintf("calendar:events:%s:%s:%s", start, end, typeFilter)
	if h.cache != nil {
		if val, ok := h.cache.Get(cacheKey); ok {
			if events, ok := val.([]repository.CalendarEventItem); ok {
				httputil.WriteJSON(w, http.StatusOK, events)
				return nil
			}
		}
	}

	events, err := h.calendarSvc.GetCalendarEvents(r.Context(), start, end)
	if err != nil {
		return fmt.Errorf("calendar: get events: %w", err)
	}

	if typeFilter != "" {
		var filtered []repository.CalendarEventItem
		for _, e := range events {
			switch {
			case typeFilter == "anime" && e.IsAnime:
				filtered = append(filtered, e)
			case typeFilter == "movie" && e.Type == "movie":
				filtered = append(filtered, e)
			case typeFilter == "series" && e.Type == "series" && !e.IsAnime:
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	if events == nil {
		events = []repository.CalendarEventItem{}
	}

	if h.cache != nil {
		h.cache.Set(cacheKey, events, cache.TagCalendar, cache.TagTitles)
	}

	httputil.WriteJSON(w, http.StatusOK, events)
	return nil
}

// GetToken returns the current calendar token and subscription URLs.
func (h *CalendarHandler) GetToken(w http.ResponseWriter, r *http.Request) error {
	token, err := h.calendarSvc.GetOrCreateToken(r.Context())
	if err != nil {
		return fmt.Errorf("calendar: get token: %w", err)
	}

	resp := buildURLs(r, token)
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// RegenerateToken rotates the calendar token and returns the new subscription URLs.
func (h *CalendarHandler) RegenerateToken(w http.ResponseWriter, r *http.Request) error {
	token, err := h.calendarSvc.RegenerateToken(r.Context())
	if err != nil {
		return fmt.Errorf("calendar: regenerate token: %w", err)
	}

	resp := buildURLs(r, token)
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}
