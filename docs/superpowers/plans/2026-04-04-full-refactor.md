# PlexTracker Full Refactor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve maintainability, performance, and code quality across the entire PlexTracker application in 6 incremental phases.

**Architecture:** Hybrid approach — transversal foundations first (error handling, helpers, frontend state/styling), then domain-by-domain refactoring (titles, matching, episodes), finishing with quality improvements (tests, lint, docs).

**Tech Stack:** Go 1.24, chi, SQLite, Preact 10, Vite, Zustand, CSS Modules, `golang.org/x/time/rate`, `@testing-library/preact`

---

## File Structure Overview

### New files (backend)

| File | Responsibility |
|---|---|
| `internal/handler/httputil/response.go` | WriteJSON, ReadJSON helpers |
| `internal/handler/httputil/request.go` | ParseIDParam, ParseQueryInt helpers |
| `internal/handler/httputil/errors.go` | APIError type + HandlerFunc wrapper + error middleware |
| `internal/handler/httputil/response_test.go` | Tests for response helpers |
| `internal/handler/httputil/request_test.go` | Tests for request helpers |
| `internal/handler/httputil/errors_test.go` | Tests for error handling |
| `internal/repository/title_search.go` | FTS5, LIKE fallback, Levenshtein fuzzy search |
| `internal/repository/title_loader.go` | Batch loading of names, seasons, episodes |
| `internal/repository/title_search_test.go` | Tests for search |
| `internal/repository/title_loader_test.go` | Tests for batch loading |
| `internal/service/ratelimiter.go` | API rate limiter wrapping `x/time/rate` |
| `.golangci.yml` | Linter configuration |

### New files (frontend)

| File | Responsibility |
|---|---|
| `frontend/src/store.ts` | Zustand store for title cache + invalidation |
| `frontend/src/utils.ts` | getName, getTypeLabel, formatDate helpers |
| `frontend/src/components/ErrorBanner.tsx` | Reusable error display component |
| `frontend/src/components/ErrorBanner.module.css` | Styles for ErrorBanner |
| CSS module files per component (progressive migration) | |

### Modified files

All existing handler files (`title.go`, `auth.go`, `episode.go`, `webhook.go`, `settings.go`, `push.go`, `anilist_auth.go`, `season.go`) — migrated to use httputil helpers and error pattern.

`internal/repository/title.go` — reduced to CRUD only (search and loader extracted).

`internal/router/router.go` — uses error middleware wrapper.

`internal/service/background.go` — uses `x/time/rate` instead of `time.Sleep`.

All frontend pages and components — migrated to CSS Modules, Zustand store, shared utils.

---

## Phase 1 — Backend Foundations

### Task 1: HTTP response/request helpers

**Files:**
- Create: `internal/handler/httputil/response.go`
- Create: `internal/handler/httputil/request.go`
- Create: `internal/handler/httputil/response_test.go`
- Create: `internal/handler/httputil/request_test.go`

- [ ] **Step 1: Create the httputil package with response helpers**

```go
// internal/handler/httputil/response.go
package httputil

import (
	"encoding/json"
	"net/http"
)

// WriteJSON serializes v as JSON and writes it with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 2: Create request parsing helpers**

```go
// internal/handler/httputil/request.go
package httputil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ParseIDParam extracts an int64 URL parameter by name.
func ParseIDParam(r *http.Request, key string) (int64, error) {
	raw := chi.URLParam(r, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return id, nil
}

// ParseQueryInt extracts an integer query parameter with a default value.
func ParseQueryInt(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return v
}

// ReadJSON decodes the request body (limited to maxBytes) into v.
func ReadJSON(r *http.Request, v interface{}, maxBytes int64) error {
	return json.NewDecoder(io.LimitReader(r.Body, maxBytes)).Decode(v)
}
```

- [ ] **Step 3: Write tests for response helpers**

```go
// internal/handler/httputil/response_test.go
package httputil

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, 201, map[string]string{"status": "created"})

	assert.Equal(t, 201, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "created", body["status"])
}

func TestWriteJSONNil(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, 200, nil)
	assert.Equal(t, 200, w.Code)
}
```

- [ ] **Step 4: Write tests for request helpers**

```go
// internal/handler/httputil/request_test.go
package httputil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIDParam(t *testing.T) {
	r := httptest.NewRequest("GET", "/titles/42", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "42")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	id, err := ParseIDParam(r, "id")
	require.NoError(t, err)
	assert.Equal(t, int64(42), id)
}

func TestParseIDParamInvalid(t *testing.T) {
	r := httptest.NewRequest("GET", "/titles/abc", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "abc")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	_, err := ParseIDParam(r, "id")
	assert.Error(t, err)
}

func TestParseQueryInt(t *testing.T) {
	r := httptest.NewRequest("GET", "/titles?limit=25", nil)
	assert.Equal(t, 25, ParseQueryInt(r, "limit", 20))
}

func TestParseQueryIntDefault(t *testing.T) {
	r := httptest.NewRequest("GET", "/titles", nil)
	assert.Equal(t, 20, ParseQueryInt(r, "limit", 20))
}

func TestReadJSON(t *testing.T) {
	body := strings.NewReader(`{"name":"test"}`)
	r := httptest.NewRequest("POST", "/", body)

	var v struct{ Name string }
	require.NoError(t, ReadJSON(r, &v, 4096))
	assert.Equal(t, "test", v.Name)
}
```

- [ ] **Step 5: Run tests**

Run: `make test`
Expected: all tests pass including new httputil tests.

- [ ] **Step 6: Commit**

```
feat(handler): ajoute les helpers httputil pour réponses et parsing de requêtes
```

### Task 2: Centralized error handling

**Files:**
- Create: `internal/handler/httputil/errors.go`
- Create: `internal/handler/httputil/errors_test.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: Create APIError type and HandlerFunc wrapper**

