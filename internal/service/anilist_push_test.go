package service_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestDeriveSeasonState(t *testing.T) {
	tests := []struct {
		name                           string
		titleStatus                    string
		totalEpisodes, watchedEpisodes int
		wantStatus                     string
		wantProgress                   int
	}{
		{"plan_to_watch, none watched", "plan_to_watch", 10, 0, "PLANNING", 0},
		{"watching, 5/10", "watching", 10, 5, "CURRENT", 5},
		{"watching, all watched", "watching", 10, 10, "COMPLETED", 10},
		{"completed, all watched", "completed", 10, 10, "COMPLETED", 10},
		{"dropped, none watched", "dropped", 10, 0, "DROPPED", 0},
		{"dropped, 3/10 watched", "dropped", 10, 3, "DROPPED", 3},
		{"dropped, all watched — COMPLETED wins", "dropped", 10, 10, "COMPLETED", 10},
		{"watching, 0/10 (fallback CURRENT)", "watching", 10, 0, "CURRENT", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotProgress := service.DeriveSeasonState(tt.titleStatus, tt.totalEpisodes, tt.watchedEpisodes)
			assert.Equal(t, tt.wantStatus, gotStatus)
			assert.Equal(t, tt.wantProgress, gotProgress)
		})
	}
}

func TestShouldPushRating(t *testing.T) {
	assert.True(t, service.ShouldPushRating("COMPLETED"))
	assert.True(t, service.ShouldPushRating("DROPPED"))
	assert.False(t, service.ShouldPushRating("CURRENT"))
	assert.False(t, service.ShouldPushRating("PLANNING"))
}
