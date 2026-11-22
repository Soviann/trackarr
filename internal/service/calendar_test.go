package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

func TestCalendarService_GenerateICS_UTF8Folding(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	calSvc := service.NewCalendarService(db, titleRepo, settingRepo)

	today := time.Now().UTC().Format("2006-01-02")
	longJapaneseName := "華野超美（ごにんめ）華野超美（ごにんめ）華野超美（ごにんめ）華野超美（ごにんめ）"

	seriesID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, IsAnime: true, Year: 2026, Status: model.TitleStatusPlanToWatch, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "A Certain Dark Item", Language: "en", IsPrimary: true}},
	)
	season := testutil.UpsertSeason(t, db, seriesID, 1, 12)
	testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
		{EpisodeNumber: 3, Name: longJapaneseName, AirDate: today},
	})

	icsBytes, err := calSvc.GenerateICS(context.Background(), "https://trackarr.example.com")
	require.NoError(t, err)
	assert.True(t, utf8.Valid(icsBytes), "ICS output must be valid UTF-8 after line folding")
	for _, line := range strings.Split(strings.TrimSuffix(string(icsBytes), "\r\n"), "\r\n") {
		assert.LessOrEqual(t, len(line), 75, "folded line exceeds 75 octets: %q", line)
	}
	unfolded := strings.ReplaceAll(string(icsBytes), "\r\n ", "")
	assert.Contains(t, unfolded, longJapaneseName)
}

func parseICSVEvents(t *testing.T, icsBytes []byte) []map[string]string {
	t.Helper()
	require.True(t, utf8.Valid(icsBytes), "ICS output must be valid UTF-8")
	raw := string(icsBytes)
	require.True(t, strings.HasSuffix(raw, "\r\n"), "ICS output must end with CRLF")

	for _, line := range strings.Split(strings.TrimSuffix(raw, "\r\n"), "\r\n") {
		assert.LessOrEqual(t, len(line), 75, "folded line exceeds 75 octets: %q", line)
	}

	unfolded := strings.ReplaceAll(raw, "\r\n ", "")
	var events []map[string]string
	var current map[string]string

	for _, line := range strings.Split(strings.TrimSuffix(unfolded, "\r\n"), "\r\n") {
		switch line {
		case "BEGIN:VEVENT":
			require.Nil(t, current, "nested BEGIN:VEVENT")
			current = make(map[string]string)
		case "END:VEVENT":
			require.NotNil(t, current, "END:VEVENT without BEGIN:VEVENT")
			events = append(events, current)
			current = nil
		default:
			if current != nil {
				k, v, ok := strings.Cut(line, ":")
				require.True(t, ok, "malformed VEVENT line %q", line)
				current[k] = v
			}
		}
	}
	require.Nil(t, current, "unclosed BEGIN:VEVENT")
	return events
}

