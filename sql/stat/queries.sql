-- name: PutStat :exec
INSERT INTO peer_stats (
    peer_id,
    tx_bytes,
    rx_bytes,
    timestamp    
) VALUES (
    @peer_id,
    @tx_bytes,
    @rx_bytes,
    @timestamp
);

-- name: GetLastPeerStat :one
SELECT * FROM peer_stats
WHERE peer_id = @peer_id
ORDER BY timestamp DESC
LIMIT 1;

-- name: GetPeerStatOlderThan :one
SELECT * FROM peer_stats
WHERE
    peer_id = @peer_id AND
    timestamp < @timestamp
ORDER BY timestamp DESC
LIMIT 1;
