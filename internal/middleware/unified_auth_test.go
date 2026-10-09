package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/middleware"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnifiedAuth(t *testing.T) {
	writeDB, _, err := database.Open(":memory:")
	require.NoError(t, err)
	defer writeDB.Close()
	require.NoError(t, database.Migrate(writeDB))

	readRepo := repository.NewAPIKeyRepository(writeDB)
	apiKeySvc := service.NewAPIKeyService(writeDB, readRepo)
	jwtSecret := "test-secret"
	ctx := context.Background()

	// Create test API key
	key, rawToken, err := apiKeySvc.CreateKey(ctx, "Test Key", []string{model.ScopeLibraryRead, model.ScopeLibraryWrite})
	require.NoError(t, err)
	require.NotNil(t, key)

	// Handler under test requiring library:read
	handler := middleware.UnifiedAuth(jwtSecret, apiKeySvc)(
		middleware.RequireScope(model.ScopeLibraryRead)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}),
		),
	)

	// 1. Nominal: Bearer token auth
	{
		req := httptest.NewRequest("GET", "/api/titles", nil)
		req.Header.Set("Authorization", "Bearer "+rawToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "120", rr.Header().Get("RateLimit-Limit"))
		assert.NotEmpty(t, rr.Header().Get("RateLimit-Remaining"))
	}

	// Nominal: X-Api-Key header
	{
		req := httptest.NewRequest("GET", "/api/titles", nil)
		req.Header.Set("X-Api-Key", rawToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// Nominal: Cookie JWT auth
	{
		claims := jwt.MapClaims{"email": "admin@example.com"}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(jwtSecret))
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/api/titles", nil)
		req.AddCookie(&http.Cookie{Name: "token", Value: signed})
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// 2. Boundary: Insufficient Scope
	{
		deleteHandler := middleware.UnifiedAuth(jwtSecret, apiKeySvc)(
			middleware.RequireScope(model.ScopeLibraryDelete)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			),
		)

		req := httptest.NewRequest("DELETE", "/api/titles/1", nil)
		req.Header.Set("Authorization", "Bearer "+rawToken)
		rr := httptest.NewRecorder()
		deleteHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
		assert.Contains(t, rr.Body.String(), "insufficient_scope")
		assert.Contains(t, rr.Body.String(), "library:delete")
	}

	// 3. Error: Unauthorized (no credentials, invalid token, revoked token)
	{
		// No credentials
		req := httptest.NewRequest("GET", "/api/titles", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Header().Get("WWW-Authenticate"), "invalid_token")

		// Invalid token regex in X-Api-Key
		req2 := httptest.NewRequest("GET", "/api/titles", nil)
		req2.Header.Set("X-Api-Key", "invalid_token_format")
		rr2 := httptest.NewRecorder()
		handler.ServeHTTP(rr2, req2)
		assert.Equal(t, http.StatusUnauthorized, rr2.Code)

		// Authorization header with non-matching format should fallback to cookie (and 401 if no cookie)
		req3 := httptest.NewRequest("GET", "/api/titles", nil)
		req3.Header.Set("Authorization", "Bearer not_a_real_token")
		rr3 := httptest.NewRecorder()
		handler.ServeHTTP(rr3, req3)
		assert.Equal(t, http.StatusUnauthorized, rr3.Code)

		// Revoked key
		err := apiKeySvc.RevokeKey(ctx, key.ID)
		require.NoError(t, err)

		req4 := httptest.NewRequest("GET", "/api/titles", nil)
		req4.Header.Set("Authorization", "Bearer "+rawToken)
		rr4 := httptest.NewRecorder()
		handler.ServeHTTP(rr4, req4)
		assert.Equal(t, http.StatusUnauthorized, rr4.Code)
	}
}
