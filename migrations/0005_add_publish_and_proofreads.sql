ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS published BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS proofreads (
    id TEXT PRIMARY KEY,
    page_id TEXT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    author_name TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    stance TEXT NOT NULL DEFAULT 'review',
    annotations JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_proofreads_page_id ON proofreads(page_id);
CREATE INDEX IF NOT EXISTS idx_proofreads_created_at ON proofreads(created_at DESC);
