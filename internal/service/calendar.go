package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/database"
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

	for _, event := range events {
		t, parseErr := time.Parse("2006-01-02", event.AirDate)
		if parseErr != nil {
			continue
		}
		dtstart := t.Format("20060102")
		dtend := t.AddDate(0, 0, 1).Format("20060102") // All-day DTEND is exclusive

		uid := fmt.Sprintf("trackarr-%s@trackarr", event.ID)

		// Summary
		summary := event.TitleName
		if event.SeasonNumber != nil && event.EpisodeNumber != nil {
			epTag := fmt.Sprintf("S%02dE%02d", *event.SeasonNumber, *event.EpisodeNumber)
			if event.EpisodeName != nil && *event.EpisodeName != "" {
				summary = fmt.Sprintf("%s - %s - %s", event.TitleName, epTag, *event.EpisodeName)
			} else {
				summary = fmt.Sprintf("%s - %s", event.TitleName, epTag)
			}
		} else if event.NextAirEpisode != nil && *event.NextAirEpisode != "" {
			summary = fmt.Sprintf("%s (%s)", event.TitleName, *event.NextAirEpisode)
		}

		// Categories
		category := "Series"
		if event.IsAnime {
			category = "Anime"
		} else if event.Type == "movie" {
			category = "Movie"
		}

		// Description
		var descParts []string
		descParts = append(descParts, fmt.Sprintf("Type: %s", category))
		descParts = append(descParts, fmt.Sprintf("Status: %s", event.Status))
		if event.SeasonNumber != nil && event.EpisodeNumber != nil {
			epStr := fmt.Sprintf("S%02dE%02d", *event.SeasonNumber, *event.EpisodeNumber)
			if event.EpisodeName != nil && *event.EpisodeName != "" {
				epStr += fmt.Sprintf(" (%s)", *event.EpisodeName)
			}
			descParts = append(descParts, fmt.Sprintf("Episode: %s", epStr))
		}
		if len(event.WatchProviders) > 0 {
			var provNames []string
			for _, wp := range event.WatchProviders {
				provNames = append(provNames, wp.Name)
			}
			descParts = append(descParts, fmt.Sprintf("Streaming: %s", strings.Join(provNames, ", ")))
		}
		if event.Overview != nil && *event.Overview != "" {
			descParts = append(descParts, fmt.Sprintf("Overview: %s", *event.Overview))
		}
		if baseURL != "" {
			descParts = append(descParts, fmt.Sprintf("Trackarr: %s/title/%d", baseURL, event.TitleID))
		}
		description := strings.Join(descParts, "\n")

		writeICSLine(&sb, "BEGIN:VEVENT")
		writeICSLine(&sb, fmt.Sprintf("UID:%s", uid))
		writeICSLine(&sb, fmt.Sprintf("DTSTAMP:%s", dtstamp))
		writeICSLine(&sb, fmt.Sprintf("DTSTART;VALUE=DATE:%s", dtstart))
		writeICSLine(&sb, fmt.Sprintf("DTEND;VALUE=DATE:%s", dtend))
		writeICSLine(&sb, fmt.Sprintf("SUMMARY:%s", escapeICS(summary)))
		writeICSLine(&sb, fmt.Sprintf("DESCRIPTION:%s", escapeICS(description)))
		writeICSLine(&sb, fmt.Sprintf("CATEGORIES:%s", category))
		writeICSLine(&sb, "STATUS:CONFIRMED")
		writeICSLine(&sb, "TRANSP:TRANSPARENT")
		if baseURL != "" {
			writeICSLine(&sb, fmt.Sprintf("URL:%s/title/%d", baseURL, event.TitleID))
		}
		writeICSLine(&sb, "END:VEVENT")
	}

	writeICSLine(&sb, "END:VCALENDAR")

	return []byte(sb.String()), nil
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

// writeICSLine writes a folded RFC 5545 line ending in CRLF.
func writeICSLine(sb *strings.Builder, line string) {
	const maxLen = 75
	if len(line) <= maxLen {
		sb.WriteString(line)
		sb.WriteString("\r\n")
		return
	}

	// Line folding: limit lines to 75 octets, continuations prefixed with a space
	for len(line) > maxLen {
		sb.WriteString(line[:maxLen])
		sb.WriteString("\r\n ")
		line = line[maxLen:]
	}
	sb.WriteString(line)
	sb.WriteString("\r\n")
}
