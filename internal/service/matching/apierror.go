package matching

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIError represents an HTTP error from an external API.
type APIError struct {
	Service    string
	StatusCode int
	Body       string
	RetryAfter time.Duration // parsed from Retry-After header, zero if absent
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s API error %d: %s", e.Service, e.StatusCode, e.Body)
}

// newAPIError creates an APIError, parsing the Retry-After header if present.
func newAPIError(service string, resp *http.Response, body []byte) *APIError {
	e := &APIError{
		Service:    service,
		StatusCode: resp.StatusCode,
		Body:       string(body),
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			e.RetryAfter = time.Duration(secs) * time.Second
		}
	}
	return e
}

// IsRetryableError returns true if the error is transient and the task should be retried.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// API errors: retry on 429 and 5xx
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500
	}

	// Network errors: retry on timeout, DNS, connection refused
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Catch common transient error patterns in wrapped errors
	msg := err.Error()
	for _, pattern := range []string{"connection refused", "no such host", "i/o timeout", "TLS handshake timeout"} {
		if strings.Contains(msg, pattern) {
			return true
		}
	}

	// "all Gemini API keys rate-limited" is retryable
	if strings.Contains(msg, "rate-limited") {
		return true
	}

	return false
}

// IsRateLimitError returns true if the error is a 429 Too Many Requests.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusTooManyRequests
	}

	if strings.Contains(err.Error(), "rate-limited") {
		return true
	}

	return false
}

// ExtractRetryAfter extracts the Retry-After duration from an error, if available.
func ExtractRetryAfter(err error) time.Duration {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.RetryAfter
	}
	return 0
}
