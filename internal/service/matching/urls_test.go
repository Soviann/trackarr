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
			name: "IMDb title URL",
			url:  "https://www.imdb.com/title/tt1234567/",
			expected: &ExternalIDs{IMDB: "tt1234567"},
		},
		{
			name: "IMDb title URL without trailing slash",
			url:  "https://www.imdb.com/title/tt1234567",
			expected: &ExternalIDs{IMDB: "tt1234567"},
		},
		{
			name: "AniList anime URL",
			url:  "https://anilist.co/anime/12345/",
			expected: &ExternalIDs{AniList: 12345},
		},
		{
			name: "AniList anime URL without trailing slash",
			url:  "https://anilist.co/anime/12345",
			expected: &ExternalIDs{AniList: 12345},
		},
		{
			name: "TVDB series URL (detect but no ID)",
			url:  "https://thetvdb.com/series/shogun-2024",
			expected: nil, // Currently we don't extract IDs from TVDB slugs
		},
		{
			name: "Invalid URL",
			url:  "https://google.com",
			expected: nil,
		},
		{
			name: "Random string",
			url:  "not a url",
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
