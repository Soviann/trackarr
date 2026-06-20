package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/require"
)

// InsertSeasonExternalID adds a (seasonID, provider, externalID) part mapping.
// Thin wrapper around SeasonExternalIDRepository.Add for test readability.
func InsertSeasonExternalID(t *testing.T, db *sql.DB, seasonID int64, provider, externalID string) {
	t.Helper()
	require.NoError(t, repository.NewSeasonExternalIDRepository(db).Add(context.Background(), seasonID, provider, externalID))
}

// SetPartEpisodeCount sets anilist_episode_count for a specific part so the
// per-part AniList push can derive a COMPLETED status from a full count.
func SetPartEpisodeCount(t *testing.T, db *sql.DB, seasonID int64, provider, externalID string, count int) {
	t.Helper()
	require.NoError(t, repository.NewSeasonExternalIDRepository(db).
		UpdatePartMeta(context.Background(), seasonID, provider, externalID, nil, &count, nil))
}

// GetSeasonExternalID reads the external_id for (seasonID, provider), or ""
// if no mapping exists.
func GetSeasonExternalID(t *testing.T, db *sql.DB, seasonID int64, provider string) (string, error) {
	t.Helper()
	return repository.NewSeasonExternalIDRepository(db).Get(context.Background(), seasonID, provider)
}
