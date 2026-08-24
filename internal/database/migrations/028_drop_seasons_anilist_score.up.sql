-- 028_drop_seasons_anilist_score.up.sql
-- Drop the now-unused seasons.anilist_average_score column. The per-part
-- score lives on season_external_ids.anilist_average_score since migration 027;
-- nothing reads or writes the seasons-level column anymore.
ALTER TABLE seasons DROP COLUMN anilist_average_score;
