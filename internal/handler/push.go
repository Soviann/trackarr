package handler

import (
	"io"
	"net/http"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/service"
)

type PushHandler struct {
	push service.PushNotifier
}

func NewPushHandler(push service.PushNotifier) *PushHandler {
	return &PushHandler{push: push}
}

func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err != nil {
		return httputil.BadRequest("Invalid request")
	}

	if err := h.push.Subscribe(r.Context(), string(body)); err != nil {
		return httputil.BadRequest("Invalid subscription")
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) error {
	if err := h.push.Unsubscribe(r.Context()); err != nil {
		return httputil.InternalError("Internal error", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
