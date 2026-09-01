package matching

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Soviann/trackarr/internal/model"
)

const geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.6-flash:generateContent"

type GeminiClient struct {
	apiKeys    []string
	keyIndex   atomic.Int64
	httpClient *http.Client
	apiURL     string // overridable for tests
}

func NewGeminiClient(apiKeys []string) *GeminiClient {
	c := &GeminiClient{
		apiKeys: apiKeys,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiURL: geminiAPIURL,
	}
	// Start at len-1 so the first Add(1) in generate() yields index 0.
	if len(apiKeys) > 0 {
		c.keyIndex.Store(int64(len(apiKeys) - 1))
	}
	return c
}

// SetBaseURL overrides the Gemini base URL (for tests).
func (c *GeminiClient) SetBaseURL(u string) { c.apiURL = u }

type MatchVerification struct {
	Confirmed  bool   `json:"confirmed"`
	Confidence string `json:"confidence"` // "high", "medium", "low"
	Reason     string `json:"reason"`
}

type FuzzyResolution struct {
	CandidateTitle string `json:"candidate_title"`
	CandidateYear  int    `json:"candidate_year"`
	Confidence     string `json:"confidence"`
	Reason         string `json:"reason"`
}

type PlexInfo struct {
	Title string
	Year  int
	Type  string // "movie", "series", "anime"
}

type MatchCandidate struct {
	Title  string
	Year   int
	TMDBID int64
	IMDBID string
}

// VerifyMatch asks Gemini to verify whether a candidate match is correct for the source.
func (c *GeminiClient) VerifyMatch(ctx context.Context, source PlexInfo, candidate MatchCandidate) (*MatchVerification, error) {
	if c == nil {
		return nil, nil
	}
	prompt := fmt.Sprintf(`You are a media matching verification system. Determine if the candidate match is correct for the source title.

Source (from Plex):
- Title: %q
- Year: %d
- Type: %s

Candidate match:
- Title: %q
- Year: %d
- TMDB ID: %d
- IMDB ID: %s

Respond with ONLY a JSON object (no markdown, no explanation outside JSON):
{"confirmed": true/false, "confidence": "high"/"medium"/"low", "reason": "brief explanation"}`,
		source.Title, source.Year, source.Type,
		candidate.Title, candidate.Year, candidate.TMDBID, candidate.IMDBID)

	body, err := c.generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("verify match: %w", err)
	}

	var result MatchVerification
	if err := parseJSONFromResponse(body, &result); err != nil {
		return nil, fmt.Errorf("parse verify response: %w", err)
	}
	return &result, nil
}

