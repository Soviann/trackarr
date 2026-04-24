package service_test

import (
	"context"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAniListClient struct {
	calls       []matching.SaveMediaListEntryInput
	errToReturn error
}

func (f *fakeAniListClient) SaveMediaListEntry(_ context.Context, in matching.SaveMediaListEntryInput, _ string) error {
	f.calls = append(f.calls, in)
	return f.errToReturn
}

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

func TestPushSeasonState_CurrentSkipsRating(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Solo Leveling", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 2)
	testutil.SetSeasonEpisodeCount(t, db, seasonID, 13)
	testutil.MarkEpisodesWatched(t, db, seasonID, 5)
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "166240")
	testutil.SetTitleStatus(t, db, titleID, "watching")
	testutil.SetTitleRating(t, db, titleID, 9)
	testutil.SetSetting(t, db, "anilist_token", "test-token")

	fake := &fakeAniListClient{}
	svc := service.NewAniListPushService(db, fake, testutil.NopLogger())
	require.NoError(t, svc.PushSeasonState(context.Background(), seasonID))

	require.Len(t, fake.calls, 1)
	assert.Equal(t, int64(166240), fake.calls[0].MediaID)
	assert.Equal(t, "CURRENT", fake.calls[0].Status)
	assert.Equal(t, 5, fake.calls[0].Progress)
	assert.Nil(t, fake.calls[0].Score, "rating must not leak to a CURRENT season")
}

func TestPushSeasonState_CompletedPushesRating(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Solo Leveling", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	testutil.SetSeasonEpisodeCount(t, db, seasonID, 12)
	testutil.MarkEpisodesWatched(t, db, seasonID, 12)
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "113415")
	testutil.SetTitleStatus(t, db, titleID, "watching")
	testutil.SetTitleRating(t, db, titleID, 8)
	testutil.SetSetting(t, db, "anilist_token", "test-token")

	fake := &fakeAniListClient{}
	svc := service.NewAniListPushService(db, fake, testutil.NopLogger())
	require.NoError(t, svc.PushSeasonState(context.Background(), seasonID))

	require.Len(t, fake.calls, 1)
	assert.Equal(t, "COMPLETED", fake.calls[0].Status)
	require.NotNil(t, fake.calls[0].Score)
	assert.Equal(t, 8, *fake.calls[0].Score)
}

func TestPushSeasonState_SkipsWhenNoMapping(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Show", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	testutil.SetSetting(t, db, "anilist_token", "test-token")

	fake := &fakeAniListClient{}
	svc := service.NewAniListPushService(db, fake, testutil.NopLogger())
	require.NoError(t, svc.PushSeasonState(context.Background(), seasonID))
	assert.Empty(t, fake.calls)
}

func TestPushSeasonState_SkipsWhenTokenMissing(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Show", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "100")

	fake := &fakeAniListClient{}
	svc := service.NewAniListPushService(db, fake, testutil.NopLogger())
	require.NoError(t, svc.PushSeasonState(context.Background(), seasonID))
	assert.Empty(t, fake.calls)
}

func TestPushSeasonState_SkipsWhenTokenFlaggedInvalid(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Show", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "100")
	testutil.SetSetting(t, db, "anilist_token", "test-token")
	testutil.SetSetting(t, db, "anilist_token_invalid", "true")

	fake := &fakeAniListClient{}
	svc := service.NewAniListPushService(db, fake, testutil.NopLogger())
	require.NoError(t, svc.PushSeasonState(context.Background(), seasonID))
	assert.Empty(t, fake.calls)
}

func TestPushSeasonState_On401FlagsTokenInvalid(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Show", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	testutil.SetSeasonEpisodeCount(t, db, seasonID, 12)
	testutil.MarkEpisodesWatched(t, db, seasonID, 12)
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "100")
	testutil.SetTitleStatus(t, db, titleID, "watching")
	testutil.SetSetting(t, db, "anilist_token", "test-token")

	fake := &fakeAniListClient{errToReturn: matching.TokenInvalidError{}}
	svc := service.NewAniListPushService(db, fake, testutil.NopLogger())
	require.NoError(t, svc.PushSeasonState(context.Background(), seasonID))

	got, _ := testutil.GetSetting(t, db, "anilist_token_invalid")
	assert.Equal(t, "true", got)
}

func TestPushMovieState_Watched(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertMovieTitle(t, db, "Your Name", 21519)
	testutil.SetTitleStatus(t, db, titleID, "completed")
	testutil.SetTitleRating(t, db, titleID, 10)
	testutil.SetSetting(t, db, "anilist_token", "test-token")

	fake := &fakeAniListClient{}
	svc := service.NewAniListPushService(db, fake, testutil.NopLogger())
	require.NoError(t, svc.PushMovieState(context.Background(), titleID))

	require.Len(t, fake.calls, 1)
	assert.Equal(t, int64(21519), fake.calls[0].MediaID)
	assert.Equal(t, "COMPLETED", fake.calls[0].Status)
	require.NotNil(t, fake.calls[0].Score)
	assert.Equal(t, 10, *fake.calls[0].Score)
}

func TestPushMovieState_Dropped(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertMovieTitle(t, db, "Some Movie", 99999)
	testutil.SetTitleStatus(t, db, titleID, "dropped")
	testutil.SetSetting(t, db, "anilist_token", "test-token")

	fake := &fakeAniListClient{}
	svc := service.NewAniListPushService(db, fake, testutil.NopLogger())
	require.NoError(t, svc.PushMovieState(context.Background(), titleID))

	require.Len(t, fake.calls, 1)
	assert.Equal(t, "DROPPED", fake.calls[0].Status)
}

func TestPushMovieState_SkipsWhenNoAniListID(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Non-anime movie", false)
	testutil.SetTitleStatus(t, db, titleID, "completed")
	testutil.SetSetting(t, db, "anilist_token", "test-token")

	fake := &fakeAniListClient{}
	svc := service.NewAniListPushService(db, fake, testutil.NopLogger())
	require.NoError(t, svc.PushMovieState(context.Background(), titleID))
	assert.Empty(t, fake.calls)
}
