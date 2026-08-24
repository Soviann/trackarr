DROP TRIGGER IF EXISTS trg_update_title_last_watched;
DROP INDEX IF EXISTS idx_titles_last_watched_at;
-- SQLite does not support ALTER TABLE DROP COLUMN before 3.35.0
-- We can't easily drop it in a simple script without creating a temporary table.
-- But we can just leave the column.