// FuzzyResolve asks Gemini to identify a title from partial/ambiguous info.
func (c *GeminiClient) FuzzyResolve(ctx context.Context, source PlexInfo) (*FuzzyResolution, error) {
	if c == nil {
		return nil, nil
	}
	prompt := fmt.Sprintf(`You are a media identification system. Given the following partial information, identify the most likely title.

Source (from Plex):
- Title: %q
- Year: %d
- Type: %s

Respond with ONLY a JSON object (no markdown, no explanation outside JSON):
{"candidate_title": "official title", "candidate_year": year, "confidence": "high"/"medium"/"low", "reason": "brief explanation"}`,
		source.Title, source.Year, source.Type)

	body, err := c.generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("fuzzy resolve: %w", err)
	}

	var result FuzzyResolution
	if err := parseJSONFromResponse(body, &result); err != nil {
		return nil, fmt.Errorf("parse fuzzy response: %w", err)
	}
	return &result, nil
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (c *GeminiClient) generate(ctx context.Context, prompt string) (string, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Try each API key on 429 or transient 5xx errors. keyIndex.Add(1) is atomic so concurrent failures
	// each claim a distinct slot rather than both landing on the same next key.
	var lastErr error
	for attempts := 0; attempts < len(c.apiKeys); attempts++ {
		idx := c.keyIndex.Add(1) % int64(len(c.apiKeys))
		apiKey := c.apiKeys[idx]

		url := fmt.Sprintf("%s?key=%s", c.apiURL, apiKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read gemini response: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusInternalServerError ||
			resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusGatewayTimeout {
			lastErr = newAPIError("Gemini", resp, respBody)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return "", newAPIError("Gemini", resp, respBody)
		}

		var gResp geminiResponse
		if err := json.Unmarshal(respBody, &gResp); err != nil {
			return "", fmt.Errorf("decode gemini response: %w", err)
		}

		if len(gResp.Candidates) == 0 || len(gResp.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("empty gemini response")
		}

		return gResp.Candidates[0].Content.Parts[0].Text, nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("all API keys failed (last error: %w)", lastErr)
	}
	return "", &APIError{Service: "Gemini", StatusCode: http.StatusTooManyRequests, Body: "all API keys exhausted"}
}

type AnimeSeasonIdentification struct {
	IsSeason         bool   `json:"is_season"`
	ParentSeriesName string `json:"parent_series_name"`
	SeasonNumber     int    `json:"season_number"`
	Confidence       string `json:"confidence"`
	Reason           string `json:"reason"`
}

// IdentifyAnimeSeason asks Gemini if a title is actually a specific season of a series.
// Useful for anime where sequels have different names (e.g. "Solo Leveling: Arise from the Shadow" -> Season 2).
func (c *GeminiClient) IdentifyAnimeSeason(ctx context.Context, title string, year int) (*AnimeSeasonIdentification, error) {
	if c == nil {
		return nil, nil
	}
	prompt := fmt.Sprintf(`You are an anime metadata expert. Identify if the following title is a specific season of a parent series.
Many anime sequels have distinct names instead of "Season 2".

Input:
- Title: %q
- Year: %d

Respond with ONLY a JSON object:
{
  "is_season": true/false,
  "parent_series_name": "The main series name (e.g. 'Solo Leveling')",
  "season_number": 1, 2, 3, etc,
  "confidence": "high"/"medium"/"low",
  "reason": "brief explanation"
}`, title, year)

	body, err := c.generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("identify anime season: %w", err)
	}

	var result AnimeSeasonIdentification
	if err := parseJSONFromResponse(body, &result); err != nil {
		return nil, fmt.Errorf("parse identification response: %w", err)
	}
	return &result, nil
}

// parseJSONFromResponse extracts JSON from a Gemini response that may contain markdown fences.
func parseJSONFromResponse(text string, dest any) error {
	// Strip markdown code fences if present
	cleaned := text
	if idx := strings.Index(cleaned, "```json"); idx != -1 {
		cleaned = cleaned[idx+7:]
	} else if idx := strings.Index(cleaned, "```"); idx != -1 {
		cleaned = cleaned[idx+3:]
	}
	if idx := strings.LastIndex(cleaned, "```"); idx != -1 {
		cleaned = cleaned[:idx]
	}
	cleaned = strings.TrimSpace(cleaned)

	return json.Unmarshal([]byte(cleaned), dest)
}

// GenerateWrappedStory generates an AI-powered personality, annual summary, witty quote, and tailored fun facts.
func (c *GeminiClient) GenerateWrappedStory(ctx context.Context, stats *model.WrappedRawStats) (*model.WrappedAIPersona, error) {
	if c == nil || len(c.apiKeys) == 0 || stats == nil {
		return FallbackWrappedPersona(stats), nil
	}

	statsJSON, err := json.Marshal(stats)
	if err != nil {
		return FallbackWrappedPersona(stats), nil
	}

	prompt := fmt.Sprintf(`You are Trackarr Wrapped's creative narrator and pop-culture commentator.
Given the following raw annual viewing statistics for the user in year %d:
%s

Generate an engaging, witty, perceptive, and concise annual retrospective in JSON format.
CRITICAL CONSTRAINT: Keep all texts short and punchy so that the entire persona fits cleanly on a single mobile screen without scrolling.

Requirements:
1. "title": A catchy, evocative viewing archetype title (max 4-5 words, e.g. "The Nocturnal Crime Sleuth", "The Weekend Marathoner").
2. "badges": An array of exactly 2-3 short badge labels (1-2 words each, e.g. ["Night Owl", "Binge Legend"]).
3. "quote": Exactly 1 short, witty, memorable one-liner quote (max 15 words).
4. "summary": Exactly 1 concise sentence celebrating their year in media (max 25 words).
5. "fun_facts": An array of exactly 2 short, crisp bullet points highlighting surprising numbers from their raw stats (each max 12-15 words).

Respond with ONLY a valid JSON object:
{
  "title": "string",
  "badges": ["badge 1", "badge 2"],
  "quote": "string",
  "summary": "string",
  "fun_facts": ["fact 1", "fact 2"]
}`, stats.Year, string(statsJSON))

	body, err := c.generate(ctx, prompt)
	if err != nil {
		return FallbackWrappedPersona(stats), nil
	}

	var result model.WrappedAIPersona
	if err := parseJSONFromResponse(body, &result); err != nil || result.Title == "" {
		return FallbackWrappedPersona(stats), nil
	}

	if len(result.FunFacts) == 0 {
		fallback := FallbackWrappedPersona(stats)
		result.FunFacts = fallback.FunFacts
	}

	return &result, nil
}

