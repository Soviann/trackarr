package matching

import (
	"context"

	"github.com/Soviann/trackarr/internal/model"
)

// AIProvider abstracts LLM-powered verification, fuzzy matching, and anime season identification.
// Implementations can provide Gemini, OpenAI, Anthropic Claude, or local models (e.g. Ollama).
type AIProvider interface {
	VerifyMatch(ctx context.Context, source PlexInfo, candidate MatchCandidate) (*MatchVerification, error)
	FuzzyResolve(ctx context.Context, source PlexInfo) (*FuzzyResolution, error)
	IdentifyAnimeSeason(ctx context.Context, title string, year int) (*AnimeSeasonIdentification, error)
	GenerateWrappedStory(ctx context.Context, rawStats *model.WrappedRawStats) (*model.WrappedAIPersona, error)
}