func TestCalendarService_GenerateICS_GroupingContiguous(t *testing.T) {
	t.Run("anime contiguous episodes with metadata and missing episode name", func(t *testing.T) {
		db := testutil.NewTestDB(t)
		calSvc := service.NewCalendarService(db, repository.NewTitleRepository(db), repository.NewSettingRepository(db))

		airTime := time.Now().UTC()
		airDate := airTime.Format("2006-01-02")
		wantStart := airTime.Format("20060102")
		wantEnd := airTime.AddDate(0, 0, 1).Format("20060102")

		overview := "After the party of heroes defeated the Demon King."
		seriesID := testutil.CreateTitle(t, db,
			&model.Title{
				Type:        model.TitleTypeSeries,
				IsAnime:     true,
				Year:        2026,
				Status:      model.TitleStatusWatching,
				MatchStatus: model.MatchStatusConfirmed,
				Overview:    &overview,
			},
			[]model.TitleName{{Name: "Frieren", Language: "en", IsPrimary: true}},
		)
		wpJSON, err := json.Marshal([]model.WatchProvider{
			{ID: 283, Name: "Crunchyroll"},
			{ID: 8, Name: "Netflix"},
		})
		require.NoError(t, err)
		wpStr := string(wpJSON)
		testutil.UpdateTitle(t, db, seriesID, repository.TitleUpdate{WatchProviders: &wpStr})

		season := testutil.UpsertSeason(t, db, seriesID, 1, 12)
		testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
			{EpisodeNumber: 1, Name: "The Journey's End", AirDate: airDate},
			{EpisodeNumber: 2, Name: "It Didn't Have to Be Magic", AirDate: airDate},
			{EpisodeNumber: 3, Name: "", AirDate: airDate},
		})

		icsBytes, err := calSvc.GenerateICS(context.Background(), "https://trackarr.example.com")
		require.NoError(t, err)

		events := parseICSVEvents(t, icsBytes)
		require.Len(t, events, 1)

		ev := events[0]
		assert.Equal(t, fmt.Sprintf("trackarr-group-%d-%s@trackarr", seriesID, wantStart), ev["UID"])
		assert.Equal(t, wantStart, ev["DTSTART;VALUE=DATE"])
		assert.Equal(t, wantEnd, ev["DTEND;VALUE=DATE"])
		assert.Equal(t, "Anime", ev["CATEGORIES"])
		assert.Equal(t, "CONFIRMED", ev["STATUS"])
		assert.Equal(t, "TRANSPARENT", ev["TRANSP"])
		assert.Equal(t, fmt.Sprintf("https://trackarr.example.com/title/%d", seriesID), ev["URL"])
		assert.Equal(t, "Frieren - S01E01-E03", ev["SUMMARY"])

		wantDesc := strings.Join([]string{
			"Type: Anime",
			"Status: watching",
			"Episodes (3):",
			"- S01E01 (The Journey's End)",
			"- S01E02 (It Didn't Have to Be Magic)",
			"- S01E03",
			`Streaming: Crunchyroll\, Netflix`,
			"Overview: After the party of heroes defeated the Demon King.",
			fmt.Sprintf("Trackarr: https://trackarr.example.com/title/%d", seriesID),
		}, `\n`)
		assert.Equal(t, wantDesc, ev["DESCRIPTION"])
	})

	t.Run("contiguous more than 3 episodes still formats as range and omits URL when baseURL empty", func(t *testing.T) {
		db := testutil.NewTestDB(t)
		calSvc := service.NewCalendarService(db, repository.NewTitleRepository(db), repository.NewSettingRepository(db))

		airDate := time.Now().UTC().Format("2006-01-02")
		seriesID := testutil.InsertTitle(t, db, "Andor", false)
		season := testutil.UpsertSeason(t, db, seriesID, 1, 12)
		testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
			{EpisodeNumber: 1, Name: "Kassa", AirDate: airDate},
			{EpisodeNumber: 2, Name: "That Would Be Me", AirDate: airDate},
			{EpisodeNumber: 3, Name: "Reckoning", AirDate: airDate},
			{EpisodeNumber: 4, Name: "Aldhani", AirDate: airDate},
			{EpisodeNumber: 5, Name: "The Axe Forgets", AirDate: airDate},
		})

		icsBytes, err := calSvc.GenerateICS(context.Background(), "")
		require.NoError(t, err)

		events := parseICSVEvents(t, icsBytes)
		require.Len(t, events, 1)

		ev := events[0]
		assert.Equal(t, "Series", ev["CATEGORIES"])
		assert.Equal(t, "Andor - S01E01-E05", ev["SUMMARY"])
		_, hasURL := ev["URL"]
		assert.False(t, hasURL)
		assert.NotContains(t, ev["DESCRIPTION"], "Trackarr:")
		assert.Contains(t, ev["DESCRIPTION"], `Episodes (5):\n- S01E01 (Kassa)\n- S01E02 (That Would Be Me)\n- S01E03 (Reckoning)\n- S01E04 (Aldhani)\n- S01E05 (The Axe Forgets)`)
	})
}

