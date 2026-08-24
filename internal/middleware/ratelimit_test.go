package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimit_LimitsByIPIgnoringPort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := RateLimit(ctx, 2, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request 1 from IP 192.168.1.50 with port 10001 -> OK
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "192.168.1.50:10001"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Request 2 from IP 192.168.1.50 with different ephemeral port 10002 -> OK
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "192.168.1.50:10002"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)

	// Request 3 from IP 192.168.1.50 with different port 10003 -> 429 Too Many Requests
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.RemoteAddr = "192.168.1.50:10003"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusTooManyRequests, rec3.Code)

	// Request from another IP 192.168.1.51 -> OK (separate quota)
	reqOther := httptest.NewRequest(http.MethodGet, "/", nil)
	reqOther.RemoteAddr = "192.168.1.51:10001"
	recOther := httptest.NewRecorder()
	handler.ServeHTTP(recOther, reqOther)
	assert.Equal(t, http.StatusOK, recOther.Code)
}

func TestClientIP(t *testing.T) {
	assert.Equal(t, "192.168.1.1", clientIP("192.168.1.1:8080"))
	assert.Equal(t, "2001:db8::1", clientIP("[2001:db8::1]:443"))
	assert.Equal(t, "127.0.0.1", clientIP("127.0.0.1"))
	assert.Equal(t, "unknown-format", clientIP("unknown-format"))
}
