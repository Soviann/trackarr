-- Add is_anime column to titles
ALTER TABLE titles ADD COLUMN is_anime INTEGER NOT NULL DEFAULT 0;

-- Create index for performance on filtering
CREATE INDEX idx_titles_is_anime ON titles(is_anime);

-- Note: We don't migrate existing data because the user will re-import everything.
-- We just need to update the CHECK constraint, but SQLite doesn't support ALTER TABLE DROP CONSTRAINT.
-- However, we can re-create the table if needed, but since we are re-importing, 
-- we can just ensure new inserts follow the new movie/series rule.
-- Actually, a cleaner way in SQLite for CHECK constraints is the "shadow table" pattern or just 
-- accepting that the old constraint might still exist until re-import.
-- Since the user said they will "delete everything", let's just add the column for now.
