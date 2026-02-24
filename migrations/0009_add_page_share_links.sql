CREATE TABLE IF NOT EXISTS page_share_links (
    token TEXT PRIMARY KEY,
    page_id TEXT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    access TEXT NOT NULL CHECK (access IN ('view', 'edit')),
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_page_share_links_page_id ON page_share_links(page_id);
