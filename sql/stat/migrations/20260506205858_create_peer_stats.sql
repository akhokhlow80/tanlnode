-- +goose Up
CREATE TABLE peer_stats (
    peer_id INT NOT NULL,
    tx_bytes INT NOT NULL,
    rx_bytes INT NOT NULL,
    timestamp INT NOT NULL PRIMARY KEY
);
CREATE INDEX peer_stats_peer_id_idx ON peer_stats(peer_id);

-- +goose Down
DROP TABLE peer_stats;
DROP INDEX peer_stats_peer_id_idx;
