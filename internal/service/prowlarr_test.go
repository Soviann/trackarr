package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReleaseTitle(t *testing.T) {
	tests := []struct {
		raw       string
		wantTitle string
		wantYear  int
		wantType  string
	}{
		{
			raw:       "Gladiator.II.2024.MULTi.1080p.WEB-DL.DDP5.1.Atmos.H.264-FW",
			wantTitle: "Gladiator II",
			wantYear:  2024,
			wantType:  "movie",
		},
		{
			raw:       "Severance.S02E01.FRENCH.1080p.WEB.H264-FW",
			wantTitle: "Severance",
			wantYear:  0,
			wantType:  "series",
		},
		{
			raw:       "Fallout.S01.MULTi.1080p.Prime.WEB-DL",
			wantTitle: "Fallout",
			wantYear:  0,
			wantType:  "series",
		},
		{
			raw:       "The.Matrix.1999.2160p.UHD.Remux.HEVC",
			wantTitle: "The Matrix",
			wantYear:  1999,
			wantType:  "movie",
		},
		{
			raw:       "Stranger Things Saison 4 MULTi 1080p",
			wantTitle: "Stranger Things",
			wantYear:  0,
			wantType:  "series",
		},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			title, year, relType := ParseReleaseTitle(tt.raw)
			assert.Equal(t, tt.wantTitle, title)
			assert.Equal(t, tt.wantYear, year)
			assert.Equal(t, tt.wantType, relType)
		})
	}
}

func TestProwlarrService_GetReleases(t *testing.T) {
	fakeReleases := []prowlarrRawItem{
		{
			GUID:        "guid-1",
			Title:       "Dune.Part.Two.2024.MULTi.1080p.WEB",
			Size:        4500000000,
			PublishDate: time.Now(),
			Seeders:     120,
			Leechers:    10,
			Indexer:     "C411",
			IndexerID:   1,
			TMDBID:      float64(693134),
			IMDBID:      "tt15239678",
			Categories: []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}{
				{ID: 2000, Name: "Movies"},
			},
		},
		{
			GUID:        "guid-2",
			Title:       "The.Penguin.S01.MULTi.1080p.WEB",
			Size:        15000000000,
			PublishDate: time.Now(),
			Seeders:     85,
			Leechers:    5,
			Indexer:     "C411",
			IndexerID:   1,
			TMDBID:      float64(134949),
			Categories: []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}{
				{ID: 5000, Name: "TV"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-api-key", r.Header.Get("X-Api-Key"))
		assert.Contains(t, r.URL.Path, "/api/v1/search")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fakeReleases)
	}))
	defer server.Close()

	cfg := &config.Config{
		ProwlarrURL:    server.URL,
		ProwlarrAPIKey: "test-api-key",
	}

	svc := NewProwlarrService(cfg, nil, nil, nil)
	releases, err := svc.GetReleases(context.Background(), "all", false)
	require.NoError(t, err)
	require.Len(t, releases, 2)

	assert.Equal(t, "Dune Part Two", releases[0].CleanTitle)
	assert.Equal(t, 2024, releases[0].Year)
	assert.Equal(t, "movie", releases[0].Type)
	assert.Equal(t, int64(693134), releases[0].TMDBID)

	assert.Equal(t, "The Penguin", releases[1].CleanTitle)
	assert.Equal(t, "series", releases[1].Type)
	assert.Equal(t, int64(134949), releases[1].TMDBID)

	// Test cache hit
	cached, err := svc.GetReleases(context.Background(), "all", false)
	require.NoError(t, err)
	assert.Len(t, cached, 2)
}
