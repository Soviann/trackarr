-- 028_drop_seasons_anilist_score.down.sql
-- Re-add the season-level AniList score column (as originally created in 023).
-- Data is not restored — the per-part rows in season_external_ids remain the
-- source of truth.
ALTER TABLE seasons ADD COLUMN anilist_average_score INTEGER NULL;