```go
// internal/handler/httputil/errors.go
package httputil

import (
	"log"
	"net/http"
)

// APIError represents an error that should be returned to the client.
type APIError struct {
	Status  int
	Message string
	Err     error // original error for logging, not exposed to client
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

// NewAPIError creates an APIError with status code and client-facing message.
func NewAPIError(status int, message string) *APIError {
	return &APIError{Status: status, Message: message}
}

// WrapError creates an APIError wrapping an original error.
func WrapError(status int, message string, err error) *APIError {
	return &APIError{Status: status, Message: message, Err: err}
}

// BadRequest returns a 400 APIError.
func BadRequest(message string) *APIError {
	return NewAPIError(http.StatusBadRequest, message)
}

// NotFound returns a 404 APIError.
func NotFound(message string) *APIError {
	return NewAPIError(http.StatusNotFound, message)
}

// InternalError returns a 500 APIError wrapping the original error.
func InternalError(message string, err error) *APIError {
	return WrapError(http.StatusInternalServerError, message, err)
}

// HandlerFunc is an HTTP handler that returns an error.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// WrapHandler converts a HandlerFunc to a standard http.HandlerFunc.
// It handles APIError by logging the original error and returning the
// appropriate status code and message to the client.
func WrapHandler(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			if apiErr, ok := err.(*APIError); ok {
				if apiErr.Err != nil {
					log.Printf("%s %s: %v", r.Method, r.URL.Path, apiErr.Err)
				}
				http.Error(w, apiErr.Message, apiErr.Status)
				return
			}
			log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
		}
	}
}
```

- [ ] **Step 2: Write tests for error handling**

```go
// internal/handler/httputil/errors_test.go
package httputil

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWrapHandlerAPIError(t *testing.T) {
	handler := WrapHandler(func(w http.ResponseWriter, r *http.Request) error {
		return BadRequest("invalid input")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid input")
}

func TestWrapHandlerInternalError(t *testing.T) {
	handler := WrapHandler(func(w http.ResponseWriter, r *http.Request) error {
		return InternalError("db failed", errors.New("connection refused"))
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)

	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "db failed")
	assert.NotContains(t, w.Body.String(), "connection refused")
}

func TestWrapHandlerNoError(t *testing.T) {
	handler := WrapHandler(func(w http.ResponseWriter, r *http.Request) error {
		WriteJSON(w, 200, map[string]string{"ok": "true"})
		return nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)

	assert.Equal(t, 200, w.Code)
}

func TestWrapHandlerGenericError(t *testing.T) {
	handler := WrapHandler(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("unexpected")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)

	assert.Equal(t, 500, w.Code)
}
```

- [ ] **Step 3: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 4: Commit**

```
feat(handler): ajoute la gestion d'erreurs centralisée avec APIError et WrapHandler
```

### Task 3: Migrate handlers to use httputil

**Files:**
- Modify: `internal/handler/title.go`
- Modify: `internal/handler/episode.go`
- Modify: `internal/handler/season.go`
- Modify: `internal/handler/auth.go`
- Modify: `internal/handler/webhook.go`
- Modify: `internal/handler/push.go`
- Modify: `internal/handler/anilist_auth.go`
- Modify: `internal/handler/settings.go`
- Modify: `internal/router/router.go`

This task migrates all handlers to use `httputil.HandlerFunc` (returning error) and the `httputil` helpers. Each handler method changes signature from `func(w, r)` to `func(w, r) error` and uses `httputil.WriteJSON`, `httputil.ReadJSON`, `httputil.ParseIDParam`, and returns `httputil.BadRequest()`/`httputil.InternalError()` instead of `http.Error()`.

- [ ] **Step 1: Migrate TitleHandler**

Remove the standalone `writeJSON` function from `title.go`. Change all 4 methods (List, GetByID, Create, Update) to return `error`. Replace `strconv.ParseInt(chi.URLParam(...))` with `httputil.ParseIDParam(r, "id")`. Replace `json.NewDecoder(io.LimitReader(...)).Decode(...)` with `httputil.ReadJSON(r, &body, 65536)`. Replace `http.Error(w, ...)` with returning `httputil.BadRequest(...)`, `httputil.NotFound(...)`, or `httputil.InternalError(...)`. Replace `writeJSON(w, ...)` with `httputil.WriteJSON(w, 200, ...)`.

Example — `GetByID` becomes:

```go
func (h *TitleHandler) GetByID(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return httputil.BadRequest("Invalid ID")
	}

	title, err := h.titles.GetByID(id)
	if err != nil {
		return httputil.NotFound("Not found")
	}

	httputil.WriteJSON(w, http.StatusOK, title)
	return nil
}
```

Apply this pattern to List, Create, Update similarly.

- [ ] **Step 2: Migrate EpisodeHandler**

Change ToggleWatched and BatchMarkWatched to return `error`. Use `httputil.ParseIDParam` for titleID and episodeID. Use `httputil.ReadJSON` for body parsing. Return `httputil.InternalError` on failures.

- [ ] **Step 3: Migrate SeasonHandler, WebhookHandler, PushHandler, AniListAuthHandler, SettingsHandler, AuthHandler**

Same pattern for all remaining handlers. Each handler method returns `error` and uses httputil helpers.

Special cases:
- `AuthHandler.GoogleCallback`: keep Google token verification logic, just replace error returns with `httputil` errors
- `WebhookHandler.HandlePlex`: keep multipart parsing, replace `http.Error` with `httputil` returns
- `PushHandler`: keep nil check on `h.push`, return `httputil.NewAPIError(http.StatusServiceUnavailable, "Push not configured")`
- `SPAHandler`, `Health`, `PublicConfig`: these are simple and don't need the error pattern — leave as-is

- [ ] **Step 4: Update router to use WrapHandler**

In `internal/router/router.go`, wrap all handler methods with `httputil.WrapHandler()`:

```go
r.Get("/titles", httputil.WrapHandler(titles.List))
r.Get("/titles/{id}", httputil.WrapHandler(titles.GetByID))
r.Post("/titles", httputil.WrapHandler(titles.Create))
r.Patch("/titles/{id}", httputil.WrapHandler(titles.Update))
// ... same for all handlers returning error
```

Import `httputil "github.com/nicolasvasse/plextracker/internal/handler/httputil"`.

- [ ] **Step 5: Run tests**

Run: `make test`
Expected: all existing tests pass. Some handler tests may need signature adjustments if they call handler methods directly — update them to call the wrapped version or test the HandlerFunc directly.

- [ ] **Step 6: Commit**

