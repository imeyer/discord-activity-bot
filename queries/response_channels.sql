-- name: SetResponseChannel :exec
-- Set response channel for a command (empty command_name = global)
INSERT INTO response_channel_config (guild_id, command_name, channel_id, configured_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (guild_id, command_name) 
DO UPDATE SET 
    channel_id = EXCLUDED.channel_id,
    configured_by = EXCLUDED.configured_by,
    configured_at = NOW();

-- name: GetResponseChannel :one
-- Get response channel for a specific command
SELECT channel_id, configured_by, configured_at
FROM response_channel_config
WHERE guild_id = $1 AND command_name = $2;

-- name: GetGlobalResponseChannel :one
-- Get global response channel for a guild
SELECT channel_id, configured_by, configured_at
FROM response_channel_config
WHERE guild_id = $1 AND command_name = '';

-- name: ListResponseChannels :many
-- List all response channel configurations for a guild
SELECT command_name, channel_id, configured_by, configured_at
FROM response_channel_config
WHERE guild_id = $1
ORDER BY command_name;

-- name: ClearResponseChannel :exec
-- Clear response channel configuration for a specific command
DELETE FROM response_channel_config
WHERE guild_id = $1 AND command_name = $2;

-- name: ClearAllResponseChannels :exec
-- Clear all response channel configurations for a guild
DELETE FROM response_channel_config
WHERE guild_id = $1;