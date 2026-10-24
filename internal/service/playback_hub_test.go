package service_test

import (
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaybackHub_Nominal(t *testing.T) {
	hub := service.NewPlaybackHubService()

	// Initial state: empty
	assert.Nil(t, hub.GetActiveSession())

	// Register active playback
	accent := "#e50914"
	epNum := 3
	seasonNum := 1
	epName := "Pilot Part 3"
	cover := "covers/severance.jpg"
	session := model.ActivePlaybackSession{
		Source:        "plex",
		Player:        "Shield TV",
		User:          "alice",
		TitleID:       42,
		TitleName:     "Severance",
		TitleType:     model.TitleTypeSeries,
		Year:          2022,
		CoverURL:      &cover,
		AccentHex:     &accent,
		SeasonNumber:  &seasonNum,
		EpisodeNumber: &epNum,
		EpisodeName:   &epName,
		Overview:      "Mark leads a team of office workers.",
		DurationSec:   3600,
		ViewOffsetSec: 1200,
	}

	hub.SetSession(session)

	active := hub.GetActiveSession()
	require.NotNil(t, active)
	assert.Equal(t, int64(42), active.TitleID)
	assert.Equal(t, "Severance", active.TitleName)
	assert.Equal(t, "Shield TV", active.Player)
	assert.Equal(t, &epNum, active.EpisodeNumber)

	// Clear session
	hub.ClearSession("plex", "Shield TV")
	assert.Nil(t, hub.GetActiveSession())
}

func TestPlaybackHub_Boundary(t *testing.T) {
	hub := service.NewPlaybackHubService()

	// Boundary 1: multiple concurrent sessions -> returns the newest
	session1 := model.ActivePlaybackSession{
		Source:    "plex",
		Player:    "Bedroom TV",
		TitleID:   10,
		TitleName: "Movie 1",
		TitleType: model.TitleTypeMovie,
	}
	session2 := model.ActivePlaybackSession{
		Source:    "jellyfin",
		Player:    "Living Room TV",
		TitleID:   20,
		TitleName: "Movie 2",
		TitleType: model.TitleTypeMovie,
	}

	hub.SetSession(session1)
	time.Sleep(5 * time.Millisecond)
	hub.SetSession(session2)

	active := hub.GetActiveSession()
	require.NotNil(t, active)
	assert.Equal(t, int64(20), active.TitleID)

	// Boundary 2: ClearByTitleID clears session for that specific title
	hub.ClearByTitleID(20)
	activeAfterClear := hub.GetActiveSession()
	require.NotNil(t, activeAfterClear)
	assert.Equal(t, int64(10), activeAfterClear.TitleID)

	// Boundary 3: ClearAll
	hub.ClearAll()
	assert.Nil(t, hub.GetActiveSession())
}

func TestPlaybackHub_Negative(t *testing.T) {
	hub := service.NewPlaybackHubService()

	// Negative 1: query when no sessions exist
	assert.Nil(t, hub.GetActiveSession())

	// Negative 2: clear non-existent session is a safe no-op
	assert.NotPanics(t, func() {
		hub.ClearSession("plex", "nonexistent")
		hub.ClearByTitleID(999999)
	})
	assert.Nil(t, hub.GetActiveSession())
}
