package repository

import (
	"context"
	"database/sql"
	"fmt"
)

// SeasonAuditWriter records dismissed season-attachment proposals.
type SeasonAuditWriter struct{ tx *sql.Tx }

// NewSeasonAuditWriter creates a new SeasonAuditWriter.
func NewSeasonAuditWriter(tx *sql.Tx) *SeasonAuditWriter { return &SeasonAuditWriter{tx: tx} }

// Dismiss records that the (source, target) attachment proposal should never be
// surfaced again. Idempotent via INSERT OR IGNORE on the composite primary key.
func (w *SeasonAuditWriter) Dismiss(ctx context.Context, sourceID, targetID int64) error {
	if _, err := w.tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO season_audit_dismissals (source_title_id, target_title_id) VALUES (?, ?)`,
		sourceID, targetID); err != nil {
		return fmt.Errorf("season_audit: dismiss: %w", err)
	}
	return nil
}
