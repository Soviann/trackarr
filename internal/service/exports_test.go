package service

import (
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

// SetPushHTTPClientForTest swaps the package-level pushHTTPClient so push
// tests can inject a stub RoundTripper. Returns a restore func.
func SetPushHTTPClientForTest(c *http.Client) func() {
	prev := pushHTTPClient
	pushHTTPClient = c
	return func() { pushHTTPClient = prev }
}

// BuildEnrichmentUpdateForTest exposes buildEnrichmentUpdate to the external
// test package so its ID-lock and PreserveMatch behavior can be asserted.
func BuildEnrichmentUpdateForTest(result *matching.MatchResult, payload EnrichmentPayload) repository.TitleUpdate {
	return buildEnrichmentUpdate(result, payload)
}

// IsSearchSourceForTest exposes isSearchSource for unit tests.
func IsSearchSourceForTest(source string) bool {
	return isSearchSource(source)
}

// ResolvedNameForTest exposes resolvedName for unit tests.
func ResolvedNameForTest(result *matching.MatchResult, payload EnrichmentPayload) string {
	return resolvedName(result, payload)
}

// Season-action kinds re-exported (as plain int) for the external test package.
var (
	SeasonActionNoneForTest       = int(seasonActionNone)
	SeasonActionLegacyForTest     = int(seasonActionLegacy)
	SeasonActionLegacyRootForTest = int(seasonActionLegacyRoot)
	SeasonActionMergeIntoForTest  = int(seasonActionMergeInto)
	SeasonActionCreateRootForTest = int(seasonActionCreateRoot)
)

// SeasonActionForTest mirrors seasonAction for assertions in the external test
// package, exposing the otherwise-unexported decision fields.
type SeasonActionForTest struct {
	Kind     int
	ParentID int64
	Offset   int
}

// DecideSeasonActionForTest exposes decideSeasonAction to the external test
// package so the franchise-protection rule table is unit-testable.
func DecideSeasonActionForTest(chain *matching.SeasonChain, result *matching.MatchResult, parentByIDs, parentByRoot *model.Title) SeasonActionForTest {
	a := decideSeasonAction(chain, result, parentByIDs, parentByRoot)
	return SeasonActionForTest{Kind: int(a.Kind), ParentID: a.ParentID, Offset: a.Offset}
}
