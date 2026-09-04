package matching

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
)

type AniListSearchResult struct {
	ID           int64  `json:"id"`
	RomajiTitle  string `json:"romajiTitle"`
	EnglishTitle string `json:"englishTitle"`
	Episodes     *int   `json:"episodes"`
	Format       string `json:"format"` // TV, MOVIE, OVA, ONA, SPECIAL, MUSIC
	SeasonYear   *int   `json:"seasonYear"`
	MALID        *int64 `json:"idMal"`
	CoverURL     string `json:"coverURL,omitempty"`
}

func (r *AniListSearchResult) DisplayTitle() string {
	if r.EnglishTitle != "" {
		return r.EnglishTitle
	}
	return r.RomajiTitle
}

type AniListDetails struct {
	ID              int64    `json:"id"`
	MALID           *int64   `json:"idMal"`
	RomajiTitle     string   `json:"romajiTitle"`
	EnglishTitle    string   `json:"englishTitle"`
	Episodes        *int     `json:"episodes"`
	Format          string   `json:"format"`
	SeasonYear      *int     `json:"seasonYear"`
	CoverURL        string   `json:"coverURL"` // extraLarge or large
	AverageScore    *int     `json:"averageScore"`
	Description     string   `json:"description"` // synopsis, HTML stripped to plain text
	Genres          []string `json:"genres"`
	Duration        *int     `json:"duration"`        // minutes per episode
	StartDate       *string  `json:"startDate"`       // ISO YYYY-MM-DD, nil if unknown
	CountryOfOrigin *string  `json:"countryOfOrigin"` // ISO-3166-1 alpha-2, nil if AniList has none
}

type AniListNames struct {
	Romaji  string
	English string
}

const searchAnimeQuery = `
query ($search: String) {
  Page(perPage: 10) {
    media(search: $search, type: ANIME) {
      id
      idMal
      title { romaji english }
      episodes
      format
      seasonYear
      coverImage { extraLarge large }
    }
  }
}
`

const getAnimeDetailsQuery = `
query ($id: Int) {
  Media(id: $id, type: ANIME) {
    id
    idMal
    title { romaji english }
    episodes
    duration
    format
    seasonYear
    averageScore
    genres
    countryOfOrigin
    description(asHtml: false)
    coverImage { extraLarge large }
    startDate { year month day }
  }
}
`

var (
	anilistBreakRe = regexp.MustCompile(`(?i)<br\s*/?>|</p>`)
	anilistTagRe   = regexp.MustCompile(`<[^>]*>`)
	anilistBlankRe = regexp.MustCompile(`\n{3,}`)
)

