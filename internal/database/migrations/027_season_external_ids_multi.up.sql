-- 027_season_external_ids_multi.up.sql
-- Allow multiple AniList entries per season (split-cour seasons).
-- Rebuild: PK (season_id, provider) -> (season_id, provider, external_id);
-- add per-part meta columns; migrate seasons.anilist_average_score into the row.
PRAGMA foreign_keys=OFF;

CREATE TABLE season_external_ids_new (
    season_id             INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    provider              TEXT    NOT NULL CHECK(provider IN ('anilist')),
    external_id           TEXT    NOT NULL,
    anilist_episode_count INTEGER NULL,
    anilist_start_date    TEXT    NULL,
    anilist_average_score INTEGER NULL,
    sort_order            INTEGER NULL,
    created_at            TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (season_id, provider, external_id)
);

INSERT INTO season_external_ids_new
    (season_id, provider, external_id, anilist_average_score, created_at, updated_at)
SELECT sei.season_id, sei.provider, sei.external_id, s.anilist_average_score,
       sei.created_at, sei.updated_at
FROM season_external_ids sei
JOIN seasons s ON s.id = sei.season_id;

DROP TABLE season_external_ids;
ALTER TABLE season_external_ids_new RENAME TO season_external_ids;

PRAGMA foreign_keys=ON;
