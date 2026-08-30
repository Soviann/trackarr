-- 043_personal_notes.up.sql
-- Add personal_notes column to titles table to allow users to store private memos/notes.
ALTER TABLE titles ADD COLUMN personal_notes TEXT;
