-- +goose Up
CREATE TABLE subnets (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    -- Expanded form.
    -- Unique among all subnets with may_overlap=false
    prefix      TEXT NOT NULL,
    peer_id     INTEGER,
    comment     TEXT NOT NULL,
    may_overlap BOOLEAN NOT NULL,

    FOREIGN KEY(peer_id) REFERENCES peers(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE
);
CREATE INDEX idx_subnets_peer_id ON subnets(peer_id);

-- +goose Down
DROP TABLE subnets;
DROP INDEX idx_subnets_peer_id;