// FallbackWrappedPersona generates deterministic personas and insights when AI is unavailable.
func FallbackWrappedPersona(stats *model.WrappedRawStats) *model.WrappedAIPersona {
	if stats == nil {
		return &model.WrappedAIPersona{
			Title:    "The Dedicated Viewer",
			Summary:  "A steady and enjoyable year of media discovery.",
			Quote:    "Another year, another watchlist conquered.",
			FunFacts: []string{"Tracked and saved for posterity."},
			Badges:   []string{"Viewer"},
		}
	}

	title := "The Eclectic Explorer"
	summary := fmt.Sprintf("You logged %d titles and %d episodes in %d, spanning rich genres and thrilling stories.", stats.TotalTitles, stats.EpisodesWatched, stats.Year)
	quote := "One more episode never hurt anyone."
	badges := []string{"Dedicated Streamer"}

	switch {
	case stats.TotalAnime > stats.TotalMovies && stats.TotalAnime > stats.TotalSeries:
		title = "The Anime Connoisseur"
		summary = fmt.Sprintf("Anime was your undisputed domain in %d with %d titles tracked and endless seasons enjoyed.", stats.Year, stats.TotalAnime)
		quote = "Subbed over dubbed, always and forever."
		badges = append(badges, "Anime Devotee")
	case stats.NightOwlPct >= 50:
		title = "The Midnight Binger"
		summary = fmt.Sprintf("You thrived in the quiet hours of the night in %d, watching %d%% of your screen time after dark.", stats.Year, stats.NightOwlPct)
		quote = "Sleep is optional, the next cliffhanger is not."
		badges = append(badges, "Night Owl")
	case stats.TotalMovies >= stats.TotalSeries && stats.TotalMovies > 0:
		title = "The Celluloid Cinephile"
		summary = fmt.Sprintf("A true lover of feature films, you immersed yourself in %d cinema gems throughout %d.", stats.TotalMovies, stats.Year)
		quote = "Cinema is a matter of what's in the frame and what's out."
		badges = append(badges, "Cinephile")
	case stats.LongestBingeEps >= 6:
		title = "The Marathon Champion"
		summary = fmt.Sprintf("When you commit to a story, nothing stops you — powering through %d episodes in a single day.", stats.LongestBingeEps)
		quote = "Are you still watching? Absolutely."
		badges = append(badges, "Binge Legend")
	}

	var funFacts []string
	if stats.NightOwlPct > 0 {
		funFacts = append(funFacts, fmt.Sprintf("%d%% of your watch sessions happened after 8 PM.", stats.NightOwlPct))
	}
	if stats.LongestBingeEps >= 3 && stats.LongestBingeTitle != "" {
		funFacts = append(funFacts, fmt.Sprintf("Your biggest binge was %d episodes of %s in a single day.", stats.LongestBingeEps, stats.LongestBingeTitle))
	} else if stats.BestStreakDays >= 3 {
		funFacts = append(funFacts, fmt.Sprintf("You maintained a peak daily watch streak of %d consecutive days.", stats.BestStreakDays))
	}
	if stats.PeakDayOfWeek != "" {
		funFacts = append(funFacts, fmt.Sprintf("%s was your favorite day of the week to watch.", stats.PeakDayOfWeek))
	} else if len(stats.TopGenres) > 0 {
		funFacts = append(funFacts, fmt.Sprintf("Your most-watched genre was %s.", stats.TopGenres[0]))
	}

	if len(funFacts) == 0 {
		funFacts = []string{
			fmt.Sprintf("Logged %d titles in %d.", stats.TotalTitles, stats.Year),
			"A dedicated year of media tracking on Trackarr.",
		}
	}

	return &model.WrappedAIPersona{
		Title:    title,
		Summary:  summary,
		Quote:    quote,
		FunFacts: funFacts,
		Badges:   badges,
	}
}
