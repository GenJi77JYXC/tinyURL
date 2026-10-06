-- Day 1: initial schema for the minimal short-link service.
-- id is a BIGSERIAL (sequence-backed); the service takes nextval() first,
-- base62-encodes the id into a fixed 6-char code, then inserts the row.
-- short_code is fixed CHAR(6) with a unique index for O(1) redirect lookups.

CREATE TABLE IF NOT EXISTS links (
    id           BIGSERIAL    PRIMARY KEY,
    short_code   CHAR(6)      NOT NULL,
    original_url TEXT         NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_links_short_code ON links (short_code);
