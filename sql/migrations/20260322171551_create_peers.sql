-- +goose Up
CREATE TABLE peers (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    public_key_base64      TEXT NOT NULL UNIQUE,
    is_enabled             BOOLEAN NOT NULL,
    preshared_key_base64   TEXT,
    endpoint               TEXT,
    persistent_keepalive   INTEGER,
    owner                  TEXT
);
CREATE UNIQUE INDEX idx_peers_public_key_base64 ON peers(public_key_base64);
CREATE INDEX idx_peers_owner ON peers(owner);

-- +goose Down
DROP TABLE peers;
DROP INDEX idx_peers_public_key_base64;
DROP INDEX idx_peers_owner;
