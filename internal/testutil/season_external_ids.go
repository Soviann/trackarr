package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/stretchr/testify/require"
)

// InsertSeasonExternalID adds a (seasonID, provider, externalID) part mapping.
func InsertSeasonExternalID(t *testing.T, db *sql.DB, seasonID int64, provider, externalID string) {
	t.Helper()
	err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		return repository.NewSeasonExternalIDWriter(tx).Add(context.Background(), seasonID, provider, externalID)
	})
	require.NoError(t, err)
}

// SetPartEpisodeCount sets anilist_episode_count for a specific part so the
// per-part AniList push can derive a COMPLETED status from a full count.
func SetPartEpisodeCount(t *testing.T, db *sql.DB, seasonID int64, provider, externalID string, count int) {
	t.Helper()
	err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		return repository.NewSeasonExternalIDWriter(tx).
			UpdatePartMeta(context.Background(), seasonID, provider, externalID, nil, &count, nil)
	})
	require.NoError(t, err)
}

// GetSeasonExternalID reads the external_id for (seasonID, provider), or ""
// if no mapping exists.
func GetSeasonExternalID(t *testing.T, db *sql.DB, seasonID int64, provider string) (string, error) {
	t.Helper()
	return repository.NewSeasonExternalIDRepository(db).Get(context.Background(), seasonID, provider)
}
