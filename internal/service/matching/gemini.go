package matching

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"

type GeminiClient struct {
	apiKeys    []string
	keyIndex   atomic.Int64
	httpClient *http.Client
	apiURL     string // overridable for tests
}

func NewGeminiClient(apiKeys []string) *GeminiClient {
	return &GeminiClient{
		apiKeys: apiKeys,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiURL: geminiAPIURL,
	}
}

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
func (c *GeminiClient) VerifyMatch(source PlexInfo, candidate MatchCandidate) (*MatchVerification, error) {
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

	body, err := c.generate(prompt)
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
func (c *GeminiClient) FuzzyResolve(source PlexInfo) (*FuzzyResolution, error) {
	prompt := fmt.Sprintf(`You are a media identification system. Given the following partial information, identify the most likely title.

Source (from Plex):
- Title: %q
- Year: %d
- Type: %s

Respond with ONLY a JSON object (no markdown, no explanation outside JSON):
{"candidate_title": "official title", "candidate_year": year, "confidence": "high"/"medium"/"low", "reason": "brief explanation"}`,
		source.Title, source.Year, source.Type)

	body, err := c.generate(prompt)
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

func (c *GeminiClient) generate(prompt string) (string, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Try each API key on 429
	for attempts := 0; attempts < len(c.apiKeys); attempts++ {
		idx := c.keyIndex.Load() % int64(len(c.apiKeys))
		apiKey := c.apiKeys[idx]

		url := fmt.Sprintf("%s?key=%s", c.apiURL, apiKey)
		resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			return "", err
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			// Rotate to next key
			c.keyIndex.Add(1)
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

// parseJSONFromResponse extracts JSON from a Gemini response that may contain markdown fences.
func parseJSONFromResponse(text string, dest interface{}) error {
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
