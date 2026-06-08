package service

import (
	"net/http"

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
