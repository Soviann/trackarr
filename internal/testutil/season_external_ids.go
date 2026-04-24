package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/require"
)

// InsertSeasonExternalID upserts a (seasonID, provider) → externalID mapping.
// Thin wrapper around SeasonExternalIDRepository.Set for test readability.
func InsertSeasonExternalID(t *testing.T, db *sql.DB, seasonID int64, provider, externalID string) {
	t.Helper()
	require.NoError(t, repository.NewSeasonExternalIDRepository(db).Set(context.Background(), seasonID, provider, externalID))
}

// GetSeasonExternalID reads the external_id for (seasonID, provider), or ""
// if no mapping exists.
func GetSeasonExternalID(t *testing.T, db *sql.DB, seasonID int64, provider string) (string, error) {
	t.Helper()
	return repository.NewSeasonExternalIDRepository(db).Get(context.Background(), seasonID, provider)
}
