-- +goose Up
CREATE TABLE peer_stats (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp_ms      INTEGER NOT NULL,
    peer_id           INTEGER NOT NULL,
    rx                INTEGER NOT NULL,
    tx                INTEGER NOT NULL,
    FOREIGN KEY (peer_id) REFERENCES peers(id)
);
CREATE INDEX peer_stats_timestamp_ms_idx ON peer_stats(timestamp_ms);

-- +goose Down
DROP TABLE peer_stats;
DROP INDEX peer_stats_timestamp_ms_idx;
