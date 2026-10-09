package model

import "time"

// Scopes for API Key permissions.
const (
	ScopeLibraryRead   = "library:read"
	ScopeLibraryWrite  = "library:write"
	ScopeLibraryDelete = "library:delete"
	ScopeArrRead       = "arr:read"
	ScopeArrWrite      = "arr:write"
)

var AllScopes = []string{
	ScopeLibraryRead,
	ScopeLibraryWrite,
	ScopeLibraryDelete,
	ScopeArrRead,
	ScopeArrWrite,
}

// APIKey represents a machine-to-machine authentication credential.
type APIKey struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	KeyHash    string     `json:"-"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// HasScope checks whether the key has the specified scope.
func (k *APIKey) HasScope(scope string) bool {
	if k == nil {
		return false
	}
	for _, s := range k.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// CreateAPIKeyResponse wraps the created key metadata and the raw one-time secret token.
type CreateAPIKeyResponse struct {
	Key   *APIKey `json:"key"`
	Token string  `json:"token"`
}
