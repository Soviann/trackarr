CREATE TRIGGER IF NOT EXISTS trg_update_title_last_watched
AFTER INSERT ON watch_events
FOR EACH ROW
BEGIN
    UPDATE titles SET last_watched_at = NEW.created_at
    WHERE id = NEW.title_id AND (last_watched_at IS NULL OR NEW.created_at > last_watched_at);
END;
