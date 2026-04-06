-- Add last_watched_at column to titles
ALTER TABLE titles ADD COLUMN last_watched_at DATETIME;

-- Create an index for sorting
CREATE INDEX idx_titles_last_watched_at ON titles(last_watched_at);

-- Update existing last_watched_at from watch_events
UPDATE titles SET last_watched_at = (
    SELECT MAX(created_at) FROM watch_events WHERE title_id = titles.id
);

-- Trigger to keep last_watched_at updated
CREATE TRIGGER IF NOT EXISTS trg_update_title_last_watched
AFTER INSERT ON watch_events
BEGIN
    UPDATE titles SET last_watched_at = NEW.created_at
    WHERE id = NEW.title_id AND (last_watched_at IS NULL OR NEW.created_at > last_watched_at);
END;
