-- PicoShare accepts a single API key at a time, so the table holds at most one
-- row. key_hash is the SHA-256 digest of the key, never the key itself.
CREATE TABLE api_keys (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    key_hash BLOB NOT NULL CHECK (length(key_hash) = 32),
    created TEXT NOT NULL
) STRICT;
