package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

var (
	apiKeyFormatRegex     = regexp.MustCompile(`^trck_live_[0-9a-f]{64}$`)
	ErrInvalidTokenFormat = errors.New("invalid token format")
	ErrKeyNotFound        = errors.New("api key not found")
	ErrInvalidScope       = errors.New("invalid scope specified")
	ErrEmptyKeyName       = errors.New("key name cannot be empty")
)

type APIKeyService struct {
	writeDB  *sql.DB
	readRepo *repository.APIKeyRepository

	mu       sync.Mutex
	lastUsed map[int64]time.Time
}

func NewAPIKeyService(writeDB *sql.DB, readRepo *repository.APIKeyRepository) *APIKeyService {
	return &APIKeyService{
		writeDB:  writeDB,
		readRepo: readRepo,
		lastUsed: make(map[int64]time.Time),
	}
}

// ValidateTokenFormat checks if the raw token matches ^trck_live_[0-9a-f]{64}$.
func (s *APIKeyService) ValidateTokenFormat(token string) bool {
	return apiKeyFormatRegex.MatchString(token)
}

// HashToken calculates the SHA-256 hex digest of a token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// AuthenticateToken checks the token format, hashes it, queries for an active key,
// and touches the last_used_at timestamp with memory throttling.
func (s *APIKeyService) AuthenticateToken(ctx context.Context, token string) (*model.APIKey, error) {
	if !s.ValidateTokenFormat(token) {
		return nil, ErrInvalidTokenFormat
	}

	hash := HashToken(token)
	key, err := s.readRepo.GetByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("lookup api key: %w", err)
	}
	if key == nil || key.RevokedAt != nil {
		return nil, ErrKeyNotFound
	}

	s.TouchLastUsed(key.ID)
	return key, nil
}

// TouchLastUsed records usage, throttled in-memory to once every 15 minutes per key.
func (s *APIKeyService) TouchLastUsed(keyID int64) {
	s.mu.Lock()
	now := time.Now().UTC()
	last, exists := s.lastUsed[keyID]
	if exists && now.Sub(last) < 15*time.Minute {
		s.mu.Unlock()
		return
	}
	s.lastUsed[keyID] = now
	s.mu.Unlock()

	go func() {
		_ = database.WithTx(s.writeDB, func(tx *sql.Tx) error {
			return repository.NewAPIKeyWriter(tx).UpdateLastUsedAt(context.Background(), keyID, now)
		})
	}()
}

// ListKeys returns all configured API keys.
func (s *APIKeyService) ListKeys(ctx context.Context) ([]*model.APIKey, error) {
	return s.readRepo.List(ctx)
}

// CreateKey validates inputs, generates a high-entropy token, and stores the key record.
func (s *APIKeyService) CreateKey(ctx context.Context, name string, scopes []string) (*model.APIKey, string, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, "", ErrEmptyKeyName
	}
	if len(trimmedName) > 100 {
		return nil, "", errors.New("key name exceeds 100 characters")
	}

	cleanScopes := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		trimmed := strings.TrimSpace(sc)
		if !isValidScope(trimmed) {
			return nil, "", fmt.Errorf("%w: %s", ErrInvalidScope, trimmed)
		}
		cleanScopes = append(cleanScopes, trimmed)
	}
	if len(cleanScopes) == 0 {
		return nil, "", errors.New("at least one valid scope must be selected")
	}

	rawToken, keyHash, keyPrefix, err := generateToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}

	var createdKey *model.APIKey
	err = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		writer := repository.NewAPIKeyWriter(tx)
		k, err := writer.Create(ctx, trimmedName, keyHash, keyPrefix, cleanScopes)
		if err != nil {
			return err
		}
		createdKey = k
		return nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("persist api key: %w", err)
	}

	return createdKey, rawToken, nil
}

// RevokeKey soft-deletes the key.
func (s *APIKeyService) RevokeKey(ctx context.Context, id int64) error {
	return database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewAPIKeyWriter(tx).Revoke(ctx, id)
	})
}

// DeleteKey permanently deletes the key.
func (s *APIKeyService) DeleteKey(ctx context.Context, id int64) error {
	s.mu.Lock()
	delete(s.lastUsed, id)
	s.mu.Unlock()

	return database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewAPIKeyWriter(tx).Delete(ctx, id)
	})
}

func isValidScope(s string) bool {
	for _, valid := range model.AllScopes {
		if s == valid {
			return true
		}
	}
	return false
}

func generateToken() (rawToken string, keyHash string, keyPrefix string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	hexPart := hex.EncodeToString(b)
	rawToken = "trck_live_" + hexPart
	keyPrefix = rawToken[:12]
	keyHash = HashToken(rawToken)
	return rawToken, keyHash, keyPrefix, nil
}
