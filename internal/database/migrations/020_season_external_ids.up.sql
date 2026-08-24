-- Mapping season → identifiant externe (AniList, extensible à d'autres providers).
CREATE TABLE season_external_ids (
    season_id   INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    provider    TEXT    NOT NULL CHECK(provider IN ('anilist')),
    external_id TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (season_id, provider)
);

CREATE INDEX idx_season_external_ids_provider_external
    ON season_external_ids(provider, external_id);

-- Backfill : pour les titres anime ayant déjà un anilist_id, on amorce la saison 1.
INSERT INTO season_external_ids (season_id, provider, external_id)
SELECT s.id, 'anilist', CAST(t.anilist_id AS TEXT)
FROM titles t
JOIN seasons s ON s.title_id = t.id AND s.season_number = 1
WHERE t.is_anime = 1 AND t.anilist_id IS NOT NULL;
