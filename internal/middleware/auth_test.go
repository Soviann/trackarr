package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	mw "github.com/nicolasvasse/plextracker/internal/middleware"
	"github.com/stretchr/testify/assert"
)

func TestJWTAuth_ValidToken(t *testing.T) {
	secret := "test-secret"
	token := createTestToken(t, secret, "user@test.com", time.Now().Add(time.Hour))

	handler := mw.JWTAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTAuth_MissingToken(t *testing.T) {
	handler := mw.JWTAuth("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	token := createTestToken(t, secret, "user@test.com", time.Now().Add(-time.Hour))

	handler := mw.JWTAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func createTestToken(t *testing.T, secret, email string, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{"email": email, "exp": jwt.NewNumericDate(exp)}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