func TestCalendarService_GenerateICS_GroupingSummaryVariants(t *testing.T) {
	airDate := time.Now().UTC().Format("2006-01-02")

	cases := []struct {
		name        string
		episodes    map[int][]repository.EpisodeUpsert
		wantSummary string
		wantDescSeq []string
	}{
		{
			name: "non-contiguous two episodes",
			episodes: map[int][]repository.EpisodeUpsert{
				1: {
					{EpisodeNumber: 1, Name: "First", AirDate: airDate},
					{EpisodeNumber: 3, Name: "Third", AirDate: airDate},
				},
			},
			wantSummary: `Show - S01E01\, S01E03`,
			wantDescSeq: []string{"Episodes (2):", "- S01E01 (First)", "- S01E03 (Third)"},
		},
		{
			name: "non-contiguous exactly three episodes has no remainder suffix",
			episodes: map[int][]repository.EpisodeUpsert{
				1: {
					{EpisodeNumber: 1, Name: "One", AirDate: airDate},
					{EpisodeNumber: 2, Name: "Two", AirDate: airDate},
					{EpisodeNumber: 4, Name: "Four", AirDate: airDate},
				},
			},
			wantSummary: `Show - S01E01\, S01E02\, S01E04`,
			wantDescSeq: []string{"Episodes (3):", "- S01E01 (One)", "- S01E02 (Two)", "- S01E04 (Four)"},
		},
		{
			name: "non-contiguous more than three episodes truncates summary but lists all in description",
			episodes: map[int][]repository.EpisodeUpsert{
				1: {
					{EpisodeNumber: 9, Name: "Nine", AirDate: airDate},
					{EpisodeNumber: 1, Name: "One", AirDate: airDate},
					{EpisodeNumber: 5, Name: "Five", AirDate: airDate},
					{EpisodeNumber: 3, Name: "Three", AirDate: airDate},
					{EpisodeNumber: 7, Name: "Seven", AirDate: airDate},
				},
			},
			wantSummary: `Show - S01E01\, S01E03\, S01E05 (+2)`,
			wantDescSeq: []string{"Episodes (5):", "- S01E01 (One)", "- S01E03 (Three)", "- S01E05 (Five)", "- S01E07 (Seven)", "- S01E09 (Nine)"},
		},
		{
			name: "multi-season same day sorts season ascending then episode ascending",
			episodes: map[int][]repository.EpisodeUpsert{
				2: {
					{EpisodeNumber: 1, Name: "Premiere", AirDate: airDate},
				},
				1: {
					{EpisodeNumber: 12, Name: "Finale", AirDate: airDate},
				},
			},
			wantSummary: `Show - S01E12\, S02E01`,
			wantDescSeq: []string{"Episodes (2):", "- S01E12 (Finale)", "- S02E01 (Premiere)"},
		},
		{
			name: "multi-season more than three episodes uses (+N) suffix",
			episodes: map[int][]repository.EpisodeUpsert{
				1: {
					{EpisodeNumber: 11, Name: "Penultimate", AirDate: airDate},
					{EpisodeNumber: 12, Name: "Finale", AirDate: airDate},
				},
				2: {
					{EpisodeNumber: 1, Name: "Part 1", AirDate: airDate},
					{EpisodeNumber: 2, Name: "Part 2", AirDate: airDate},
				},
			},
			wantSummary: `Show - S01E11\, S01E12\, S02E01 (+1)`,
			wantDescSeq: []string{"Episodes (4):", "- S01E11 (Penultimate)", "- S01E12 (Finale)", "- S02E01 (Part 1)", "- S02E02 (Part 2)"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.NewTestDB(t)
			calSvc := service.NewCalendarService(db, repository.NewTitleRepository(db), repository.NewSettingRepository(db))

			seriesID := testutil.InsertTitle(t, db, "Show", false)
			// Insert higher season numbers first to verify deterministic sorting regardless of insert order.
			for sNum := 2; sNum >= 1; sNum-- {
				eps, ok := tc.episodes[sNum]
				if !ok {
					continue
				}
				season := testutil.UpsertSeason(t, db, seriesID, sNum, 24)
				testutil.UpsertEpisodesBatch(t, db, season.ID, eps)
			}

			icsBytes, err := calSvc.GenerateICS(context.Background(), "https://trackarr.example.com")
			require.NoError(t, err)

			events := parseICSVEvents(t, icsBytes)
			require.Len(t, events, 1)
			assert.Equal(t, tc.wantSummary, events[0]["SUMMARY"])
			assert.Contains(t, events[0]["DESCRIPTION"], strings.Join(tc.wantDescSeq, `\n`))
		})
	}
}

