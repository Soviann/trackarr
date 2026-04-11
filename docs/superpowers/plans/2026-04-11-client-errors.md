# Client-Side Error Reporting — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `POST /api/client-errors` so frontend crash reports sent by `ErrorBoundary` are logged server-side.

**Architecture:** New handler file with a single method. Auth required (session cookie already present on every authenticated request). Rate-limited via the existing `RateLimit` middleware already used on other sensitive endpoints. No DB storage — WARN-level slog output is sufficient for a personal app.

**Tech Stack:** Go 1.24, chi, `log/slog`, testify/assert

---

## File Map

| Action | File |
|---|---|
| Create | `internal/handler/client_errors.go` |
| Create | `internal/handler/client_errors_test.go` |
| Modify | `cmd/serve.go` — register route |
| Modify | `frontend/src/components/ErrorBoundary.tsx` — remove TODO comment |

---

### Task 1: Handler

**Files:**
- Create: `internal/handler/client_errors.go`

- [x] **Step 1: Create the handler**

```go
package handler

import (
	"log/slog"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
)

type ClientErrorHandler struct{}

type clientErrorPayload struct {
	Message string `json:"message"`
	Stack   string `json:"stack,omitempty"`
}

func (h *ClientErrorHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	var payload clientErrorPayload
	if err := httputil.ReadJSON(r, &payload); err != nil {
		return httputil.APIError(http.StatusBadRequest, "invalid body")
	}
	if payload.Message == "" {
		return httputil.APIError(http.StatusBadRequest, "message is required")
	}
	slog.WarnContext(r.Context(), "[client-error]",
		"message", payload.Message,
		"stack", payload.Stack,
	)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
```

- [x] **Step 2: Write the test**

```go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
)

func TestClientErrorHandler_Handle(t *testing.T) {
	h := &handler.ClientErrorHandler{}

	t.Run("valid payload returns 204", func(t *testing.T) {
		body := `{"message":"TypeError: cannot read property","stack":"at App.tsx:42"}`
		r := httptest.NewRequest(http.MethodPost, "/api/client-errors", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		err := h.Handle(w, r)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("empty message returns 400", func(t *testing.T) {
		body := `{"message":""}`
		r := httptest.NewRequest(http.MethodPost, "/api/client-errors", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		err := h.Handle(w, r)
		var apiErr *httputil.Error
		assert.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/client-errors", strings.NewReader("{invalid"))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		err := h.Handle(w, r)
		var apiErr *httputil.Error
		assert.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})

	t.Run("stack is optional", func(t *testing.T) {
		body := `{"message":"render error"}`
		r := httptest.NewRequest(http.MethodPost, "/api/client-errors", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		err := h.Handle(w, r)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}
```

- [x] **Step 3: Run the test — expect PASS**

```bash
make test
```

Expected: all 4 sub-tests pass.

- [x] **Step 4: Commit**

```bash
git add internal/handler/client_errors.go internal/handler/client_errors_test.go
git commit -m "$(cat <<'EOF'
feat(handler): ajoute le handler POST /api/client-errors

Reçoit les rapports de crash frontend, valide le payload et loggue
en WARN via slog. Aucun stockage en base — les logs serveur suffisent.
EOF
)"
```

---

### Task 2: Route Registration

**Files:**
- Modify: `cmd/serve.go`

- [x] **Step 1: Locate the route registration block in `cmd/serve.go`**

Find where other authenticated routes are registered (look for `r.With(authMiddleware).Group(...)` or similar). Add the new route in the authenticated group, with rate limiting consistent with other write endpoints.

- [x] **Step 2: Register the route**

Add alongside existing authenticated routes:

```go
clientErrorHandler := &handler.ClientErrorHandler{}
r.With(authMiddleware).Post("/api/client-errors", httputil.WrapHandler(clientErrorHandler.Handle))
```

> Note: Check if a rate limiter is applied to this router group already. If not, add `r.With(authMiddleware, rateLimiter).Post(...)` — look at how `/api/auth/google` uses rate limiting for reference.

- [x] **Step 3: Build to verify no compile errors**

```bash
make build
```

Expected: exits 0.

- [x] **Step 4: Commit**

```bash
git add cmd/serve.go
git commit -m "$(cat <<'EOF'
feat(router): enregistre POST /api/client-errors dans le routeur chi

Route authentifiée pour la réception des rapports d'erreur frontend.
EOF
)"
```

---

### Task 3: Frontend Cleanup

**Files:**
- Modify: `frontend/src/components/ErrorBoundary.tsx`

- [x] **Step 1: Remove the TODO comment from `ErrorBoundary.tsx:22`**

Change:
```tsx
// TODO: implement POST /api/client-errors on the backend to persist these
try {
```

To:
```tsx
try {
```

- [x] **Step 2: Verify frontend builds**

```bash
make test-front
```

Expected: exits 0.

- [x] **Step 3: Commit**

```bash
git add frontend/src/components/ErrorBoundary.tsx
git commit -m "$(cat <<'EOF'
chore(frontend): supprime le TODO ErrorBoundary — endpoint implémenté
EOF
)"
```
