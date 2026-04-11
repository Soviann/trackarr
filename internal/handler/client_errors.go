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
	if err := httputil.ReadJSON(r, &payload, 1<<16); err != nil {
		return httputil.NewAPIError(http.StatusBadRequest, "invalid body")
	}
	if payload.Message == "" {
		return httputil.NewAPIError(http.StatusBadRequest, "message is required")
	}
	slog.WarnContext(r.Context(), "[client-error]",
		"message", payload.Message,
		"stack", payload.Stack,
	)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