func TestCalendarService_GenerateICS_NonGroupedBoundaries(t *testing.T) {
	db := testutil.NewTestDB(t)
	calSvc := service.NewCalendarService(db, repository.NewTitleRepository(db), repository.NewSettingRepository(db))

	now := time.Now().UTC()
	day1 := now.Format("2006-01-02")
	day2 := now.AddDate(0, 0, 1).Format("2006-01-02")
	day3 := now.AddDate(0, 0, 2).Format("2006-01-02")

	// 1. Series A with two distinct TitleIDs sharing the exact same display name on day1
	// (Title A1 has 1 unwatched + 1 watched on day1 -> only 1 unwatched -> not grouped;
	//  Title A1 also has 1 unwatched on day2 -> separate VEVENT on day2)
	titleA1 := testutil.InsertTitle(t, db, "Duplicate Name", false)
	sA1 := testutil.UpsertSeason(t, db, titleA1, 1, 10)
	epA1D1Unwatched := testutil.SeedEpisode(t, db, sA1.ID, 1, day1, false)
	testutil.SeedEpisode(t, db, sA1.ID, 2, day1, true) // watched -> excluded
	epA1D2 := testutil.SeedEpisode(t, db, sA1.ID, 3, day2, false)

	titleA2 := testutil.InsertTitle(t, db, "Duplicate Name", false)
	sA2 := testutil.UpsertSeason(t, db, titleA2, 1, 10)
	epA2D1 := testutil.SeedEpisode(t, db, sA2.ID, 1, day1, false)

	// 2. Two movies sharing the same release_date (day1)
	movie1ID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2026, Status: model.TitleStatusPlanToWatch, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &day1},
		[]model.TitleName{{Name: "Movie Alpha", Language: "en", IsPrimary: true}},
	)
	movie2ID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeMovie, Year: 2026, Status: model.TitleStatusPlanToWatch, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &day1},
		[]model.TitleName{{Name: "Movie Beta", Language: "en", IsPrimary: true}},
	)

	// 3. Series with next_air_date fallback (no episodes on day3)
	nextEp := "S02E01"
	fallbackID := testutil.CreateTitle(t, db,
		&model.Title{
			Type:           model.TitleTypeSeries,
			Year:           2026,
			Status:         model.TitleStatusWatching,
			MatchStatus:    model.MatchStatusConfirmed,
			NextAirDate:    &day3,
			NextAirEpisode: &nextEp,
		},
		[]model.TitleName{{Name: "Fallback Show", Language: "en", IsPrimary: true}},
	)

	icsBytes, err := calSvc.GenerateICS(context.Background(), "https://trackarr.example.com")
	require.NoError(t, err)

	events := parseICSVEvents(t, icsBytes)
	require.Len(t, events, 6)

	uids := make(map[string]map[string]string, len(events))
	for _, ev := range events {
		assert.NotContains(t, ev["UID"], "trackarr-group-")
		uids[ev["UID"]] = ev
	}

	assert.Contains(t, uids, fmt.Sprintf("trackarr-ep-%d@trackarr", epA1D1Unwatched.ID))
	assert.Contains(t, uids, fmt.Sprintf("trackarr-ep-%d@trackarr", epA2D1.ID))
	assert.Contains(t, uids, fmt.Sprintf("trackarr-ep-%d@trackarr", epA1D2.ID))
	assert.Contains(t, uids, fmt.Sprintf("trackarr-title-%d-%s@trackarr", movie1ID, day1))
	assert.Contains(t, uids, fmt.Sprintf("trackarr-title-%d-%s@trackarr", movie2ID, day1))

	fallbackUID := fmt.Sprintf("trackarr-title-%d-%s@trackarr", fallbackID, day3)
	require.Contains(t, uids, fallbackUID)
	assert.Equal(t, "Fallback Show (S02E01)", uids[fallbackUID]["SUMMARY"])
}

