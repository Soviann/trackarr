package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/golang-jwt/jwt/v5"
	lru "github.com/hashicorp/golang-lru/v2"
)

const (
	APIKeyContextKey   contextKey = "api_key"
	AuthTypeContextKey contextKey = "auth_type"

	apiKeyRateLimitMax    = 120
	apiKeyRateLimitWindow = 60 * time.Second
)

var apiKeyHeaderRegex = regexp.MustCompile(`^trck_live_[0-9a-f]{64}$`)

// apiKeyLimiter tracks per-key request history for IETF RateLimit headers.
type apiKeyLimiter struct {
	mu      sync.Mutex
	history *lru.Cache[int64, []time.Time]
	max     int
	window  time.Duration
}

func newAPIKeyLimiter(max int, window time.Duration) *apiKeyLimiter {
	cache, _ := lru.New[int64, []time.Time](1000)
	return &apiKeyLimiter{
		history: cache,
		max:     max,
		window:  window,
	}
}

func (l *apiKeyLimiter) check(keyID int64) (allowed bool, remaining int, resetSecs int, retryAfter int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	attempts, _ := l.history.Get(keyID)
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.max {
		oldest := valid[0]
		retryAfter = int(oldest.Add(l.window).Sub(now).Seconds()) + 1
		if retryAfter < 1 {
			retryAfter = 1
		}
		l.history.Add(keyID, valid)
		return false, 0, retryAfter, retryAfter
	}

	valid = append(valid, now)
	l.history.Add(keyID, valid)
	remaining = l.max - len(valid)

	resetSecs = int(l.window.Seconds())
	if len(valid) > 1 {
		resetSecs = int(valid[0].Add(l.window).Sub(now).Seconds()) + 1
		if resetSecs < 1 {
			resetSecs = 1
		}
	}
	return true, remaining, resetSecs, 0
}

// UnifiedAuth validates either an admin interactive session cookie ("token") or an API key.
// External clients authenticate via `Authorization: Bearer trck_live_...` or `X-Api-Key: trck_live_...`.
// If the Authorization header does not match the token regex, it is ignored and falls back to
// the cookie to prevent browser session disconnects.
func UnifiedAuth(jwtSecret string, apiKeySvc *service.APIKeyService) func(http.Handler) http.Handler {
	limiter := newAPIKeyLimiter(apiKeyRateLimitMax, apiKeyRateLimitWindow)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Check Authorization header
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
				if apiKeyHeaderRegex.MatchString(rawToken) {
					handleAPIKey(w, r, next, apiKeySvc, limiter, rawToken)
					return
				}
				// If not an API key format, fall through to cookie check to avoid breaking session cookies
			}

			// 2. Check X-Api-Key header
			if rawToken := strings.TrimSpace(r.Header.Get("X-Api-Key")); rawToken != "" {
				if !apiKeyHeaderRegex.MatchString(rawToken) {
					writeUnauthorized(w)
					return
				}

				handleAPIKey(w, r, next, apiKeySvc, limiter, rawToken)
				return
			}

			// 3. Fallback to interactive cookie session
			cookie, err := r.Cookie("token")
			if err == nil && cookie.Value != "" {
				token, err := jwt.Parse(cookie.Value, func(t *jwt.Token) (any, error) {
					return []byte(jwtSecret), nil
				}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithLeeway(30*time.Second))

				if err == nil && token.Valid {
					if claims, ok := token.Claims.(jwt.MapClaims); ok {
						email, _ := claims["email"].(string)
						sub, _ := claims["sub"].(string)
						userIdentifier := email
						if userIdentifier == "" {
							userIdentifier = sub
						}
						if userIdentifier != "" {
							ctx := context.WithValue(r.Context(), EmailKey, userIdentifier)
							ctx = context.WithValue(ctx, AuthTypeContextKey, "session")
							next.ServeHTTP(w, r.WithContext(ctx))
							return
						}
					}
				}
			}

			// 4. Neither valid API key nor session cookie
			writeUnauthorized(w)
		})
	}
}

func handleAPIKey(w http.ResponseWriter, r *http.Request, next http.Handler, apiKeySvc *service.APIKeyService, limiter *apiKeyLimiter, rawToken string) {
	key, err := apiKeySvc.AuthenticateToken(r.Context(), rawToken)
	if err != nil || key == nil {
		writeUnauthorized(w)
		return
	}

	allowed, remaining, resetSecs, retryAfter := limiter.check(key.ID)
	w.Header().Set("RateLimit-Limit", strconv.Itoa(apiKeyRateLimitMax))
	w.Header().Set("RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("RateLimit-Reset", strconv.Itoa(resetSecs))

	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":       "Too many requests",
			"retry_after": retryAfter,
		})
		return
	}

	ctx := context.WithValue(r.Context(), APIKeyContextKey, key)
	ctx = context.WithValue(ctx, AuthTypeContextKey, "api_key")
	next.ServeHTTP(w, r.WithContext(ctx))
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="Trackarr", error="invalid_token"`)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "Unauthorized",
	})
}

// RequireScope ensures that the caller has the required scope.
// Interactive sessions automatically bypass scope checks.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authType, _ := r.Context().Value(AuthTypeContextKey).(string)
			if authType == "session" {
				next.ServeHTTP(w, r)
				return
			}

			key, ok := r.Context().Value(APIKeyContextKey).(*model.APIKey)
			if !ok || key == nil {
				writeUnauthorized(w)
				return
			}

			if !key.HasScope(scope) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "Forbidden",
					"message": fmt.Sprintf("API key lacks required scope '%s'", scope),
					"code":    "insufficient_scope",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
