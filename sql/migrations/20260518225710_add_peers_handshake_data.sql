-- +goose Up
ALTER TABLE peers ADD COLUMN latest_handshake_at TIMESTAMP;
-- Could be different from the configured one due to roaming.
ALTER TABLE peers ADD COLUMN latest_endpoint TEXT;

-- +goose Down
ALTER TABLE peers DROP COLUMN endpoint;
ALTER TABLE peers DROP COLUMN latest_handshake_at;
