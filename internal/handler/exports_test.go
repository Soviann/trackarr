package handler

import "net/http"

// SetGoogleTokenInfoURLForTest swaps the upstream tokeninfo URL so tests can
// point GoogleCallback at a httptest.Server. Returns a restore func.
func SetGoogleTokenInfoURLForTest(u string) func() {
	prev := googleTokenInfoURL
	googleTokenInfoURL = u
	return func() { googleTokenInfoURL = prev }
}

// SetGoogleAuthClientForTest swaps the http.Client used by GoogleCallback.
// Tests use this to install an aggressive timeout without sleeping 5s.
func SetGoogleAuthClientForTest(c *http.Client) func() {
	prev := googleAuthClient
	googleAuthClient = c
	return func() { googleAuthClient = prev }
}
