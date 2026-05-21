--- ======= Subnets ======= 

-- name: AddSubnet :one
INSERT INTO subnets (
    prefix,
    peer_id,
    comment,
    may_overlap
) VALUES (
    @prefix,
    @peer_id,
    @comment,
    @may_overlap
) RETURNING *;

-- name: GetReservedSubnets :many
SELECT * FROM subnets
WHERE peer_id IS NULL;

-- name: GetAllSubnets :many
SELECT * FROM subnets;

-- name: DeleteSubnet :execrows
DELETE FROM subnets WHERE id = @id;

-- name: GetSubnetByID :one
SELECT * FROM subnets
WHERE id = @id;

-- name: GetPeerSubnets :many
SELECT * FROM subnets
WHERE peer_id = @peer_id;

--- ======= Peers ======= 

-- name: AddPeer :one
INSERT INTO peers (
    public_key_base64,
    is_enabled,
    preshared_key_base64,
    endpoint,
    persistent_keepalive,
    owner
) VALUES (
    @public_key_base64,
    @is_enabled,
    @preshared_key_base64,
    @endpoint,
    @persistent_keepalive,
    @owner
)
RETURNING *;

-- name: RemovePeer :execrows
DELETE FROM peers
WHERE public_key_base64 = @public_key_base64;

-- name: GetPeers :many
SELECT * FROM peers
WHERE owner = COALESCE(sqlc.narg('owner'), owner)
ORDER BY public_key_base64;

-- name: GetPeerByPublicKey :one
SELECT * FROM peers
WHERE public_key_base64 = @public_key_base64
LIMIT 1;

-- name: UpdatePeer :one
UPDATE peers
SET
    is_enabled = @is_enabled,
    preshared_key_base64 = @preshared_key_base64,
    endpoint = @endpoint,
    persistent_keepalive = @persistent_keepalive,
    owner = @owner
WHERE public_key_base64 = @public_key_base64
RETURNING *;

-- name: UpdatePeerHandshakeData :execrows
UPDATE peers
SET
    latest_handshake_at = @latest_handshake_at,
    latest_endpoint = @latest_endpoint
WHERE id = @id;

--- ======= Stats ======= 

-- name: PutPeerStat :one
INSERT INTO peer_stats (
    timestamp_ms,
    peer_id,
    rx,
    tx
) VALUES (
    @timestamp_ms,
    @peer_id,
    @rx,
    @tx
) RETURNING *;

-- name: GetLastPeerStat :one
SELECT * FROM peer_stats
WHERE peer_id = @peer_id AND coalesce(timestamp_ms <= sqlc.narg('until_ms'), TRUE)
ORDER BY timestamp_ms DESC
LIMIT 1;

-- name: RemoveOldStats :exec
DELETE FROM peer_stats
WHERE timestamp_ms <= @oldest_ms;
