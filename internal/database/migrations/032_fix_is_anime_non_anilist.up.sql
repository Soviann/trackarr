-- Reset is_anime to 0 for titles incorrectly flagged as anime due to TVDB's Animation genre
UPDATE titles SET is_anime = 0 WHERE is_anime = 1 AND (anilist_id IS NULL OR anilist_id = 0);
