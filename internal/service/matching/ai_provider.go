package matching

import "context"

// AIProvider abstracts LLM-powered verification, fuzzy matching, and anime season identification.
// Implementations can provide Gemini, OpenAI, Anthropic Claude, or local models (e.g. Ollama).
type AIProvider interface {
	VerifyMatch(ctx context.Context, source PlexInfo, candidate MatchCandidate) (*MatchVerification, error)
	FuzzyResolve(ctx context.Context, source PlexInfo) (*FuzzyResolution, error)
	IdentifyAnimeSeason(ctx context.Context, title string, year int) (*AnimeSeasonIdentification, error)
}
