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

-- -- name: AddPeer :one
-- INSERT INTO peers (
--     public_key_base64,
--     is_enabled,
--     preshared_key_base64,
--     endpoint,
--     persistent_keepalive,
--     allowed_ips,
--     owner
-- ) VALUES (
--     @public_key_base64,
--     @is_enabled,
--     @preshared_key_base64,
--     @endpoint,
--     @persistent_keepalive,
--     @allowed_ips,
--     @owner
-- )
-- RETURNING *;

-- -- name: RemovePeer :execrows
-- DELETE FROM peers
-- WHERE public_key_base64 = @public_key_base64;

-- -- name: GetPeers :many
-- SELECT * FROM peers
-- WHERE owner = COALESCE(@owner, owner)
-- ORDER BY public_key_base64;

-- -- name: GetPeerByPublicKey :one
-- SELECT * FROM peers
-- WHERE public_key_base64 = @public_key_base64
-- LIMIT 1;

-- -- name: UpdatePeer :one
-- UPDATE peers
-- SET
--     is_enabled = @is_enabled,
--     preshared_key_base64 = @preshared_key_base64,
--     endpoint = @endpoint,
--     persistent_keepalive = @persistent_keepalive,
--     allowed_ips = @allowed_ips
-- WHERE public_key_base64 = @public_key_base64
-- RETURNING *;
