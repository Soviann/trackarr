package matching

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
)

// TVDBSeriesData holds series data from the TVDB API.
type TVDBSeriesData struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Overview     string   `json:"overview"`
	Year         string   `json:"year"`
	Image        string   `json:"image"`
	Genres       []string `json:"genres"`
	AverageScore float64  `json:"score"`
}

// TVDBMovieData holds movie data from the TVDB API.
type TVDBMovieData struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Overview     string   `json:"overview"`
	Year         string   `json:"year"`
	Image        string   `json:"image"`
	Genres       []string `json:"genres"`
	AverageScore float64  `json:"score"`
}

type tvdbSeriesResponse struct {
	Data *tvdbSeriesDetail `json:"data"`
}

type tvdbMovieResponse struct {
	Data *tvdbMovieDetail `json:"data"`
}

type tvdbSeriesDetail struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Overview string `json:"overview"`
	Year     string `json:"year"`
	Image    string `json:"image"`
	Genres   []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Score   float64 `json:"score"`
	Runtime *int    `json:"averageRuntime"`
	// TVDB v4 returns status as an object ({id, name, recordType, keepUpdated}),
	// not a string. Decoding it as a string aborts the whole series fetch.
	Status struct {
		Name string `json:"name"`
	} `json:"status"`
	RemoteIDs    []tvdbRemoteID    `json:"remoteIds"`
	Translations *tvdbTranslations `json:"translations"`
}

type tvdbMovieDetail struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Overview string `json:"overview"`
	Year     string `json:"year"`
	Image    string `json:"image"`
	Genres   []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Score        float64           `json:"score"`
	Runtime      *int              `json:"runtime"`
	RemoteIDs    []tvdbRemoteID    `json:"remoteIds"`
	Translations *tvdbTranslations `json:"translations"`
}

type tvdbRemoteID struct {
	ID       string `json:"id"`
	SourceID int64  `json:"sourceId"`
	// sourceId 2 = IMDB, 5 = TMDB
}

type tvdbTranslations struct {
	NameTranslations     []tvdbTranslation `json:"nameTranslations"`
	OverviewTranslations []tvdbTranslation `json:"overviewTranslations"`
}

type tvdbTranslation struct {
	Language string `json:"language"`
	Name     string `json:"name"`
	Overview string `json:"overview"`
}

