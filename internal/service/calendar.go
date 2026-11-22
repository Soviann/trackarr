package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

type CalendarService struct {
	writeDB     *sql.DB
	titleRepo   *repository.TitleRepository
	settingRepo *repository.SettingRepository
}

func NewCalendarService(
	writeDB *sql.DB,
	titleRepo *repository.TitleRepository,
	settingRepo *repository.SettingRepository,
) *CalendarService {
	return &CalendarService{
		writeDB:     writeDB,
		titleRepo:   titleRepo,
		settingRepo: settingRepo,
	}
}

// GetOrCreateToken returns the existing calendar token or creates a new 32-byte hex token.
func (s *CalendarService) GetOrCreateToken(ctx context.Context) (string, error) {
	tok, err := s.settingRepo.Get(repository.SettingKeyCalendarToken)
	if err == nil && tok != "" {
		return tok, nil
	}

	return s.RegenerateToken(ctx)
}

// RegenerateToken creates and stores a new random 32-byte hex token.
func (s *CalendarService) RegenerateToken(ctx context.Context) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate calendar token: %w", err)
	}
	token := hex.EncodeToString(b)

	err := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewSettingWriter(tx).Set(ctx, repository.SettingKeyCalendarToken, token)
	})
	if err != nil {
		return "", fmt.Errorf("save calendar token: %w", err)
	}

	return token, nil
}

// ValidateToken returns true if the provided token matches the stored calendar token.
func (s *CalendarService) ValidateToken(_ context.Context, token string) bool {
	if token == "" {
		return false
	}
	stored, err := s.settingRepo.Get(repository.SettingKeyCalendarToken)
	if err != nil || stored == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(stored)) == 1
}

// GetCalendarEvents returns calendar events within the given date range.
func (s *CalendarService) GetCalendarEvents(_ context.Context, fromDate, toDate string) ([]repository.CalendarEventItem, error) {
	return s.titleRepo.ListCalendarEvents(fromDate, toDate)
}

type episodeGroupKey struct {
	titleID int64
	airDate string
}

type icsEventEntry struct {
	airTime time.Time
	items   []repository.CalendarEventItem
}

type icsVEvent struct {
	uid         string
	dtstamp     string
	dtstart     string
	dtend       string
	summary     string
	description string
	category    string
	url         string
}

func (s *CalendarService) shouldGroupSameDayEpisodes() (bool, error) {
	if s.settingRepo == nil {
		return true, nil
	}
	val, err := s.settingRepo.Get(repository.SettingKeyICalGroupSameDay)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		return false, fmt.Errorf("fetch %s setting: %w", repository.SettingKeyICalGroupSameDay, err)
	}
	return !strings.EqualFold(strings.TrimSpace(val), "false"), nil
}

// GenerateICS builds a RFC 5545 compliant iCalendar feed.
func (s *CalendarService) GenerateICS(ctx context.Context, baseURL string) ([]byte, error) {
	now := time.Now().UTC()
	dtstamp := now.Format("20060102T150405Z")

	// Include past 30 days and next 180 days of airings
	fromDate := now.AddDate(0, 0, -30).Format("2006-01-02")
	toDate := now.AddDate(0, 0, 180).Format("2006-01-02")

	events, err := s.titleRepo.ListCalendarEvents(fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("fetch calendar events for ics: %w", err)
	}

	groupSameDay, err := s.shouldGroupSameDayEpisodes()
	if err != nil {
		return nil, err
	}

	var sb strings.Builder

	// VCALENDAR Header
	writeICSLine(&sb, "BEGIN:VCALENDAR")
	writeICSLine(&sb, "VERSION:2.0")
	writeICSLine(&sb, "PRODID:-//Trackarr//Media Calendar//EN")
	writeICSLine(&sb, "CALSCALE:GREGORIAN")
	writeICSLine(&sb, "METHOD:PUBLISH")
	writeICSLine(&sb, "X-WR-CALNAME:Trackarr Calendar")
	writeICSLine(&sb, "X-WR-CALDESC:Upcoming episodes and movie releases from your Trackarr watchlist")
	writeICSLine(&sb, "X-WR-TIMEZONE:UTC")
	writeICSLine(&sb, "REFRESH-INTERVAL;VALUE=DURATION:PT6H")
	writeICSLine(&sb, "X-PUBLISHED-TTL:PT6H")

	for _, entry := range buildICSEntries(events, groupSameDay) {
		var vevent icsVEvent
		if len(entry.items) > 1 {
			vevent = buildGroupedVEvent(entry.items, entry.airTime, dtstamp, baseURL)
		} else {
			vevent = buildSingleVEvent(entry.items[0], entry.airTime, dtstamp, baseURL)
		}
		writeICSEvent(&sb, vevent)
	}

	writeICSLine(&sb, "END:VCALENDAR")

	return []byte(sb.String()), nil
}

