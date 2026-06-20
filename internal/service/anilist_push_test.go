package service_test

import (
	"context"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptrInt(v int) *int { return &v }

// seq returns []int{a, a+1, ..., b}.
func seq(a, b int) []int {
	out := make([]int, 0, b-a+1)
	for i := a; i <= b; i++ {
		out = append(out, i)
	}
	return out
}

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

func TestDerivePartStates_TwoParts(t *testing.T) {
	parts := []model.AniListPart{
		{ExternalID: "100", EpisodeCount: ptrInt(12)},
		{ExternalID: "200", EpisodeCount: ptrInt(16)},
	}

	// All 28 watched → both parts COMPLETED at their full counts.
	got := service.DerivePartStates("watching", parts, seq(1, 28))
	require.Len(t, got, 2)
	assert.Equal(t, int64(100), got[0].MediaID)
	assert.Equal(t, "COMPLETED", got[0].Status)
	assert.Equal(t, 12, got[0].Progress)
	assert.True(t, got[0].Rating)
	assert.Equal(t, int64(200), got[1].MediaID)
	assert.Equal(t, "COMPLETED", got[1].Status)
	assert.Equal(t, 16, got[1].Progress)
	assert.True(t, got[1].Rating)

	// Exactly part 1 watched → part1 COMPLETED(12), part2 CURRENT(0).
	got = service.DerivePartStates("watching", parts, seq(1, 12))
	require.Len(t, got, 2)
	assert.Equal(t, "COMPLETED", got[0].Status)
	assert.Equal(t, 12, got[0].Progress)
	assert.Equal(t, "CURRENT", got[1].Status)
	assert.Equal(t, 0, got[1].Progress)
	assert.False(t, got[1].Rating)

	// 18 watched → part1 COMPLETED(12), part2 CURRENT(6).
	got = service.DerivePartStates("watching", parts, seq(1, 18))
	require.Len(t, got, 2)
	assert.Equal(t, "COMPLETED", got[0].Status)
	assert.Equal(t, 12, got[0].Progress)
	assert.Equal(t, "CURRENT", got[1].Status)
	assert.Equal(t, 6, got[1].Progress)
}

func TestDerivePartStates_DroppedPartial(t *testing.T) {
	parts := []model.AniListPart{
		{ExternalID: "100", EpisodeCount: ptrInt(12)},
		{ExternalID: "200", EpisodeCount: ptrInt(16)},
	}

	// Dropped title, part1 fully watched, part2 partial: COMPLETED still wins
	// for the finished part; the unfinished part is DROPPED with its progress.
	got := service.DerivePartStates("dropped", parts, seq(1, 18))
	require.Len(t, got, 2)
	assert.Equal(t, "COMPLETED", got[0].Status)
	assert.Equal(t, 12, got[0].Progress)
	assert.Equal(t, "DROPPED", got[1].Status)
	assert.Equal(t, 6, got[1].Progress)
	assert.True(t, got[1].Rating)
}

func TestDerivePartStates_OverflowAbsorbsRemainder(t *testing.T) {
	// Part 2 has an unknown count → it absorbs every watched episode past
	// part 1's range so nothing is dropped.
	parts := []model.AniListPart{
		{ExternalID: "100", EpisodeCount: ptrInt(12)},
		{ExternalID: "200", EpisodeCount: nil},
	}

	got := service.DerivePartStates("watching", parts, seq(1, 20))
	require.Len(t, got, 2)
	assert.Equal(t, "COMPLETED", got[0].Status)
	assert.Equal(t, 12, got[0].Progress)
	// count==0 (unknown) can never reach COMPLETED; remainder lands here.
	assert.Equal(t, "CURRENT", got[1].Status)
	assert.Equal(t, 8, got[1].Progress)
}

func TestDerivePartStates_SkipsNonNumericID(t *testing.T) {
	parts := []model.AniListPart{
		{ExternalID: "100", EpisodeCount: ptrInt(12)},
		{ExternalID: "not-a-number", EpisodeCount: ptrInt(16)},
	}

	got := service.DerivePartStates("watching", parts, seq(1, 28))
	// The non-numeric part is dropped entirely; only the parsed part emits.
	require.Len(t, got, 1)
	assert.Equal(t, int64(100), got[0].MediaID)
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
	testutil.SetPartEpisodeCount(t, db, seasonID, "anilist", "113415", 12)
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

func TestPushSeasonState_MultiPartSplitsProgress(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Attack on Titan", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 4)
	// Final Season as one PlexTracker season of 28 episodes, mapped to two
	// AniList parts (12 + 16). 18 watched → part1 COMPLETED(12), part2 CURRENT(6).
	testutil.SetSeasonEpisodeCount(t, db, seasonID, 28)
	testutil.MarkEpisodesWatched(t, db, seasonID, 18)
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "100")
	testutil.InsertSeasonExternalID(t, db, seasonID, "anilist", "200")
	testutil.SetPartEpisodeCount(t, db, seasonID, "anilist", "100", 12)
	testutil.SetPartEpisodeCount(t, db, seasonID, "anilist", "200", 16)
	testutil.SetTitleStatus(t, db, titleID, "watching")
	testutil.SetSetting(t, db, "anilist_token", "test-token")

	fake := &fakeAniListClient{}
	svc := service.NewAniListPushService(db, fake, testutil.NopLogger())
	require.NoError(t, svc.PushSeasonState(context.Background(), seasonID))

	require.Len(t, fake.calls, 2)
	byMedia := make(map[int64]matching.SaveMediaListEntryInput, len(fake.calls))
	for _, c := range fake.calls {
		byMedia[c.MediaID] = c
	}
	require.Contains(t, byMedia, int64(100))
	require.Contains(t, byMedia, int64(200))
	assert.Equal(t, "COMPLETED", byMedia[100].Status)
	assert.Equal(t, 12, byMedia[100].Progress)
	assert.Equal(t, "CURRENT", byMedia[200].Status)
	assert.Equal(t, 6, byMedia[200].Progress)
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
