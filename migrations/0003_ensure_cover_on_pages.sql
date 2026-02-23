-- Ensure cover column exists on pages table
ALTER TABLE pages ADD COLUMN IF NOT EXISTS cover TEXT DEFAULT NULL;