func buildICSEntries(events []repository.CalendarEventItem, groupSameDay bool) []icsEventEntry {
	var entries []icsEventEntry
	groupIdx := make(map[episodeGroupKey]int)

	for _, event := range events {
		airTime, parseErr := time.Parse("2006-01-02", event.AirDate)
		if parseErr != nil {
			continue
		}

		if groupSameDay && event.Type != string(model.TitleTypeMovie) && event.SeasonNumber != nil && event.EpisodeNumber != nil {
			key := episodeGroupKey{titleID: event.TitleID, airDate: event.AirDate}
			if idx, exists := groupIdx[key]; exists {
				entries[idx].items = append(entries[idx].items, event)
				continue
			}
			groupIdx[key] = len(entries)
		}

		entries = append(entries, icsEventEntry{
			airTime: airTime,
			items:   []repository.CalendarEventItem{event},
		})
	}

	return entries
}

func buildSingleVEvent(event repository.CalendarEventItem, airTime time.Time, dtstamp, baseURL string) icsVEvent {
	dtstart := airTime.Format("20060102")
	dtend := airTime.AddDate(0, 0, 1).Format("20060102") // All-day DTEND is exclusive
	category := icsEventCategory(event)

	summary := event.TitleName
	if event.SeasonNumber != nil && event.EpisodeNumber != nil {
		epTag := formatEpisodeTag(*event.SeasonNumber, *event.EpisodeNumber)
		if event.EpisodeName != nil && *event.EpisodeName != "" {
			summary = fmt.Sprintf("%s - %s - %s", event.TitleName, epTag, *event.EpisodeName)
		} else {
			summary = fmt.Sprintf("%s - %s", event.TitleName, epTag)
		}
	} else if event.NextAirEpisode != nil && *event.NextAirEpisode != "" {
		summary = fmt.Sprintf("%s (%s)", event.TitleName, *event.NextAirEpisode)
	}

	descParts := []string{
		fmt.Sprintf("Type: %s", category),
		fmt.Sprintf("Status: %s", event.Status),
	}
	if event.SeasonNumber != nil && event.EpisodeNumber != nil {
		descParts = append(descParts, fmt.Sprintf("Episode: %s", formatEpisodeWithName(event)))
	}
	descParts = appendICSTrailerDescription(descParts, event, baseURL)

	return icsVEvent{
		uid:         fmt.Sprintf("trackarr-%s@trackarr", event.ID),
		dtstamp:     dtstamp,
		dtstart:     dtstart,
		dtend:       dtend,
		summary:     summary,
		description: strings.Join(descParts, "\n"),
		category:    category,
		url:         icsTitleURL(baseURL, event.TitleID),
	}
}

func buildGroupedVEvent(group []repository.CalendarEventItem, airTime time.Time, dtstamp, baseURL string) icsVEvent {
	sort.SliceStable(group, func(i, j int) bool {
		si, sj := *group[i].SeasonNumber, *group[j].SeasonNumber
		if si != sj {
			return si < sj
		}
		ei, ej := *group[i].EpisodeNumber, *group[j].EpisodeNumber
		if ei != ej {
			return ei < ej
		}
		return group[i].ID < group[j].ID
	})

	first := group[0]
	dtstart := airTime.Format("20060102")
	dtend := airTime.AddDate(0, 0, 1).Format("20060102") // All-day DTEND is exclusive
	category := icsEventCategory(first)

	descParts := make([]string, 0, 3+len(group)+3)
	descParts = append(descParts,
		fmt.Sprintf("Type: %s", category),
		fmt.Sprintf("Status: %s", first.Status),
		fmt.Sprintf("Episodes (%d):", len(group)),
	)
	for _, ep := range group {
		descParts = append(descParts, "- "+formatEpisodeWithName(ep))
	}
	descParts = appendICSTrailerDescription(descParts, first, baseURL)

	return icsVEvent{
		uid:         fmt.Sprintf("trackarr-group-%d-%s@trackarr", first.TitleID, dtstart),
		dtstamp:     dtstamp,
		dtstart:     dtstart,
		dtend:       dtend,
		summary:     formatGroupedSummary(first.TitleName, group),
		description: strings.Join(descParts, "\n"),
		category:    category,
		url:         icsTitleURL(baseURL, first.TitleID),
	}
}

func formatGroupedSummary(titleName string, group []repository.CalendarEventItem) string {
	if isContiguousSameSeason(group) {
		first := group[0]
		last := group[len(group)-1]
		return fmt.Sprintf("%s - %s-E%02d", titleName, formatEpisodeTag(*first.SeasonNumber, *first.EpisodeNumber), *last.EpisodeNumber)
	}

	const maxSummaryTags = 3
	tagCount := min(len(group), maxSummaryTags)
	tags := make([]string, 0, tagCount)
	for i := range tagCount {
		tags = append(tags, formatEpisodeTag(*group[i].SeasonNumber, *group[i].EpisodeNumber))
	}

	summary := fmt.Sprintf("%s - %s", titleName, strings.Join(tags, ", "))
	if len(group) > maxSummaryTags {
		summary = fmt.Sprintf("%s (+%d)", summary, len(group)-maxSummaryTags)
	}
	return summary
}

