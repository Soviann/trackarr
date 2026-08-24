package httputil

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON serializes v as JSON and writes it with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httputil.WriteJSON: encode: %v", err)
	}
}
