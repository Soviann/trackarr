package service

import (
	"testing"

	plexwebhooks "github.com/hekmon/plexwebhooks"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeJellyfinPayload_Movie(t *testing.T) {
	jf := &model.JellyfinPayload{
		NotificationType:   "PlaybackStop",
		ItemType:           "Movie",
		Name:               "The Matrix",
		Year:               "1999",
		PlayedToCompletion: "True",
		ProviderIMDB:       "tt0133093",
		ProviderTMDB:       "603",
		ItemID:             "abc123",
	}

	payload, ok := normalizeJellyfinPayload(jf)
	require.True(t, ok)
	assert.Equal(t, plexwebhooks.EventTypeScrobble, payload.Event)
	assert.Equal(t, plexwebhooks.MediaTypeMovie, payload.Metadata.Type)
	assert.Equal(t, "The Matrix", payload.Metadata.Title)
	assert.Equal(t, 1999, payload.Metadata.Year)
	assert.Equal(t, "abc123", payload.Metadata.RatingKey)

	// GUIDs must be parseable by the shared ParseGUIDs helper.
	ids := ParseGUIDs(payload.Metadata.GUIDExternal)
	assert.Equal(t, "tt0133093", ids.IMDB)
	assert.Equal(t, int64(603), ids.TMDB)
}

func TestNormalizeJellyfinPayload_Episode(t *testing.T) {
	jf := &model.JellyfinPayload{
		NotificationType:   "PlaybackStop",
		ItemType:           "Episode",
		Name:               "Winter Is Coming",
		Year:               "2011",
		PlayedToCompletion: "True",
		SeriesName:         "Game of Thrones",
		SeriesID:           "series-guid-1",
		Season:             "1",
		Episode:            "1",
	}

	payload, ok := normalizeJellyfinPayload(jf)
	require.True(t, ok)
	assert.Equal(t, plexwebhooks.MediaTypeEpisode, payload.Metadata.Type)
	assert.Equal(t, "Game of Thrones", payload.Metadata.GrandparentTitle)
	assert.Equal(t, "series-guid-1", payload.Metadata.GrandparentRatingKey)
	assert.Equal(t, 1, payload.Metadata.ParentIndex)
	assert.Equal(t, 1, payload.Metadata.Index)
	// Episode-level provider IDs must NOT leak in as series IDs.
	assert.Empty(t, ParseGUIDs(payload.Metadata.GUIDExternal).IMDB)
}

func TestNormalizeJellyfinPayload_Ignored(t *testing.T) {
	cases := []struct {
		name string
		jf   *model.JellyfinPayload
	}{
		{"playback start", &model.JellyfinPayload{NotificationType: "PlaybackStart", ItemType: "Movie", PlayedToCompletion: "True"}},
		{"stopped before completion", &model.JellyfinPayload{NotificationType: "PlaybackStop", ItemType: "Movie", PlayedToCompletion: "False"}},
		{"completion flag empty", &model.JellyfinPayload{NotificationType: "PlaybackStop", ItemType: "Movie", PlayedToCompletion: ""}},
		{"unsupported item type", &model.JellyfinPayload{NotificationType: "PlaybackStop", ItemType: "Audio", PlayedToCompletion: "True"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := normalizeJellyfinPayload(tc.jf)
			assert.False(t, ok)
		})
	}
}
