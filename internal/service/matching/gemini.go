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

	// Try each API key on 429. keyIndex.Add(1) is atomic so concurrent 429s
	// each claim a distinct slot rather than both landing on the same next key.
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
			return "", err
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("read gemini response: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
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

	return "", &APIError{Service: "Gemini", StatusCode: http.StatusTooManyRequests, Body: "all API keys rate-limited"}
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