```
refactor(handler): migre tous les handlers vers httputil (erreurs centralisées, helpers)
```

### Task 4: API rate limiter for external services

**Files:**
- Create: `internal/service/ratelimiter.go`
- Modify: `internal/service/background.go`
- Modify: `go.mod`

- [ ] **Step 1: Add `golang.org/x/time` dependency**

Run inside container: `go get golang.org/x/time/rate`

- [ ] **Step 2: Create rate limiter wrapper**

```go
// internal/service/ratelimiter.go
package service

import (
	"context"

	"golang.org/x/time/rate"
)

// APILimiter wraps rate.Limiter for external API rate limiting.
type APILimiter struct {
	limiter *rate.Limiter
}

// NewAPILimiter creates a limiter that allows r requests per second with burst b.
func NewAPILimiter(rps float64, burst int) *APILimiter {
	return &APILimiter{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
}

// Wait blocks until the rate limiter allows the request.
func (l *APILimiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}
```

- [ ] **Step 3: Replace time.Sleep in BackgroundService**

In `internal/service/background.go`:

Add a `limiter *APILimiter` field to `BackgroundService`. Initialize in constructor: `limiter: NewAPILimiter(2, 1)` (2 requests/second, burst 1).

Replace `time.Sleep(500 * time.Millisecond)` on line 81 with:
```go
_ = s.limiter.Wait(context.Background())
```

Replace `time.Sleep(250 * time.Millisecond)` on line 195 with:
```go
_ = s.limiter.Wait(context.Background())
```

Replace `time.Sleep(300 * time.Millisecond)` on line 265 with:
```go
_ = s.limiter.Wait(context.Background())
```

Update `NewBackgroundService` to accept and store the limiter, or create it internally.

- [ ] **Step 4: Update router.go to pass limiter**

No change needed if limiter is created internally in BackgroundService constructor.

- [ ] **Step 5: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 6: Commit**

```
refactor(service): remplace time.Sleep par un rate limiter x/time/rate dans BackgroundService
```

---

## Phase 2 — Frontend Foundations

### Task 5: Install Zustand and create title store

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/src/store.ts`

- [ ] **Step 1: Install Zustand**

Run inside container: `cd frontend && npm install zustand`

- [ ] **Step 2: Create title store**

```typescript
// frontend/src/store.ts
import { create } from 'zustand';
import { apiFetch } from './api';
import { Title } from './types';

interface TitleState {
  titles: Title[];
  loading: boolean;
  error: string | null;
  filter: {
    status?: string;
    type?: string;
    search?: string;
    match_status?: string;
  };
  setFilter: (filter: Partial<TitleState['filter']>) => void;
  fetchTitles: () => Promise<void>;
  invalidate: () => Promise<void>;
  getTitleById: (id: number) => Title | undefined;
  updateTitleInCache: (title: Title) => void;
}

export const useTitleStore = create<TitleState>((set, get) => ({
  titles: [],
  loading: false,
  error: null,
  filter: {},

  setFilter: (filter) => {
    set({ filter: { ...get().filter, ...filter } });
    get().fetchTitles();
  },

  fetchTitles: async () => {
    set({ loading: true, error: null });
    try {
      const params = new URLSearchParams();
      const f = get().filter;
      if (f.status) params.set('status', f.status);
      if (f.type) params.set('type', f.type);
      if (f.search) params.set('search', f.search);
      if (f.match_status) params.set('match_status', f.match_status);
      const qs = params.toString();
      const titles = await apiFetch<Title[]>(`/titles${qs ? '?' + qs : ''}`);
      set({ titles, loading: false });
    } catch (e) {
      set({ error: e instanceof Error ? e.message : 'Fetch failed', loading: false });
    }
  },

  invalidate: async () => {
    await get().fetchTitles();
  },

  getTitleById: (id) => get().titles.find((t) => t.id === id),

  updateTitleInCache: (title) => {
    set({
      titles: get().titles.map((t) => (t.id === title.id ? title : t)),
    });
  },
}));
```

- [ ] **Step 3: Run frontend tests**

Run: `make test-front`
Expected: PASS (compilation check)

- [ ] **Step 4: Commit**

```
feat(frontend): ajoute Zustand et le store de titres avec cache et invalidation
```

### Task 6: Create shared frontend utilities

**Files:**
- Create: `frontend/src/utils.ts`

- [ ] **Step 1: Extract shared utility functions**

Search all pages for repeated `getName`, `getTypeLabel`, formatting logic and extract:

```typescript
// frontend/src/utils.ts
import { Title, TitleType, TitleStatus } from './types';

/** Returns the best display name for a title (primary name, or first available). */
export function getName(title: Title): string {
  if (!title.names || title.names.length === 0) return '(untitled)';
  const primary = title.names.find((n) => n.is_primary);
  if (primary) return primary.name;
  const fr = title.names.find((n) => n.language === 'fr');
  if (fr) return fr.name;
  return title.names[0].name;
}

/** Returns the French display label for a title type. */
export function getTypeLabel(type: TitleType): string {
  switch (type) {
    case 'movie': return 'Film';
    case 'series': return 'Série';
    case 'anime': return 'Anime';
    default: return type;
  }
}

/** Returns the French display label for a title status. */
export function getStatusLabel(status: TitleStatus): string {
  switch (status) {
    case 'watching': return 'En cours';
    case 'completed': return 'Terminé';
    case 'dropped': return 'Abandonné';
    case 'plan_to_watch': return 'À voir';
    default: return status;
  }
}

/** Formats a date string to French locale short format. */
export function formatDate(dateStr: string | null | undefined): string {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return '';
  return d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' });
}

/** Returns the total watched episodes across all seasons. */
export function watchedCount(title: Title): number {
  return (title.seasons ?? []).reduce(
    (sum, s) => sum + (s.episodes ?? []).filter((e) => e.watched).length,
    0
  );
}

