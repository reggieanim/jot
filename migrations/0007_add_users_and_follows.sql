-- Users table
CREATE TABLE IF NOT EXISTS users (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    username   TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    bio        TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_username ON users (username);

-- Follows table (many-to-many self-referential)
CREATE TABLE IF NOT EXISTS follows (
    follower_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, followee_id),
    CHECK (follower_id <> followee_id)
);

CREATE INDEX idx_follows_followee ON follows (followee_id);

-- Add owner_id to pages so pages belong to a user
ALTER TABLE pages ADD COLUMN IF NOT EXISTS owner_id TEXT REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_pages_owner ON pages (owner_id);
