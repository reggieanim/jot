-- Track which signed-in users have accessed a page via a share link
CREATE TABLE IF NOT EXISTS page_collab_users (
    page_id      TEXT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access       TEXT NOT NULL DEFAULT 'view',
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (page_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_page_collab_users_page ON page_collab_users (page_id);
