package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	ts := httptest.NewServer(handler)
	client := NewClient(ts.URL, "trck_live_testapikey1234567890abcdef1234567890abcdef", 5*time.Second)
	return ts, client
}

func TestClient_AuthorizationHeader(t *testing.T) {
	var authHeader string
	var userAgent string

	ts, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		userAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"titles":[],"total":0}`))
	})
	defer ts.Close()

	_, err := client.SearchTitles(context.Background(), "test", "", "", 10)
	require.NoError(t, err)
	assert.Equal(t, "Bearer trck_live_testapikey1234567890abcdef1234567890abcdef", authHeader)
	assert.Contains(t, userAgent, "trackarr-mcp/")
}

func TestClient_ErrorParsing(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		errSubstr  string
	}{
		{
			name:       "401 unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":"Unauthorized","message":"Invalid token"}`,
			errSubstr:  "HTTP 401: Invalid token",
		},
		{
			name:       "403 forbidden insufficient scope",
			statusCode: http.StatusForbidden,
			body:       `{"error":"Forbidden","message":"API key lacks required scope 'library:delete'","code":"insufficient_scope"}`,
			errSubstr:  "API key lacks required scope 'library:delete'",
		},
		{
			name:       "404 not found plain text",
			statusCode: http.StatusNotFound,
			body:       "title not found",
			errSubstr:  "HTTP 404: title not found",
		},
		{
			name:       "500 internal server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":"database disk image is malformed"}`,
			errSubstr:  "HTTP 500: database disk image is malformed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			})
			defer ts.Close()

			err := client.DeleteTitle(context.Background(), 999)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}

func TestTools_NominalExecution(t *testing.T) {
	mux := http.NewServeMux()

	// GET /api/titles
	mux.HandleFunc("/api/titles", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Inception", r.URL.Query().Get("search"))
		assert.Equal(t, "movie", r.URL.Query().Get("type"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total": 1,
			"titles": []map[string]any{
				{
					"id":     42,
					"type":   "movie",
					"year":   2010,
					"status": "completed",
					"names": []map[string]any{
						{"name": "Inception", "language": "en", "is_primary": true},
					},
				},
			},
		})
	})

	// GET /api/titles/42
	mux.HandleFunc("/api/titles/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		overview := "A thief who steals corporate secrets..."
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       42,
			"type":     "movie",
			"year":     2010,
			"status":   "completed",
			"overview": overview,
			"names": []map[string]any{
				{"name": "Inception", "language": "en", "is_primary": true},
			},
		})
	})

	// GET /api/titles/continue-watching
	mux.HandleFunc("/api/titles/continue-watching", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":               10,
				"name":             "Breaking Bad",
				"type":             "series",
				"watched_episodes": 15,
				"total_episodes":   62,
			},
		})
	})

	// GET /api/titles/resolve
	mux.HandleFunc("/api/titles/resolve", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"title_name": "Fight Club",
			"year":       1999,
		})
	})

	// GET /api/stats
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_movies": 150,
			"total_series": 45,
		})
	})

	// POST /api/titles/batch
	mux.HandleFunc("/api/titles/batch", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Items []BatchCreateItem `json:"items"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		titleID := int64(101)
		_ = json.NewEncoder(w).Encode(BatchCreateResult{
			Total:   len(req.Items),
			Created: len(req.Items),
			Items: []BatchCreateResultItem{
				{
					Index:   0,
					TitleID: &titleID,
					Title:   req.Items[0].Title,
					Status:  "created",
				},
			},
		})
	})

	// PATCH /api/titles/42
	mux.HandleFunc("/api/titles/42/episodes/batch-watch", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	// POST /api/arr/push/42
	mux.HandleFunc("/api/arr/push/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"arr_id": 555,
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := NewClient(ts.URL, "trck_live_testkey", 5*time.Second)
	s := server.NewMCPServer("trackarr-test", "1.0.0")
	registerTools(s, client)
	registerResources(s, client)

	ctx := context.Background()

	// 1. trackarr_search
	t.Run("trackarr_search", func(t *testing.T) {
		st := s.GetTool("trackarr_search")
		require.NotNil(t, st)
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "trackarr_search",
				Arguments: map[string]any{
					"query": "Inception",
					"type":  "movie",
				},
			},
		}
		res, err := st.Handler(ctx, req)
		require.NoError(t, err)
		assert.False(t, res.IsError)
		textContent := res.Content[0].(mcp.TextContent).Text
		assert.Contains(t, textContent, "Inception")
	})

	// 2. trackarr_get_title
	t.Run("trackarr_get_title", func(t *testing.T) {
		gt := s.GetTool("trackarr_get_title")
		require.NotNil(t, gt)
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "trackarr_get_title",
				Arguments: map[string]any{
					"id": 42,
				},
			},
		}
		res, err := gt.Handler(ctx, req)
		require.NoError(t, err)
		assert.False(t, res.IsError)
		textContent := res.Content[0].(mcp.TextContent).Text
		assert.Contains(t, textContent, "Inception")
		assert.Contains(t, textContent, "A thief who steals corporate secrets")
	})

	// 3. trackarr_get_continue_watching
	t.Run("trackarr_get_continue_watching", func(t *testing.T) {
		cwt := s.GetTool("trackarr_get_continue_watching")
		require.NotNil(t, cwt)
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "trackarr_get_continue_watching",
			},
		}
		res, err := cwt.Handler(ctx, req)
		require.NoError(t, err)
		assert.False(t, res.IsError)
		textContent := res.Content[0].(mcp.TextContent).Text
		assert.Contains(t, textContent, "Breaking Bad")
	})

	// 4. trackarr_get_stats
	t.Run("trackarr_get_stats", func(t *testing.T) {
		stt := s.GetTool("trackarr_get_stats")
		require.NotNil(t, stt)
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "trackarr_get_stats",
			},
		}
		res, err := stt.Handler(ctx, req)
		require.NoError(t, err)
		assert.False(t, res.IsError)
		textContent := res.Content[0].(mcp.TextContent).Text
		assert.Contains(t, textContent, "total_movies")
	})

	// 5. trackarr_add_title
	t.Run("trackarr_add_title", func(t *testing.T) {
		at := s.GetTool("trackarr_add_title")
		require.NotNil(t, at)
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "trackarr_add_title",
				Arguments: map[string]any{
					"title": "Interstellar",
					"type":  "movie",
					"year":  2014,
				},
			},
		}
		res, err := at.Handler(ctx, req)
		require.NoError(t, err)
		assert.False(t, res.IsError)
		textContent := res.Content[0].(mcp.TextContent).Text
		assert.Contains(t, textContent, "created")
		assert.Contains(t, textContent, "101")
	})

	// 6. trackarr_set_episode_watched
	t.Run("trackarr_set_episode_watched", func(t *testing.T) {
		ew := s.GetTool("trackarr_set_episode_watched")
		require.NotNil(t, ew)
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "trackarr_set_episode_watched",
				Arguments: map[string]any{
					"title_id":   42,
					"episode_id": 105,
					"watched":    true,
				},
			},
		}
		res, err := ew.Handler(ctx, req)
		require.NoError(t, err)
		assert.False(t, res.IsError)
		textContent := res.Content[0].(mcp.TextContent).Text
		assert.Contains(t, textContent, "successfully marked as watched")
	})

	// 7. trackarr_push_to_arr
	t.Run("trackarr_push_to_arr", func(t *testing.T) {
		pa := s.GetTool("trackarr_push_to_arr")
		require.NotNil(t, pa)
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "trackarr_push_to_arr",
				Arguments: map[string]any{
					"id": 42,
				},
			},
		}
		res, err := pa.Handler(ctx, req)
		require.NoError(t, err)
		assert.False(t, res.IsError)
		textContent := res.Content[0].(mcp.TextContent).Text
		assert.Contains(t, textContent, "Arr ID: 555")
	})
}

