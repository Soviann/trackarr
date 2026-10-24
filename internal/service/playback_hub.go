package service

import (
	"sync"
	"time"

	"github.com/Soviann/trackarr/internal/model"
)

const defaultPlaybackTTL = 4 * time.Hour

// PlaybackHubService manages active media playback sessions in memory.
// Thread-safe and designed to avoid unnecessary SQLite disk writes on media play/resume events.
type PlaybackHubService struct {
	mu       sync.RWMutex
	sessions map[string]model.ActivePlaybackSession
	ttl      time.Duration
}

// NewPlaybackHubService creates a new in-memory playback hub.
func NewPlaybackHubService() *PlaybackHubService {
	return &PlaybackHubService{
		sessions: make(map[string]model.ActivePlaybackSession),
		ttl:      defaultPlaybackTTL,
	}
}

// sessionKey generates a map key from source and player/device.
func sessionKey(source, player string) string {
	if player == "" {
		player = "default"
	}
	return source + ":" + player
}

// SetSession records or updates an active playback session.
func (h *PlaybackHubService) SetSession(session model.ActivePlaybackSession) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	if session.StartedAt.IsZero() {
		session.StartedAt = now
	}
	session.UpdatedAt = now

	key := sessionKey(session.Source, session.Player)
	h.sessions[key] = session
}

// ClearSession removes an active session for the given source and player.
func (h *PlaybackHubService) ClearSession(source, player string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := sessionKey(source, player)
	delete(h.sessions, key)
}

// ClearByTitleID clears any active session matching the given title ID.
// Useful when playback finishes or scrobble is completed.
func (h *PlaybackHubService) ClearByTitleID(titleID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for k, s := range h.sessions {
		if s.TitleID == titleID {
			delete(h.sessions, k)
		}
	}
}

// GetActiveSession returns the most recently updated non-expired playback session, or nil.
func (h *PlaybackHubService) GetActiveSession() *model.ActivePlaybackSession {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	var newest *model.ActivePlaybackSession

	for k, s := range h.sessions {
		// Prune expired sessions
		if now.Sub(s.UpdatedAt) > h.ttl {
			delete(h.sessions, k)
			continue
		}
		if newest == nil || s.UpdatedAt.After(newest.UpdatedAt) {
			copied := s
			newest = &copied
		}
	}

	return newest
}

// ClearAll removes all active sessions.
func (h *PlaybackHubService) ClearAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions = make(map[string]model.ActivePlaybackSession)
}
