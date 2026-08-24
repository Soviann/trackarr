package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// La recherche TMDB du rematch affiche les jaquettes directement depuis le CDN
// d'images TMDB : la CSP doit l'autoriser, sinon le navigateur bloque toutes
// les vignettes (régression introduite par l'audit sécurité).
func TestSecurityHeadersCSPAllowsTMDBImages(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	imgSrc := ""
	for _, directive := range strings.Split(csp, ";") {
		if strings.HasPrefix(strings.TrimSpace(directive), "img-src ") {
			imgSrc = directive
		}
	}
	assert.Contains(t, imgSrc, "https://image.tmdb.org")
}
