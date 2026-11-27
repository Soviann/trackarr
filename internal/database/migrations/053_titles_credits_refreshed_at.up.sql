-- Timestamp of the last completed TMDB extended-credits fetch or catch-up.
-- NULL means extended credits have not yet been refreshed/caught up.
ALTER TABLE titles ADD COLUMN credits_refreshed_at DATETIME NULL;
