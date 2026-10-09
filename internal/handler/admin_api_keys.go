package handler

import (
	"errors"
	"net/http"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/service"
)

type AdminAPIKeyHandler struct {
	apiKeySvc *service.APIKeyService
}

func NewAdminAPIKeyHandler(apiKeySvc *service.APIKeyService) *AdminAPIKeyHandler {
	return &AdminAPIKeyHandler{apiKeySvc: apiKeySvc}
}

// List returns all configured API keys.
func (h *AdminAPIKeyHandler) List(w http.ResponseWriter, r *http.Request) error {
	keys, err := h.apiKeySvc.ListKeys(r.Context())
	if err != nil {
		return httputil.InternalError("list api keys", err)
	}
	if keys == nil {
		keys = []*model.APIKey{}
	}
	httputil.WriteJSON(w, http.StatusOK, keys)
	return nil
}

type CreateAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// Create generates a new labeled API key.
func (h *AdminAPIKeyHandler) Create(w http.ResponseWriter, r *http.Request) error {
	var req CreateAPIKeyRequest
	if err := httputil.ReadJSON(r, &req, 10240); err != nil {
		return httputil.BadRequest("invalid json payload")
	}

	key, rawToken, err := h.apiKeySvc.CreateKey(r.Context(), req.Name, req.Scopes)
	if err != nil {
		if errors.Is(err, service.ErrEmptyKeyName) || errors.Is(err, service.ErrInvalidScope) {
			return httputil.BadRequest(err.Error())
		}
		return httputil.InternalError("create api key", err)
	}

	httputil.WriteJSON(w, http.StatusCreated, model.CreateAPIKeyResponse{
		Key:   key,
		Token: rawToken,
	})
	return nil
}

// Revoke marks the API key as revoked.
func (h *AdminAPIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return httputil.BadRequest("invalid key id")
	}

	if err := h.apiKeySvc.RevokeKey(r.Context(), id); err != nil {
		return httputil.InternalError("revoke api key", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Delete permanently removes an API key.
func (h *AdminAPIKeyHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return httputil.BadRequest("invalid key id")
	}

	if err := h.apiKeySvc.DeleteKey(r.Context(), id); err != nil {
		return httputil.InternalError("delete api key", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