// cleanAniListDescription turns AniList's lightly-marked-up synopsis into plain
// text: block breaks become newlines, remaining tags are dropped, and HTML
// entities are decoded. AniList returns <br>/<i>/<b> even with asHtml:false.
func cleanAniListDescription(raw string) string {
	if raw == "" {
		return ""
	}
	s := anilistBreakRe.ReplaceAllString(raw, "\n")
	s = anilistTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = anilistBlankRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func (c *AniListClient) SearchAnime(ctx context.Context, title string) ([]AniListSearchResult, error) {
	var resp struct {
		Page struct {
			Media []struct {
				ID    int64  `json:"id"`
				MALID *int64 `json:"idMal"`
				Title struct {
					Romaji  string `json:"romaji"`
					English string `json:"english"`
				} `json:"title"`
				Episodes   *int   `json:"episodes"`
				Format     string `json:"format"`
				SeasonYear *int   `json:"seasonYear"`
				CoverImage struct {
					ExtraLarge string `json:"extraLarge"`
					Large      string `json:"large"`
				} `json:"coverImage"`
			} `json:"media"`
		} `json:"Page"`
	}

	err := c.query(ctx, searchAnimeQuery, map[string]any{"search": title}, "", &resp)
	if err != nil {
		return nil, fmt.Errorf("search anime: %w", err)
	}

	results := make([]AniListSearchResult, len(resp.Page.Media))
	for i, m := range resp.Page.Media {
		cover := m.CoverImage.ExtraLarge
		if cover == "" {
			cover = m.CoverImage.Large
		}
		results[i] = AniListSearchResult{
			ID:           m.ID,
			MALID:        m.MALID,
			RomajiTitle:  m.Title.Romaji,
			EnglishTitle: m.Title.English,
			Episodes:     m.Episodes,
			Format:       m.Format,
			SeasonYear:   m.SeasonYear,
			CoverURL:     cover,
		}
	}
	return results, nil
}

func (c *AniListClient) GetAnimeDetails(ctx context.Context, anilistID int64) (*AniListDetails, error) {
	var resp struct {
		Media struct {
			ID    int64  `json:"id"`
			MALID *int64 `json:"idMal"`
			Title struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
			} `json:"title"`
			Episodes        *int     `json:"episodes"`
			Duration        *int     `json:"duration"`
			Format          string   `json:"format"`
			SeasonYear      *int     `json:"seasonYear"`
			AverageScore    *int     `json:"averageScore"`
			Genres          []string `json:"genres"`
			CountryOfOrigin string   `json:"countryOfOrigin"`
			Description     string   `json:"description"`
			CoverImage      struct {
				ExtraLarge string `json:"extraLarge"`
				Large      string `json:"large"`
			} `json:"coverImage"`
			StartDate struct {
				Year  *int `json:"year"`
				Month *int `json:"month"`
				Day   *int `json:"day"`
			} `json:"startDate"`
		} `json:"Media"`
	}

	err := c.query(ctx, getAnimeDetailsQuery, map[string]any{"id": anilistID}, "", &resp)
	if err != nil {
		return nil, fmt.Errorf("get anime details: %w", err)
	}

	coverURL := resp.Media.CoverImage.ExtraLarge
	if coverURL == "" {
		coverURL = resp.Media.CoverImage.Large
	}

	return &AniListDetails{
		ID:              resp.Media.ID,
		MALID:           resp.Media.MALID,
		RomajiTitle:     resp.Media.Title.Romaji,
		EnglishTitle:    resp.Media.Title.English,
		Episodes:        resp.Media.Episodes,
		Format:          resp.Media.Format,
		SeasonYear:      resp.Media.SeasonYear,
		CoverURL:        coverURL,
		AverageScore:    resp.Media.AverageScore,
		Description:     cleanAniListDescription(resp.Media.Description),
		Genres:          resp.Media.Genres,
		Duration:        resp.Media.Duration,
		StartDate:       formatAniListDate(resp.Media.StartDate.Year, resp.Media.StartDate.Month, resp.Media.StartDate.Day),
		CountryOfOrigin: normalizeCountry(resp.Media.CountryOfOrigin),
	}, nil
}

// normalizeCountry uppercases/trims an ISO-3166-1 code, returning nil when empty.
func normalizeCountry(c string) *string {
	c = strings.ToUpper(strings.TrimSpace(c))
	if c == "" {
		return nil
	}
	return &c
}

// formatAniListDate renders AniList's {year,month,day} as ISO YYYY-MM-DD,
// zero-padding missing month/day to 01. Returns nil when year is absent
// (undated entry) so ordering falls back to external_id.
func formatAniListDate(y, m, d *int) *string {
	if y == nil {
		return nil
	}
	mm, dd := 1, 1
	if m != nil {
		mm = *m
	}
	if d != nil {
		dd = *d
	}
	s := fmt.Sprintf("%04d-%02d-%02d", *y, mm, dd)
	return &s
}

// GetNames returns romaji and English names for an anime.
func (c *AniListClient) GetNames(ctx context.Context, anilistID int64) (*AniListNames, error) {
	details, err := c.GetAnimeDetails(ctx, anilistID)
	if err != nil {
		return nil, err
	}
	return &AniListNames{
		Romaji:  details.RomajiTitle,
		English: details.EnglishTitle,
	}, nil
}