func TestCalendarService_GenerateICS_SettingAndNilRepo(t *testing.T) {
	airDate := time.Now().UTC().Format("2006-01-02")

	t.Run("nil settingRepo defaults to grouped", func(t *testing.T) {
		db := testutil.NewTestDB(t)
		calSvc := service.NewCalendarService(db, repository.NewTitleRepository(db), nil)

		seriesID := testutil.InsertTitle(t, db, "Solo Leveling", true)
		season := testutil.UpsertSeason(t, db, seriesID, 1, 12)
		testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
			{EpisodeNumber: 1, Name: "I'm Used to It", AirDate: airDate},
			{EpisodeNumber: 2, Name: "If I Had One More Chance", AirDate: airDate},
		})

		icsBytes, err := calSvc.GenerateICS(context.Background(), "https://trackarr.example.com")
		require.NoError(t, err)

		events := parseICSVEvents(t, icsBytes)
		require.Len(t, events, 1)
		assert.Equal(t, "Solo Leveling - S01E01-E02", events[0]["SUMMARY"])
	})

	settingCases := []struct {
		name       string
		value      string
		wantEvents int
	}{
		{name: "exact false disables grouping", value: "false", wantEvents: 2},
		{name: "uppercase trimmed false disables grouping", value: " FALSE ", wantEvents: 2},
		{name: "mixed-case false disables grouping", value: "False", wantEvents: 2},
		{name: "true keeps grouping enabled", value: "true", wantEvents: 1},
		{name: "empty value defaults to grouped", value: "", wantEvents: 1},
		{name: "arbitrary non-false value defaults to grouped", value: "yes", wantEvents: 1},
	}

	for _, tc := range settingCases {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.NewTestDB(t)
			settingRepo := repository.NewSettingRepository(db)
			calSvc := service.NewCalendarService(db, repository.NewTitleRepository(db), settingRepo)

			seriesID := testutil.InsertTitle(t, db, "Dan Da Dan", true)
			season := testutil.UpsertSeason(t, db, seriesID, 1, 12)
			testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
				{EpisodeNumber: 1, Name: "Ep 1", AirDate: airDate},
				{EpisodeNumber: 2, Name: "Ep 2", AirDate: airDate},
			})

			testutil.SetSetting(t, db, repository.SettingKeyICalGroupSameDay, tc.value)

			icsBytes, err := calSvc.GenerateICS(context.Background(), "https://trackarr.example.com")
			require.NoError(t, err)

			events := parseICSVEvents(t, icsBytes)
			require.Len(t, events, tc.wantEvents)
			if tc.wantEvents == 1 {
				assert.Contains(t, events[0]["UID"], "trackarr-group-")
			} else {
				assert.Contains(t, events[0]["UID"], "trackarr-ep-")
				assert.Contains(t, events[1]["UID"], "trackarr-ep-")
			}
		})
	}

	t.Run("error when settingRepo query fails unexpectedly", func(t *testing.T) {
		titleDB := testutil.NewTestDB(t)
		closedSettingDB, _, err := database.Open(":memory:")
		require.NoError(t, err)
		require.NoError(t, database.Migrate(closedSettingDB))
		require.NoError(t, closedSettingDB.Close())

		calSvc := service.NewCalendarService(
			titleDB,
			repository.NewTitleRepository(titleDB),
			repository.NewSettingRepository(closedSettingDB),
		)

		_, err = calSvc.GenerateICS(context.Background(), "https://trackarr.example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ical_group_same_day")
	})

	t.Run("error when titleRepo query fails", func(t *testing.T) {
		closedTitleDB, _, err := database.Open(":memory:")
		require.NoError(t, err)
		require.NoError(t, database.Migrate(closedTitleDB))
		require.NoError(t, closedTitleDB.Close())

		settingDB := testutil.NewTestDB(t)
		calSvc := service.NewCalendarService(
			settingDB,
			repository.NewTitleRepository(closedTitleDB),
			repository.NewSettingRepository(settingDB),
		)

		_, err = calSvc.GenerateICS(context.Background(), "https://trackarr.example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fetch calendar events for ics")
	})
}

func TestCalendarService_GenerateICS_GroupedSpecialCharsAndUTF8(t *testing.T) {
	db := testutil.NewTestDB(t)
	calSvc := service.NewCalendarService(db, repository.NewTitleRepository(db), repository.NewSettingRepository(db))

	airDate := time.Now().UTC().Format("2006-01-02")
	titleName := `Steins;Gate, 0\2 華野超美`
	overview := "Line 1; with, comma\\slash\nLine 2 華野超美（ごにんめ）🎬🔥"

	seriesID := testutil.CreateTitle(t, db,
		&model.Title{
			Type:        model.TitleTypeSeries,
			IsAnime:     true,
			Year:        2026,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
			Overview:    &overview,
		},
		[]model.TitleName{{Name: titleName, Language: "en", IsPrimary: true}},
	)
	season := testutil.UpsertSeason(t, db, seriesID, 1, 24)
	testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
		{EpisodeNumber: 1, Name: `Prologue; Part, 1\A 華野超美`, AirDate: airDate},
		{EpisodeNumber: 2, Name: "Time Leap\n華野超美（ごにんめ）華野超美（ごにんめ）華野超美（ごにんめ）", AirDate: airDate},
	})

	icsBytes, err := calSvc.GenerateICS(context.Background(), "https://trackarr.example.com")
	require.NoError(t, err)

	events := parseICSVEvents(t, icsBytes)
	require.Len(t, events, 1)

	assert.Equal(t, `Steins\;Gate\, 0\\2 華野超美 - S01E01-E02`, events[0]["SUMMARY"])
	assert.Contains(t, events[0]["DESCRIPTION"], `- S01E01 (Prologue\; Part\, 1\\A 華野超美)`)
	assert.Contains(t, events[0]["DESCRIPTION"], `- S01E02 (Time Leap\n華野超美（ごにんめ）華野超美（ごにんめ）華野超美（ごにんめ）)`)
	assert.Contains(t, events[0]["DESCRIPTION"], `Overview: Line 1\; with\, comma\\slash\nLine 2 華野超美（ごにんめ）🎬🔥`)
}
