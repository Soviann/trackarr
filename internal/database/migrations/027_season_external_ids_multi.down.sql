-- 027_season_external_ids_multi.down.sql
-- WARNING (dev-only rollback): collapsing back to PK (season_id, provider)
-- keeps only the primary part per season (lowest sort_order, then external_id)
-- and DROPS every additional split-cour part plus all per-part meta columns.
PRAGMA foreign_keys=OFF;

CREATE TABLE season_external_ids_old (
    season_id   INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    provider    TEXT    NOT NULL CHECK(provider IN ('anilist')),
    external_id TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (season_id, provider)
);

INSERT INTO season_external_ids_old (season_id, provider, external_id, created_at, updated_at)
SELECT season_id, provider, external_id, created_at, updated_at
FROM season_external_ids
WHERE rowid IN (
    SELECT rowid FROM season_external_ids sei2
    WHERE NOT EXISTS (
        SELECT 1 FROM season_external_ids sei3
        WHERE sei3.season_id = sei2.season_id AND sei3.provider = sei2.provider
          AND ( (sei3.sort_order IS NOT NULL AND (sei2.sort_order IS NULL OR sei3.sort_order < sei2.sort_order))
             OR (sei3.sort_order IS sei2.sort_order AND sei3.external_id < sei2.external_id) )
    )
);

DROP TABLE season_external_ids;
ALTER TABLE season_external_ids_old RENAME TO season_external_ids;

PRAGMA foreign_keys=ON;