/** Returns the total episode count across all seasons. */
export function totalEpisodes(title: Title): number {
  return (title.seasons ?? []).reduce(
    (sum, s) => sum + (s.episodes ?? []).length,
    0
  );
}
```

- [ ] **Step 2: Commit**

```
feat(frontend): extrait les utilitaires partagés (getName, getTypeLabel, formatDate)
```

### Task 7: Create ErrorBanner component

**Files:**
- Create: `frontend/src/components/ErrorBanner.tsx`
- Create: `frontend/src/components/ErrorBanner.module.css`

- [ ] **Step 1: Create ErrorBanner component**

```tsx
// frontend/src/components/ErrorBanner.tsx
import { FunctionComponent } from 'preact';
import styles from './ErrorBanner.module.css';

interface Props {
  message: string;
  onDismiss?: () => void;
}

const ErrorBanner: FunctionComponent<Props> = ({ message, onDismiss }) => {
  if (!message) return null;

  return (
    <div class={styles.banner}>
      <span class={styles.message}>{message}</span>
      {onDismiss && (
        <button class={styles.dismiss} onClick={onDismiss}>
          ✕
        </button>
      )}
    </div>
  );
};

export default ErrorBanner;
```

- [ ] **Step 2: Create ErrorBanner styles**

```css
/* frontend/src/components/ErrorBanner.module.css */
.banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  margin: 8px 12px;
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 8px;
  color: #fca5a5;
  font-size: 13px;
}

.message {
  flex: 1;
}

.dismiss {
  background: none;
  border: none;
  color: #fca5a5;
  cursor: pointer;
  padding: 0 0 0 8px;
  font-size: 14px;
}
```

- [ ] **Step 3: Commit**

```
feat(frontend): ajoute le composant ErrorBanner réutilisable
```

### Task 8: Extend theme tokens with spacing/sizing

**Files:**
- Modify: `frontend/src/theme.ts`

- [ ] **Step 1: Add spacing and sizing tokens**

Add to the existing `theme.ts` (after the existing color tokens):

```typescript
// Spacing scale
export const space = {
  xs: '4px',
  sm: '8px',
  md: '12px',
  lg: '16px',
  xl: '24px',
  xxl: '32px',
} as const;

// Border radii
export const radius = {
  sm: '6px',
  md: '10px',
  lg: '16px',
  full: '9999px',
} as const;

// Font sizes
export const fontSize = {
  xs: '11px',
  sm: '13px',
  md: '15px',
  lg: '18px',
  xl: '22px',
} as const;
```

- [ ] **Step 2: Commit**

```
feat(frontend): ajoute les tokens de spacing, radius et font-size au theme
```

### Task 9: Migrate pages to use Zustand store and shared utils

**Files:**
- Modify: `frontend/src/pages/Library.tsx`
- Modify: `frontend/src/pages/Search.tsx`
- Modify: `frontend/src/pages/TitleDetail.tsx`
- Modify: `frontend/src/pages/Validate.tsx`
- Modify: `frontend/src/pages/MatchReview.tsx`
- Modify: `frontend/src/pages/Add.tsx`
- Modify: `frontend/src/components/TitleCard.tsx`
- Modify: `frontend/src/components/MatchReviewCard.tsx`

- [ ] **Step 1: Migrate Library.tsx**

Replace the `useApi('/titles')` call with `useTitleStore()`. Replace inline `getName(t)` with the imported `getName` from `utils.ts`. Use `store.invalidate()` instead of `mutate()` callbacks passed down as props.

- [ ] **Step 2: Migrate TitleDetail.tsx**

Keep `useApi('/titles/{id}')` for the individual title fetch (stays local). After mutations (rating, status change), call `useTitleStore.getState().invalidate()` to refresh the library cache. Import `getName` from `utils.ts`.

- [ ] **Step 3: Migrate Search.tsx, Validate.tsx, MatchReview.tsx, Add.tsx**

Replace local `getName`/`getTypeLabel` with imports from `utils.ts`. After mutations that affect the library (confirming a match, adding a title), call `useTitleStore.getState().invalidate()`.

- [ ] **Step 4: Migrate TitleCard.tsx and MatchReviewCard.tsx**

Import `getName`, `getTypeLabel` from `utils.ts` instead of inline logic.

- [ ] **Step 5: Run frontend tests**

Run: `make test-front`
Expected: PASS

- [ ] **Step 6: Verify in browser**

Navigate to library, search, title detail, match review. Confirm:
- Library loads correctly with filtered titles
- Mutations (mark watched, rate) invalidate and refresh the list
- No console errors

- [ ] **Step 7: Commit**

```
refactor(frontend): migre les pages vers Zustand et les utilitaires partagés
```

### Task 10: Progressive CSS Modules migration

**Files:**
- Create CSS module files for each component migrated
- Modify: all component `.tsx` files

- [ ] **Step 1: Migrate Navbar**

Create `frontend/src/components/Navbar.module.css` extracting inline styles from `Navbar.tsx`. Replace all `style={{ ... }}` with `class={styles.xxx}`. Use theme tokens from `theme.ts` as CSS custom properties or import them.

- [ ] **Step 2: Migrate TitleCard, PosterCard, StatusBadge, FilterBar**

Same pattern: extract inline styles to `.module.css`, replace style props with class names.

- [ ] **Step 3: Migrate SeasonTab, EpisodeRow, ActionBar**

Same pattern.

- [ ] **Step 4: Migrate BottomSheet, RatingPrompt, EditSheet, AniListSheet, MatchReviewCard**

Same pattern.

- [ ] **Step 5: Migrate page components (Library, Search, TitleDetail, etc.)**

Same pattern for page-level styles.

- [ ] **Step 6: Run frontend tests and verify in browser**

Run: `make test-front`
Navigate all pages, verify no regressions.

- [ ] **Step 7: Commit**

```
refactor(frontend): migre tous les composants vers CSS Modules
```

---

## Phase 3 — Domain: Titles

### Task 11: Extract search logic from TitleRepository

**Files:**
- Create: `internal/repository/title_search.go`
- Modify: `internal/repository/title.go` (remove search-related functions)

- [ ] **Step 1: Move search functions to title_search.go**

Move from `title.go` to `title_search.go`:
- `buildFTSQuery` (lines 273-281)
- `levenshtein` (lines 284-310)
- `fuzzySearch` method (lines 313-414)

The `fuzzySearch` method stays as a method on `TitleRepository` since it needs the `db` field.

`title_search.go` contains:
```go
package repository