func formatEpisodeTag(seasonNum, episodeNum int) string {
	return fmt.Sprintf("S%02dE%02d", seasonNum, episodeNum)
}

func formatEpisodeWithName(ep repository.CalendarEventItem) string {
	tag := formatEpisodeTag(*ep.SeasonNumber, *ep.EpisodeNumber)
	if ep.EpisodeName != nil && *ep.EpisodeName != "" {
		return fmt.Sprintf("%s (%s)", tag, *ep.EpisodeName)
	}
	return tag
}

func isContiguousSameSeason(group []repository.CalendarEventItem) bool {
	if len(group) < 2 {
		return false
	}
	season := *group[0].SeasonNumber
	for i := 1; i < len(group); i++ {
		if *group[i].SeasonNumber != season {
			return false
		}
		if *group[i].EpisodeNumber != *group[i-1].EpisodeNumber+1 {
			return false
		}
	}
	return true
}

func icsEventCategory(event repository.CalendarEventItem) string {
	if event.IsAnime {
		return "Anime"
	}
	if event.Type == string(model.TitleTypeMovie) {
		return "Movie"
	}
	return "Series"
}

func icsTitleURL(baseURL string, titleID int64) string {
	if baseURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/title/%d", baseURL, titleID)
}

func appendICSTrailerDescription(descParts []string, event repository.CalendarEventItem, baseURL string) []string {
	if len(event.WatchProviders) > 0 {
		provNames := make([]string, 0, len(event.WatchProviders))
		for _, wp := range event.WatchProviders {
			provNames = append(provNames, wp.Name)
		}
		descParts = append(descParts, fmt.Sprintf("Streaming: %s", strings.Join(provNames, ", ")))
	}
	if event.Overview != nil && *event.Overview != "" {
		descParts = append(descParts, fmt.Sprintf("Overview: %s", *event.Overview))
	}
	if url := icsTitleURL(baseURL, event.TitleID); url != "" {
		descParts = append(descParts, fmt.Sprintf("Trackarr: %s", url))
	}
	return descParts
}

func writeICSEvent(sb *strings.Builder, ev icsVEvent) {
	writeICSLine(sb, "BEGIN:VEVENT")
	writeICSLine(sb, fmt.Sprintf("UID:%s", ev.uid))
	writeICSLine(sb, fmt.Sprintf("DTSTAMP:%s", ev.dtstamp))
	writeICSLine(sb, fmt.Sprintf("DTSTART;VALUE=DATE:%s", ev.dtstart))
	writeICSLine(sb, fmt.Sprintf("DTEND;VALUE=DATE:%s", ev.dtend))
	writeICSLine(sb, fmt.Sprintf("SUMMARY:%s", escapeICS(ev.summary)))
	writeICSLine(sb, fmt.Sprintf("DESCRIPTION:%s", escapeICS(ev.description)))
	writeICSLine(sb, fmt.Sprintf("CATEGORIES:%s", ev.category))
	writeICSLine(sb, "STATUS:CONFIRMED")
	writeICSLine(sb, "TRANSP:TRANSPARENT")
	if ev.url != "" {
		writeICSLine(sb, fmt.Sprintf("URL:%s", ev.url))
	}
	writeICSLine(sb, "END:VEVENT")
}

// escapeICS escapes text for RFC 5545 format.
func escapeICS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `;`, `\;`)
	s = strings.ReplaceAll(s, `,`, `\,`)
	s = strings.ReplaceAll(s, "\r\n", `\n`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\n`)
	return s
}

// writeICSLine writes a folded RFC 5545 line ending in CRLF without splitting UTF-8 runes.
func writeICSLine(sb *strings.Builder, line string) {
	const maxOctets = 75
	if len(line) <= maxOctets {
		sb.WriteString(line)
		sb.WriteString("\r\n")
		return
	}

	// First line allows up to 75 octets; continuation lines start with a single
	// space (1 octet), leaving 74 octets for content.
	limit := maxOctets
	for len(line) > 0 {
		if len(line) <= limit {
			sb.WriteString(line)
			break
		}
		cut := limit
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		if cut == 0 {
			// Single rune larger than limit (impossible for valid UTF-8 <= 4 bytes)
			_, size := utf8.DecodeRuneInString(line)
			cut = size
		}
		sb.WriteString(line[:cut])
		sb.WriteString("\r\n ")
		line = line[cut:]
		limit = maxOctets - 1
	}
	sb.WriteString("\r\n")
}
