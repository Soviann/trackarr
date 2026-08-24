package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected *ExternalIDs
	}{
		{
			name:     "IMDb title URL",
			url:      "https://www.imdb.com/title/tt1234567/",
			expected: &ExternalIDs{IMDB: "tt1234567"},
		},
		{
			name:     "IMDb title URL without trailing slash",
			url:      "https://www.imdb.com/title/tt1234567",
			expected: &ExternalIDs{IMDB: "tt1234567"},
		},
		{
			name:     "AniList anime URL",
			url:      "https://anilist.co/anime/12345/",
			expected: &ExternalIDs{AniList: 12345},
		},
		{
			name:     "AniList anime URL without trailing slash",
			url:      "https://anilist.co/anime/12345",
			expected: &ExternalIDs{AniList: 12345},
		},
		{
			name:     "TVDB series URL",
			url:      "https://thetvdb.com/series/shogun-2024",
			expected: &ExternalIDs{}, // TVDB slug parsed; numeric IDs require API resolution
		},
		{
			name:     "TVDB movie URL",
			url:      "https://thetvdb.com/movies/fight-club-1999",
			expected: &ExternalIDs{}, // TVDB slug parsed; numeric IDs require API resolution
		},
		{
			name:     "Invalid URL",
			url:      "https://google.com",
			expected: nil,
		},
		{
			name:     "Random string",
			url:      "not a url",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseURL(tt.url)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseURLFull_TVDBSlugs(t *testing.T) {
	got := ParseURLFull("https://thetvdb.com/series/frieren-beyond-journeys-end")
	if assert.NotNil(t, got) {
		assert.Equal(t, "frieren-beyond-journeys-end", got.TVDBSeriesSlug)
		assert.Empty(t, got.TVDBMovieSlug)
	}

	got = ParseURLFull("https://thetvdb.com/movies/fight-club-1999")
	if assert.NotNil(t, got) {
		assert.Equal(t, "fight-club-1999", got.TVDBMovieSlug)
		assert.Empty(t, got.TVDBSeriesSlug)
	}

	got = ParseURLFull("https://google.com")
	assert.Nil(t, got)
}
