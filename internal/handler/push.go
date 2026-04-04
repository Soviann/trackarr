package handler

import (
	"io"
	"log"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/service"
)

type PushHandler struct {
	push *service.PushService
}

func NewPushHandler(push *service.PushService) *PushHandler {
	return &PushHandler{push: push}
}

func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	if h.push == nil {
		http.Error(w, "Push notifications not configured", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.push.Subscribe(string(body)); err != nil {
		log.Printf("push: subscribe: %v", err)
		http.Error(w, "Invalid subscription", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	if h.push == nil {
		http.Error(w, "Push notifications not configured", http.StatusServiceUnavailable)
		return
	}

	if err := h.push.Unsubscribe(); err != nil {
		log.Printf("push: unsubscribe: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