// GetSeriesDetails retrieves extended series details including genres, translations, and ratings.
func (c *TVDBClient) GetSeriesDetails(ctx context.Context, tvdbID int64) (*tvdbSeriesDetail, error) {
	var resp tvdbSeriesResponse
	params := url.Values{"meta": {"translations"}}
	if err := c.get(ctx, fmt.Sprintf("/series/%d/extended", tvdbID), params, &resp); err != nil {
		return nil, fmt.Errorf("tvdb series details: %w", err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("tvdb series %d: empty response", tvdbID)
	}
	return resp.Data, nil
}

// GetMovieDetails retrieves extended movie details.
func (c *TVDBClient) GetMovieDetails(ctx context.Context, tvdbID int64) (*tvdbMovieDetail, error) {
	var resp tvdbMovieResponse
	params := url.Values{"meta": {"translations"}}
	if err := c.get(ctx, fmt.Sprintf("/movies/%d/extended", tvdbID), params, &resp); err != nil {
		return nil, fmt.Errorf("tvdb movie details: %w", err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("tvdb movie %d: empty response", tvdbID)
	}
	return resp.Data, nil
}

// GetSeriesBySlug resolves a series slug to its TVDB ID.
func (c *TVDBClient) GetSeriesBySlug(ctx context.Context, slug string) (*tvdbSeriesDetail, error) {
	var resp tvdbSeriesResponse
	if err := c.get(ctx, fmt.Sprintf("/series/slug/%s", slug), nil, &resp); err != nil {
		return nil, fmt.Errorf("tvdb series slug %q: %w", slug, err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("tvdb series slug %q: empty response", slug)
	}
	return resp.Data, nil
}

// GetMovieBySlug resolves a movie slug to its TVDB ID.
func (c *TVDBClient) GetMovieBySlug(ctx context.Context, slug string) (*tvdbMovieDetail, error) {
	var resp tvdbMovieResponse
	if err := c.get(ctx, fmt.Sprintf("/movies/slug/%s", slug), nil, &resp); err != nil {
		return nil, fmt.Errorf("tvdb movie slug %q: %w", slug, err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("tvdb movie slug %q: empty response", slug)
	}
	return resp.Data, nil
}

// extractSeriesGenres returns genre names from a series detail.
func extractSeriesGenres(d *tvdbSeriesDetail) []string {
	names := make([]string, 0, len(d.Genres))
	for _, g := range d.Genres {
		if g.Name != "" {
			names = append(names, g.Name)
		}
	}
	return names
}

// extractMovieGenres returns genre names from a movie detail.
func extractMovieGenres(d *tvdbMovieDetail) []string {
	names := make([]string, 0, len(d.Genres))
	for _, g := range d.Genres {
		if g.Name != "" {
			names = append(names, g.Name)
		}
	}
	return names
}

// extractSeriesIMDB returns the IMDB ID from TVDB remote IDs (sourceId 2 = IMDB).
func extractSeriesIMDB(d *tvdbSeriesDetail) string {
	for _, r := range d.RemoteIDs {
		if r.SourceID == 2 && len(r.ID) > 0 {
			return r.ID
		}
	}
	return ""
}

// extractMovieIMDB returns the IMDB ID from TVDB remote IDs.
func extractMovieIMDB(d *tvdbMovieDetail) string {
	for _, r := range d.RemoteIDs {
		if r.SourceID == 2 && len(r.ID) > 0 {
			return r.ID
		}
	}
	return ""
}

// extractSeriesTMDB returns the TMDB ID from TVDB remote IDs (sourceId 5 = TMDB).
func extractSeriesTMDB(d *tvdbSeriesDetail) int64 {
	for _, r := range d.RemoteIDs {
		if r.SourceID == 5 && len(r.ID) > 0 {
			id, err := strconv.ParseInt(r.ID, 10, 64)
			if err != nil {
				log.Printf("tvdb: malformed remote id %q for sourceId %d", r.ID, r.SourceID)
				continue
			}
			return id
		}
	}
	return 0
}

// extractMovieTMDB returns the TMDB ID from TVDB remote IDs (sourceId 5 = TMDB).
func extractMovieTMDB(d *tvdbMovieDetail) int64 {
	for _, r := range d.RemoteIDs {
		if r.SourceID == 5 && len(r.ID) > 0 {
			id, err := strconv.ParseInt(r.ID, 10, 64)
			if err != nil {
				log.Printf("tvdb: malformed remote id %q for sourceId %d", r.ID, r.SourceID)
				continue
			}
			return id
		}
	}
	return 0
}

// extractSeriesNames returns en/fr names from TVDB translations.
func extractSeriesNames(d *tvdbSeriesDetail) map[string]string {
	result := make(map[string]string)
	if d.Translations == nil {
		return result
	}
	for _, t := range d.Translations.NameTranslations {
		if (t.Language == "eng" || t.Language == "fra") && t.Name != "" {
			lang := "en"
			if t.Language == "fra" {
				lang = "fr"
			}
			result[lang] = t.Name
		}
	}
	return result
}

// extractMovieNames returns en/fr names from TVDB movie translations.
func extractMovieNames(d *tvdbMovieDetail) map[string]string {
	result := make(map[string]string)
	if d.Translations == nil {
		return result
	}
	for _, t := range d.Translations.NameTranslations {
		if (t.Language == "eng" || t.Language == "fra") && t.Name != "" {
			lang := "en"
			if t.Language == "fra" {
				lang = "fr"
			}
			result[lang] = t.Name
		}
	}
	return result
}

// extractSeriesOverview returns the English overview from TVDB translations.
func extractSeriesOverview(d *tvdbSeriesDetail) string {
	if d.Translations == nil {
		return d.Overview
	}
	for _, t := range d.Translations.OverviewTranslations {
		if t.Language == "eng" && t.Overview != "" {
			return t.Overview
		}
	}
	return d.Overview
}

// extractMovieOverview returns the English overview from TVDB movie translations,
// falling back to the top-level overview field.
func extractMovieOverview(d *tvdbMovieDetail) string {
	if d.Translations != nil {
		for _, t := range d.Translations.OverviewTranslations {
			if t.Language == "eng" && t.Overview != "" {
				return t.Overview
			}
		}
	}
	return d.Overview
}