import (
	"fmt"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// buildFTSQuery transforms a search string into an FTS5 prefix query.
func buildFTSQuery(search string) string {
	// ... (exact current implementation from lines 273-281)
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	// ... (exact current implementation from lines 284-310)
}

// fuzzySearch finds titles by Levenshtein distance when FTS5 returns few results.
func (r *TitleRepository) fuzzySearch(search string, seen map[int64]bool, filter TitleFilter) ([]model.Title, error) {
	// ... (exact current implementation from lines 313-414)
}
```

- [ ] **Step 2: Verify title.go is reduced**

`title.go` should now contain only: TitleFilter, TitleUpdate, Create, GetByID, List, Update, FindByExternalID. The search-related functions are in `title_search.go` but in the same package, so `List()` can still call `buildFTSQuery()` and `fuzzySearch()`.

- [ ] **Step 3: Run tests**

Run: `make test`
Expected: PASS (no behavior change, just file split)

- [ ] **Step 4: Commit**

```
refactor(repository): extrait la logique de recherche dans title_search.go
```

### Task 12: Extract relation loader from TitleRepository

**Files:**
- Create: `internal/repository/title_loader.go`
- Modify: `internal/repository/title.go`

- [ ] **Step 1: Create batch loader functions**

Extract the N+1 loading pattern from `GetByID` and `List` into batch functions:

```go
// internal/repository/title_loader.go
package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// loadNames loads names for multiple titles in a single query.
func loadNames(db *sql.DB, titles []model.Title) error {
	if len(titles) == 0 {
		return nil
	}

	ids := make([]interface{}, len(titles))
	for i, t := range titles {
		ids[i] = t.ID
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	rows, err := db.Query(
		`SELECT id, title_id, name, language, is_primary FROM title_names WHERE title_id IN (`+placeholders+`) ORDER BY title_id, is_primary DESC`,
		ids...,
	)
	if err != nil {
		return fmt.Errorf("load names: %w", err)
	}
	defer rows.Close()

	nameMap := make(map[int64][]model.TitleName)
	for rows.Next() {
		var n model.TitleName
		if err := rows.Scan(&n.ID, &n.TitleID, &n.Name, &n.Language, &n.IsPrimary); err != nil {
			return fmt.Errorf("scan name: %w", err)
		}
		nameMap[n.TitleID] = append(nameMap[n.TitleID], n)
	}

	for i := range titles {
		titles[i].Names = nameMap[titles[i].ID]
	}
	return nil
}

// loadSeasons loads seasons for multiple titles in a single query.
func loadSeasons(db *sql.DB, titles []model.Title) error {
	if len(titles) == 0 {
		return nil
	}

	ids := make([]interface{}, len(titles))
	for i, t := range titles {
		ids[i] = t.ID
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	rows, err := db.Query(
		`SELECT id, title_id, season_number, total_episodes, my_rating FROM seasons WHERE title_id IN (`+placeholders+`) ORDER BY title_id, season_number`,
		ids...,
	)
	if err != nil {
		return fmt.Errorf("load seasons: %w", err)
	}
	defer rows.Close()

	seasonMap := make(map[int64][]model.Season)
	for rows.Next() {
		var s model.Season
		if err := rows.Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes, &s.MyRating); err != nil {
			return fmt.Errorf("scan season: %w", err)
		}
		seasonMap[s.TitleID] = append(seasonMap[s.TitleID], s)
	}

	for i := range titles {
		titles[i].Seasons = seasonMap[titles[i].ID]
	}
	return nil
}

// loadEpisodes loads episodes for all seasons across multiple titles in a single query.
func loadEpisodes(db *sql.DB, titles []model.Title) error {
	var seasonIDs []interface{}
	for _, t := range titles {
		for _, s := range t.Seasons {
			seasonIDs = append(seasonIDs, s.ID)
		}
	}

	if len(seasonIDs) == 0 {
		return nil
	}

	placeholders := strings.Repeat("?,", len(seasonIDs))
	placeholders = placeholders[:len(placeholders)-1]

	rows, err := db.Query(
		`SELECT id, season_id, episode, name, air_date, watched, watched_at, plex_rating_key FROM episodes WHERE season_id IN (`+placeholders+`) ORDER BY season_id, episode`,
		seasonIDs...,
	)
	if err != nil {
		return fmt.Errorf("load episodes: %w", err)
	}
	defer rows.Close()

	epMap := make(map[int64][]model.Episode)
	for rows.Next() {
		var e model.Episode
		if err := rows.Scan(&e.ID, &e.SeasonID, &e.Episode, &e.Name, &e.AirDate, &e.Watched, &e.WatchedAt, &e.PlexRatingKey); err != nil {
			return fmt.Errorf("scan episode: %w", err)
		}
		epMap[e.SeasonID] = append(epMap[e.SeasonID], e)
	}

	for i := range titles {
		for j := range titles[i].Seasons {
			titles[i].Seasons[j].Episodes = epMap[titles[i].Seasons[j].ID]
		}
	}
	return nil
}

// LoadRelations loads names, seasons, and episodes for the given titles.
// This replaces the N+1 loading pattern with batch queries.
func LoadRelations(db *sql.DB, titles []model.Title, includeEpisodes bool) error {
	if err := loadNames(db, titles); err != nil {
		return err
	}
	if err := loadSeasons(db, titles); err != nil {
		return err
	}
	if includeEpisodes {
		return loadEpisodes(db, titles)
	}
	return nil
}
```

- [ ] **Step 2: Refactor GetByID to use batch loader**

Replace the N+1 loading in `GetByID` (lines 79-126) with:

```go
func (r *TitleRepository) GetByID(id int64) (*model.Title, error) {
	title := &model.Title{}
	err := r.db.QueryRow(`SELECT id, type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, created_at, updated_at FROM titles WHERE id = ?`, id).
		Scan(&title.ID, &title.Type, &title.Year, &title.CoverURL, &title.IMDBID, &title.AniListID, &title.TMDBID, &title.TVDBID,
			&title.PlexRatingKey, &title.MyRating, &title.Status, &title.SeriesStatus, &title.MatchStatus, &title.OriginalTitle, &title.MatchSource, &title.CreatedAt, &title.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get title: %w", err)
	}

	titles := []model.Title{*title}
	if err := LoadRelations(r.db, titles, true); err != nil {
		return nil, err
	}
	*title = titles[0]
	return title, nil
}
```

- [ ] **Step 3: Refactor List to use batch loader (lazy: no episodes by default)**

Replace the N+1 loop in `List()` (lines 222-266) with:

```go
// After building the titles slice from query results...
// Load names and seasons only (no episodes for list view)
if err := LoadRelations(r.db, titles, false); err != nil {
	return nil, err
}
```

This is the key performance improvement: `List()` no longer loads every episode for every title.

- [ ] **Step 4: Write tests for batch loader**

```go
// internal/repository/title_loader_test.go
package repository

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRelations(t *testing.T) {
	db := setupTestDB(t) // reuse existing test DB setup pattern

	// Create a title with names, seasons, episodes
	titleRepo := NewTitleRepository(db)
	id, err := titleRepo.Create(&model.Title{
		Type:   model.TitleTypeMovie,
		Year:   2020,
		Status: model.TitleStatusWatching,
	}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	titles := []model.Title{{ID: id}}
	err = LoadRelations(db, titles, true)
	require.NoError(t, err)
	assert.NotEmpty(t, titles[0].Names)
	assert.Equal(t, "Test", titles[0].Names[0].Name)
}
```

- [ ] **Step 5: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 6: Commit**

```
refactor(repository): remplace le chargement N+1 par des requêtes batch dans TitleRepository
```

### Task 13: Add server-side pagination to List

**Files:**
- Modify: `internal/repository/title.go`
- Modify: `internal/handler/title.go`
- Modify: `frontend/src/pages/Library.tsx`
- Modify: `frontend/src/store.ts`

- [ ] **Step 1: Add Offset/Limit to TitleFilter**

In `internal/repository/title.go`, add to TitleFilter:

```go
type TitleFilter struct {
	Status      *model.TitleStatus
	Type        *model.TitleType
	Search      *string
	MatchStatus *model.MatchStatus
	Limit       int
	Offset      int
}
```

In `List()`, append to the query before executing:

```go
if filter.Limit > 0 {
	query += fmt.Sprintf(` LIMIT %d`, filter.Limit)
	if filter.Offset > 0 {
		query += fmt.Sprintf(` OFFSET %d`, filter.Offset)
	}
}
```

- [ ] **Step 2: Update handler to parse pagination params**

In `TitleHandler.List`:

```go
filter.Limit = httputil.ParseQueryInt(r, "limit", 50)
filter.Offset = httputil.ParseQueryInt(r, "offset", 0)
```

- [ ] **Step 3: Update Zustand store for pagination**

Add pagination support to the store: track `offset`, `hasMore`, provide `fetchMore()` method that increments offset and appends results.

- [ ] **Step 4: Update Library.tsx for infinite scroll**

Use `fetchMore()` from the store when the user scrolls near the bottom. Show a loading indicator at the bottom of the list.

- [ ] **Step 5: Run tests and verify in browser**

Run: `make test && make test-front`
Navigate to library, scroll down, verify pagination loads more titles.

- [ ] **Step 6: Commit**

```
feat(api): ajoute la pagination serveur sur GET /api/titles
```

---

## Phase 4 — Domain: Matching & Services

### Task 14: Split TMDB and AniList service files

**Files:**
- Create: `internal/service/matching/tmdb_search.go`
- Create: `internal/service/matching/tmdb_details.go`
- Create: `internal/service/matching/tmdb_covers.go`
- Create: `internal/service/matching/anilist_search.go`
- Create: `internal/service/matching/anilist_sync.go`
- Modify: `internal/service/matching/tmdb.go` (becomes struct + constructor only)
- Modify: `internal/service/matching/anilist.go` (becomes struct + constructor only)

- [ ] **Step 1: Split tmdb.go**

Move methods from `tmdb.go` into three files (same `TMDBClient` struct, same package):

- `tmdb_search.go`: `SearchMovie`, `SearchTV`, response types for search
- `tmdb_details.go`: `GetMovieDetails`, `GetTVDetails`, `GetTVSeasonEpisodes`, `GetTitleNames`, detail response types
- `tmdb_covers.go`: `DownloadCover`
- `tmdb.go`: keeps `TMDBClient` struct, `NewTMDBClient`, `tmdbGet` helper, base types (`TMDBSearchResult`, `DisplayTitle`, `Year`)

- [ ] **Step 2: Split anilist.go**

Move methods from `anilist.go` into two files:

- `anilist_search.go`: `SearchAnime`, `GetAnimeDetails`, `GetNames`, search result types
- `anilist_sync.go`: `SyncRating`
- `anilist.go`: keeps `AniListClient` struct, `NewAniListClient`, `graphqlRequest`, `graphqlDo` helper

- [ ] **Step 3: Run tests**

Run: `make test`
Expected: PASS (no behavior change)

- [ ] **Step 4: Commit**

```
refactor(matching): découpe tmdb.go et anilist.go en fichiers par responsabilité
```

### Task 15: Introduce no-op client pattern

**Files:**
- Create: `internal/service/matching/interfaces.go`
- Modify: `internal/service/push.go`
- Modify: `internal/handler/push.go`
- Modify: handlers that check `if x != nil`

- [ ] **Step 1: Define PushNotifier interface and no-op implementation**

```go
// internal/service/push_notifier.go (or add to push.go)

// PushNotifier sends push notifications.
type PushNotifier interface {
	SendNotification(title, body, url string) error
	Subscribe(subscription string) error
	Unsubscribe() error
}

// noopPushNotifier does nothing when push is not configured.
type noopPushNotifier struct{}

func (n *noopPushNotifier) SendNotification(_, _, _ string) error { return nil }
func (n *noopPushNotifier) Subscribe(_ string) error              { return nil }
func (n *noopPushNotifier) Unsubscribe() error                    { return nil }

// NewPushNotifier returns a real PushService if configured, or a no-op otherwise.
func NewPushNotifier(settings *repository.SettingRepository, publicKey, privateKey, subject string) PushNotifier {
	if publicKey == "" || privateKey == "" {
		return &noopPushNotifier{}
	}
	return NewPushService(settings, publicKey, privateKey, subject)
}
```

- [ ] **Step 2: Update consumers to use PushNotifier interface**

In `EpisodeHandler`, `PlexService`, `PushHandler`, and anywhere that checks `if h.push != nil` — change the field type from `*PushService` to `PushNotifier`. Remove the nil checks since the no-op handles it.

- [ ] **Step 3: Update router.go**

Use `NewPushNotifier()` instead of the conditional `if cfg.VAPIDPublicKey != "" { ... }` block.

- [ ] **Step 4: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```
refactor(service): introduit le pattern no-op client pour PushService
```

### Task 16: Clean up pipeline constants and error handling

**Files:**
- Modify: `internal/service/matching/pipeline.go`

- [ ] **Step 1: Add confidence constants**

In `pipeline.go`, add named constants for the strings currently used in Gemini verification:

```go
const (
	ConfidenceHigh = "high"
	ConfidenceLow  = "low"
)
```

Replace `verification.Confidence == "high"` (line 233) with `verification.Confidence == ConfidenceHigh`.

- [ ] **Step 2: Document step failure behavior**

Add a comment block at the top of `Run()` documenting the explicit design:

```go
// Run executes the full matching pipeline. Each step degrades gracefully:
// a step that fails is logged but does not block subsequent steps.
// The pipeline always returns a result (never nil) with at minimum
// the input title as a fallback name.
```

- [ ] **Step 3: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 4: Commit**

```
refactor(matching): ajoute des constantes nommées et documente la gestion d'erreurs du pipeline
```

---

## Phase 5 — Domain: Episodes & Scrobble

### Task 17: Add transaction helper

**Files:**
- Modify: `internal/database/database.go`

- [ ] **Step 1: Add WithTx helper**

```go
// WithTx executes fn within a database transaction.
// The transaction is committed if fn returns nil, rolled back otherwise.
func WithTx(db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
```

- [ ] **Step 2: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 3: Commit**

```
feat(database): ajoute le helper WithTx pour les transactions
```

### Task 18: Wrap PlexService scrobble in transaction

**Files:**
- Modify: `internal/service/plex.go`
- Modify: `internal/repository/title.go` (add `CreateTx` variant)

- [ ] **Step 1: Add CreateTx to TitleRepository**

Add a `CreateTx` method that accepts `*sql.Tx` instead of using `r.db.Begin()`. The existing `Create` can delegate to `CreateTx` internally.

```go
func (r *TitleRepository) CreateTx(tx *sql.Tx, title *model.Title, names []model.TitleName) (int64, error) {
	res, err := tx.Exec(`INSERT INTO titles ...`, ...)
	// ... same insert logic but using tx instead of self-managed transaction
}

func (r *TitleRepository) Create(title *model.Title, names []model.TitleName) (int64, error) {
	var id int64
	err := database.WithTx(r.db, func(tx *sql.Tx) error {
		var createErr error
		id, createErr = r.CreateTx(tx, title, names)
		return createErr
	})
	return id, err
}
```

- [ ] **Step 2: Wrap processMovie in transaction**

In `processMovie`, wrap the title creation + watch event creation in a single `WithTx` call.

- [ ] **Step 3: Wrap processEpisode similarly**

Series creation + season creation + episode mark + watch event in a transaction.

- [ ] **Step 4: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```
refactor(service): encapsule le scrobble Plex dans une transaction SQLite
```

### Task 19: Optimize batch episode operations

**Files:**
- Modify: `internal/repository/episode.go`
- Modify: `internal/handler/episode.go`

- [ ] **Step 1: Optimize BatchMarkWatched in repository**

The current `BatchMarkWatched` in `episode.go` already uses a single query with `IN (...)`. Verify this is the case. If the handler creates N watch events in a loop (lines 76-82 of `episode.go`), batch those too:

```go
// In EpisodeHandler.BatchMarkWatched, replace the loop with a batch insert:
func (h *EpisodeHandler) batchCreateWatchEvents(titleID int64, episodeIDs []int64) {
	for _, epID := range episodeIDs {
		id := epID
		_, _ = h.events.Create(&model.WatchEvent{
			TitleID:   titleID,
			EpisodeID: &id,
			Source:    model.WatchEventSourceManual,
		})
	}
}
```

If the WatchEventRepository doesn't have a batch create, add one:

```go
func (r *WatchEventRepository) BatchCreate(events []*model.WatchEvent) error {
	// Use a transaction for multiple inserts
	return database.WithTx(r.db, func(tx *sql.Tx) error {
		for _, e := range events {
			_, err := tx.Exec(`INSERT INTO watch_events (title_id, episode_id, source, plex_payload) VALUES (?, ?, ?, ?)`,
				e.TitleID, e.EpisodeID, e.Source, e.PlexPayload)
			if err != nil {
				return fmt.Errorf("insert watch event: %w", err)
			}
		}
		return nil
	})
}
```

- [ ] **Step 2: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 3: Commit**

```
refactor(repository): optimise les opérations batch sur les épisodes et watch events
```

---

## Phase 6 — Quality & Documentation

### Task 20: Add golangci-lint configuration

**Files:**
- Create: `.golangci.yml`

- [ ] **Step 1: Create linter config**

```yaml
# .golangci.yml
run:
  timeout: 5m
  build-tags:
    - sqlite_fts5

linters:
  enable:
    - errcheck
    - gocritic
    - govet
    - ineffassign
    - staticcheck
    - unused

linters-settings:
  errcheck:
    check-type-assertions: true
  gocritic:
    enabled-tags:
      - diagnostic
      - performance

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

- [ ] **Step 2: Run linter and fix issues**

Run: `make lint`
Fix any new warnings introduced by the stricter config.

- [ ] **Step 3: Commit**

```
chore(lint): ajoute .golangci.yml avec errcheck, gocritic et govet
```

### Task 21: Increase backend test coverage

**Files:**
- Modify: `internal/repository/title_test.go`
- Modify: `internal/handler/title_test.go`
- Create: `internal/handler/episode_test.go` (if missing comprehensive tests)

- [ ] **Step 1: Add table-driven tests for TitleRepository**

Add tests for:
- `List` with various filter combinations (status, type, search, match_status)
- `List` with pagination (offset/limit)
- `Update` with partial updates
- `FindByExternalID` with different ID combinations

Use table-driven test pattern:

```go
func TestTitleRepository_List(t *testing.T) {
	tests := []struct {
		name    string
		filter  TitleFilter
		wantLen int
	}{
		{"no filter", TitleFilter{}, 3},
		{"filter by status", TitleFilter{Status: ptr(model.TitleStatusWatching)}, 2},
		// ...
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			titles, err := repo.List(tt.filter)
			require.NoError(t, err)
			assert.Len(t, titles, tt.wantLen)
		})
	}
}
```

- [ ] **Step 2: Add handler tests for error cases**

Test that handlers return correct error codes for invalid inputs, missing resources, etc.

- [ ] **Step 3: Run tests with coverage**

Run: `make shell` then `go test -tags sqlite_fts5 -coverprofile=coverage.out ./... && go tool cover -func=coverage.out`

Report coverage improvements.

- [ ] **Step 4: Commit**

```
test: augmente la couverture des tests backend (repositories et handlers)
```

### Task 22: Add frontend component tests

**Files:**
- Modify: `frontend/package.json` (ensure @testing-library/preact is installed)
- Create: `frontend/src/components/__tests__/ErrorBanner.test.tsx`
- Create: `frontend/src/utils.test.ts`

- [ ] **Step 1: Install testing dependencies if needed**

Run: `cd frontend && npm install -D @testing-library/preact`

- [ ] **Step 2: Test utils.ts**

```typescript
// frontend/src/utils.test.ts
import { describe, it, expect } from 'vitest';
import { getName, getTypeLabel, getStatusLabel, formatDate, watchedCount, totalEpisodes } from './utils';
import { Title } from './types';

describe('getName', () => {
  it('returns primary name', () => {
    const title = { names: [{ name: 'Test', language: 'en', is_primary: true }] } as Title;
    expect(getName(title)).toBe('Test');
  });

  it('falls back to French name', () => {
    const title = { names: [{ name: 'Test FR', language: 'fr', is_primary: false }] } as Title;
    expect(getName(title)).toBe('Test FR');
  });

  it('returns (untitled) when no names', () => {
    const title = { names: [] } as unknown as Title;
    expect(getName(title)).toBe('(untitled)');
  });
});

describe('getTypeLabel', () => {
  it('returns Film for movie', () => expect(getTypeLabel('movie')).toBe('Film'));
  it('returns Série for series', () => expect(getTypeLabel('series')).toBe('Série'));
  it('returns Anime for anime', () => expect(getTypeLabel('anime')).toBe('Anime'));
});

describe('formatDate', () => {
  it('formats a valid date', () => {
    const result = formatDate('2024-03-15');
    expect(result).toContain('15');
    expect(result).toContain('2024');
  });

  it('returns empty for null', () => expect(formatDate(null)).toBe(''));
});
```

- [ ] **Step 3: Test ErrorBanner component**

```tsx
// frontend/src/components/__tests__/ErrorBanner.test.tsx
import { render, fireEvent } from '@testing-library/preact';
import { describe, it, expect, vi } from 'vitest';
import ErrorBanner from '../ErrorBanner';

describe('ErrorBanner', () => {
  it('renders the error message', () => {
    const { getByText } = render(<ErrorBanner message="Something went wrong" />);
    expect(getByText('Something went wrong')).toBeTruthy();
  });

  it('renders nothing when message is empty', () => {
    const { container } = render(<ErrorBanner message="" />);
    expect(container.innerHTML).toBe('');
  });

  it('calls onDismiss when dismiss button clicked', () => {
    const onDismiss = vi.fn();
    const { getByText } = render(<ErrorBanner message="Error" onDismiss={onDismiss} />);
    fireEvent.click(getByText('✕'));
    expect(onDismiss).toHaveBeenCalledOnce();
  });
});
```

- [ ] **Step 4: Run frontend tests**

Run: `make test-front`
Expected: PASS

- [ ] **Step 5: Commit**

```
test(frontend): ajoute les tests unitaires pour utils.ts et ErrorBanner
```

### Task 23: Create static OpenAPI spec

**Files:**
- Create: `docs/openapi.yml`

- [ ] **Step 1: Write OpenAPI 3.0 specification**

Document all ~18 API routes with request/response schemas. Use the route table from `docs/patterns.md` as the source of truth. Include:
- Path parameters (id, titleID, episodeID, seasonID, secret, filename)
- Query parameters (status, type, search, match_status, limit, offset)
- Request bodies (title create/update, episode batch, auth, push subscription, anilist token)
- Response schemas (Title, Season, Episode, error responses)
- Authentication (JWT cookie)
- Status codes per endpoint

- [ ] **Step 2: Commit**

```
docs(api): ajoute la spécification OpenAPI 3.0 statique
```

### Task 24: Update patterns.md and docs

**Files:**
- Modify: `docs/patterns.md`

- [ ] **Step 1: Update patterns.md**

Add new packages and patterns introduced during the refactor:
- `internal/handler/httputil/` — response/request helpers, error handling
- `internal/repository/title_search.go` — search logic
- `internal/repository/title_loader.go` — batch relation loading
- `internal/service/ratelimiter.go` — API rate limiter
- `frontend/src/store.ts` — Zustand title store
- `frontend/src/utils.ts` — shared utilities
- `frontend/src/components/ErrorBanner.tsx` — error display
- CSS Modules pattern
- No-op client pattern for optional services
- Server-side pagination on `GET /api/titles`
- `database.WithTx` helper for transactions

- [ ] **Step 2: Commit**

```
docs: met à jour patterns.md avec les nouveaux patterns du refactor
```

---

## Cross-cutting: Verify after each phase

After completing each phase (Tasks 1-4, 5-10, 11-13, 14-16, 17-19, 20-24):

1. Run `make test && make test-front && make lint`
2. Start the app with `make up && make dev-frontend`
3. Navigate the key screens in-browser:
   - Login
   - Library (scroll, filter)
   - Title detail (seasons, episodes, rate)
   - Search
   - Match review
4. Check browser console for errors
5. Commit phase completion if all green
