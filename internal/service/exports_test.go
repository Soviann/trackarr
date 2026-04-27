package service

import "net/http"

// SetPushHTTPClientForTest swaps the package-level pushHTTPClient so push
// tests can inject a stub RoundTripper. Returns a restore func.
func SetPushHTTPClientForTest(c *http.Client) func() {
	prev := pushHTTPClient
	pushHTTPClient = c
	return func() { pushHTTPClient = prev }
}
