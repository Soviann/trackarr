package service

import (
	"context"

	"golang.org/x/time/rate"
)

// APILimiter wraps rate.Limiter for external API rate limiting.
type APILimiter struct {
	limiter *rate.Limiter
}

// NewAPILimiter creates a limiter that allows rps requests per second with burst b.
func NewAPILimiter(rps float64, burst int) *APILimiter {
	return &APILimiter{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
}

// Wait blocks until the rate limiter allows the request.
func (l *APILimiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}
