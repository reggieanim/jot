CREATE TABLE IF NOT EXISTS page_reads (
    page_id TEXT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    reader_key TEXT NOT NULL,
    read_count INT NOT NULL DEFAULT 1,
    first_read_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (page_id, reader_key)
);

CREATE INDEX IF NOT EXISTS idx_page_reads_page_id ON page_reads(page_id);
CREATE INDEX IF NOT EXISTS idx_page_reads_last_read_at ON page_reads(last_read_at DESC);
