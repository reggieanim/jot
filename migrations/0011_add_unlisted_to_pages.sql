ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS unlisted BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_pages_published_unlisted
    ON pages (published, unlisted)
    WHERE deleted_at IS NULL;
