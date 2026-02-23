-- Add background color column to pages
ALTER TABLE pages ADD COLUMN IF NOT EXISTS bg_color TEXT NOT NULL DEFAULT '';
