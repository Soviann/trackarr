package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"
)

const maxTrackedIPs = 10_000

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
}

// RateLimit restricts requests per IP to max within the given window.
// The ctx controls the lifetime of the background cleanup goroutine.
func RateLimit(ctx context.Context, max int, window time.Duration) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
	}

	go rl.cleanup(ctx, window)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(r.RemoteAddr) {
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	attempts := rl.attempts[ip]
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.max {
		rl.attempts[ip] = valid
		return false
	}

	// Cap map size to prevent unbounded memory growth under distributed attack.
	if _, tracked := rl.attempts[ip]; !tracked && len(rl.attempts) >= maxTrackedIPs {
		return false
	}

	rl.attempts[ip] = append(valid, now)
	return true
}

// cleanup periodically removes stale IP entries. Stops when ctx is cancelled.
func (rl *rateLimiter) cleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-rl.window)
			for ip, attempts := range rl.attempts {
				valid := attempts[:0]
				for _, t := range attempts {
					if t.After(cutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(rl.attempts, ip)
				} else {
					rl.attempts[ip] = valid
				}
			}
			rl.mu.Unlock()
		}
	}
}
