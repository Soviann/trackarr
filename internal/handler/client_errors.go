package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/Soviann/trackarr/internal/handler/httputil"
)

type ClientErrorHandler struct{}

type clientErrorPayload struct {
	Message string `json:"message"`
	Stack   string `json:"stack,omitempty"`
}

func (h *ClientErrorHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	var payload clientErrorPayload
	if err := httputil.ReadJSON(r, &payload, 1<<16); err != nil {
		return httputil.NewAPIError(http.StatusBadRequest, "invalid body")
	}
	if payload.Message == "" {
		return httputil.NewAPIError(http.StatusBadRequest, "message is required")
	}
	slog.WarnContext(r.Context(), "[client-error]",
		"message", sanitizeLogLine(payload.Message),
		"stack", sanitizeLogLine(payload.Stack),
	)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// sanitizeLogLine collapses CR/LF to spaces so a rogue tab or compromised
// service worker cannot inject fake log lines into our slog output.
func sanitizeLogLine(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}
