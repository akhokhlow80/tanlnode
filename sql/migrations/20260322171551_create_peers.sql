-- +goose Up
CREATE TABLE peers (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    public_key_base64      TEXT NOT NULL UNIQUE,
    is_enabled             BOOLEAN NOT NULL,
    -- Could be omitted
    preshared_key_base64   TEXT NOT NULL,
    -- Could be omitted
    endpoint               TEXT NOT NULL,
    -- Could be omitted
    persistent_keepalive   INTEGER NOT NULL,
    -- Could be omitted
    owner                  TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_peers_public_key_base64 ON peers(public_key_base64);
CREATE INDEX idx_peers_owner ON peers(owner);

-- +goose Down
DROP TABLE peers;
DROP INDEX idx_peers_public_key_base64;
DROP INDEX idx_peers_owner;
