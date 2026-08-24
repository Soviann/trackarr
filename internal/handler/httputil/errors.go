package httputil

import (
	"errors"
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
func WrapHandler(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			var apiErr *APIError
			if errors.As(err, &apiErr) {
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