func TestTools_ErrorHandling_IsErrorTrue(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/titles/999", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Not Found","message":"Title 999 does not exist"}`))
	})

	mux.HandleFunc("/api/arr/push/888", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Forbidden","message":"API key lacks required scope 'arr:write'","code":"insufficient_scope"}`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := NewClient(ts.URL, "trck_live_testkey", 5*time.Second)
	s := server.NewMCPServer("trackarr-test", "1.0.0")
	registerTools(s, client)

	ctx := context.Background()

	// Missing ID validation error
	t.Run("validation_error_missing_id", func(t *testing.T) {
		gt := s.GetTool("trackarr_get_title")
		res, err := gt.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "trackarr_get_title",
				Arguments: map[string]any{},
			},
		})
		require.NoError(t, err)
		assert.True(t, res.IsError)
		assert.Contains(t, res.Content[0].(mcp.TextContent).Text, "parameter 'id' is required")
	})

	// Backend 404 error
	t.Run("backend_404_error", func(t *testing.T) {
		gt := s.GetTool("trackarr_get_title")
		res, err := gt.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "trackarr_get_title",
				Arguments: map[string]any{"id": 999},
			},
		})
		require.NoError(t, err)
		assert.True(t, res.IsError)
		assert.Contains(t, res.Content[0].(mcp.TextContent).Text, "Title 999 does not exist")
	})

	// Backend 403 Forbidden insufficient scope error
	t.Run("backend_403_insufficient_scope", func(t *testing.T) {
		pa := s.GetTool("trackarr_push_to_arr")
		res, err := pa.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "trackarr_push_to_arr",
				Arguments: map[string]any{"id": 888},
			},
		})
		require.NoError(t, err)
		assert.True(t, res.IsError)
		assert.Contains(t, res.Content[0].(mcp.TextContent).Text, "API key lacks required scope 'arr:write'")
	})

	// Add title without any identifier
	t.Run("add_title_empty_identifiers", func(t *testing.T) {
		at := s.GetTool("trackarr_add_title")
		res, err := at.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "trackarr_add_title",
				Arguments: map[string]any{},
			},
		})
		require.NoError(t, err)
		assert.True(t, res.IsError)
		assert.Contains(t, res.Content[0].(mcp.TextContent).Text, "at least one identifier")
	})

	// Batch add with empty items
	t.Run("batch_add_empty_items", func(t *testing.T) {
		ba := s.GetTool("trackarr_batch_add")
		res, err := ba.Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "trackarr_batch_add",
				Arguments: map[string]any{"items": []any{}},
			},
		})
		require.NoError(t, err)
		assert.True(t, res.IsError)
		assert.Contains(t, res.Content[0].(mcp.TextContent).Text, "between 1 and 100 items")
	})
}

func TestResources_Read(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_titles": 200,
		})
	})

	mux.HandleFunc("/api/titles/continue-watching", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "Succession"},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := NewClient(ts.URL, "trck_live_testkey", 5*time.Second)
	s := server.NewMCPServer("trackarr-test", "1.0.0")
	registerResources(s, client)

	ctx := context.Background()
	resources := s.ListResources()
	require.Contains(t, resources, "trackarr://library/summary")
	require.Contains(t, resources, "trackarr://continue-watching")

	// Read summary
	summaryRes, err := resources["trackarr://library/summary"].Handler(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "trackarr://library/summary",
		},
	})
	require.NoError(t, err)
	require.Len(t, summaryRes, 1)
	textContents, ok := summaryRes[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Contains(t, textContents.Text, "total_titles")

	// Read continue-watching
	cwRes, err := resources["trackarr://continue-watching"].Handler(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "trackarr://continue-watching",
		},
	})
	require.NoError(t, err)
	require.Len(t, cwRes, 1)
	cwTextContents, ok := cwRes[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Contains(t, cwTextContents.Text, "Succession")
}
