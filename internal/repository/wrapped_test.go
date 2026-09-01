package repository_test

import (
	"context"
	"testing"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrappedRepository_SaveAndGetSnapshot(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewWrappedRepository(db)
	ctx := context.Background()

	// Initial check - no snapshot
	snap, createdAt, err := repo.GetSnapshot(ctx, 2025)
	require.NoError(t, err)
	assert.Nil(t, snap)
	assert.Nil(t, createdAt)

	has, err := repo.HasSnapshot(ctx, 2025)
	require.NoError(t, err)
	assert.False(t, has)

	// Save snapshot
	sampleResp := &model.WrappedResponse{
		Year:              2025,
		AvailableYears:    []int{2025},
		TotalWatchMinutes: 5400,
		Overview: model.StatsOverview{
			TotalTitles:     15,
			TotalMovies:     10,
			TotalSeries:     5,
			EpisodesWatched: 80,
			AverageRating:   8.2,
		},
		TopFavorites: model.WrappedCategoryTop{
			Movies: []model.WrappedTitleItem{
				{ID: 1, Title: "Oppenheimer", Year: 2023, Type: "movie", MyRating: ptr(9)},
			},
		},
		Persona: model.WrappedAIPersona{
			Title:    "The Cinematic Voyager",
			Summary:  "An epic year of historical epics and dramas.",
			Quote:    "Cinema is life.",
			FunFacts: []string{"70% watched at night."},
			Badges:   []string{"Cinephile"},
		},
	}

	err = repo.SaveSnapshot(ctx, 2025, sampleResp)
	require.NoError(t, err)

	has, err = repo.HasSnapshot(ctx, 2025)
	require.NoError(t, err)
	assert.True(t, has)

	// Retrieve snapshot
	snap, createdAt, err = repo.GetSnapshot(ctx, 2025)
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.NotNil(t, createdAt)
	assert.Equal(t, 2025, snap.Year)
	assert.Equal(t, 5400, snap.TotalWatchMinutes)
	assert.Equal(t, "The Cinematic Voyager", snap.Persona.Title)
	assert.Len(t, snap.TopFavorites.Movies, 1)

	// List archives
	archives, err := repo.ListArchives(ctx)
	require.NoError(t, err)
	require.Len(t, archives, 1)
	assert.Equal(t, 2025, archives[0].Year)
	assert.Equal(t, "The Cinematic Voyager", archives[0].PersonaTitle)
	assert.Equal(t, 5400, archives[0].TotalWatchMinutes)
	assert.Equal(t, 15, archives[0].TotalTitles)

	// Delete snapshot
	err = repo.DeleteSnapshot(ctx, 2025)
	require.NoError(t, err)

	has, err = repo.HasSnapshot(ctx, 2025)
	require.NoError(t, err)
	assert.False(t, has)
}
