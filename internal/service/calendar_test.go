package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarService_TokenManagement(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	calSvc := service.NewCalendarService(db, titleRepo, settingRepo)

	ctx := context.Background()

	// Initial token creation
	tok1, err := calSvc.GetOrCreateToken(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, tok1)
	assert.Len(t, tok1, 64) // 32 bytes hex encoded

	// Getting existing token returns same token
	tok2, err := calSvc.GetOrCreateToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, tok1, tok2)

	// Validation
	assert.True(t, calSvc.ValidateToken(ctx, tok1))
	assert.False(t, calSvc.ValidateToken(ctx, "invalid-token"))
	assert.False(t, calSvc.ValidateToken(ctx, ""))

	// Regeneration
	tok3, err := calSvc.RegenerateToken(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, tok3)
	assert.NotEqual(t, tok1, tok3)
	assert.False(t, calSvc.ValidateToken(ctx, tok1))
	assert.True(t, calSvc.ValidateToken(ctx, tok3))
}

func TestCalendarService_GenerateICS(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	calSvc := service.NewCalendarService(db, titleRepo, settingRepo)

	ctx := context.Background()

	today := time.Now().UTC().Format("2006-01-02")
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")

	// Series with episodes
	seriesID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, Year: 2026, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Frieren: Beyond Journey's End", Language: "en", IsPrimary: true}},
	)
	season := testutil.UpsertSeason(t, db, seriesID, 1, 10)
	testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
		{EpisodeNumber: 5, Name: "Phantom", AirDate: today},
		{EpisodeNumber: 6, Name: "The Hero's Sword", AirDate: tomorrow},
	})

	// Movie with release_date
	movieAirDate := time.Now().UTC().AddDate(0, 0, 5).Format("2006-01-02")
	testutil.CreateTitle(t, db,
		&model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2026,
			Status:      model.TitleStatusPlanToWatch,
			MatchStatus: model.MatchStatusConfirmed,
			ReleaseDate: &movieAirDate,
		},
		[]model.TitleName{{Name: "Dune 3", Language: "en", IsPrimary: true}},
	)

	icsBytes, err := calSvc.GenerateICS(ctx, "https://trackarr.example.com")
	require.NoError(t, err)

	icsStr := string(icsBytes)
	assert.Contains(t, icsStr, "BEGIN:VCALENDAR")
	assert.Contains(t, icsStr, "PRODID:-//Trackarr//Media Calendar//EN")
	assert.Contains(t, icsStr, "X-WR-CALNAME:Trackarr Calendar")
	assert.Contains(t, icsStr, "BEGIN:VEVENT")
	assert.Contains(t, icsStr, "Frieren: Beyond Journey's End - S01E05 - Phantom")
	assert.Contains(t, icsStr, "Frieren: Beyond Journey's End - S01E06 - The Hero's Sword")
	assert.Contains(t, icsStr, "Dune 3")
	assert.Contains(t, icsStr, "URL:https://trackarr.example.com/title/")
	assert.Contains(t, icsStr, "END:VCALENDAR")
	assert.True(t, strings.HasSuffix(icsStr, "END:VCALENDAR\r\n"))
}
