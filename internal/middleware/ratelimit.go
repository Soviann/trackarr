package middleware

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

const maxTrackedIPs = 10_000

type rateLimiter struct {
	mu       sync.Mutex
	attempts *lru.Cache[string, []time.Time]
	max      int
	window   time.Duration
}

// RateLimit restricts requests per IP to max within the given window.
// The ctx controls the lifetime of the background cleanup goroutine.
//
// The per-IP attempt map is backed by an LRU cache capped at maxTrackedIPs:
// when the cap is hit, the least-recently-seen IP is evicted to make room
// for a new one. This prevents a distributed attacker from filling the
// table with throwaway IPs and starving legitimate users of a fresh slot
// (previous behaviour rejected new IPs outright once the cap was reached).
func RateLimit(ctx context.Context, max int, window time.Duration) func(http.Handler) http.Handler {
	// lru.New only errors when size <= 0; maxTrackedIPs is a positive constant,
	// so the error is unreachable in practice — guard anyway to survive a future edit.
	cache, err := lru.New[string, []time.Time](maxTrackedIPs)
	if err != nil {
		log.Printf("ratelimit: LRU init failed (size=%d): %v", maxTrackedIPs, err)
		return func(next http.Handler) http.Handler { return next }
	}
	rl := &rateLimiter{
		attempts: cache,
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

	attempts, _ := rl.attempts.Get(ip)
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.max {
		rl.attempts.Add(ip, valid)
		return false
	}

	rl.attempts.Add(ip, append(valid, now))
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
			for _, ip := range rl.attempts.Keys() {
				attempts, ok := rl.attempts.Peek(ip)
				if !ok {
					continue
				}
				valid := attempts[:0]
				for _, t := range attempts {
					if t.After(cutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					rl.attempts.Remove(ip)
				} else {
					rl.attempts.Add(ip, valid)
				}
			}
			rl.mu.Unlock()
		}
	}
}
