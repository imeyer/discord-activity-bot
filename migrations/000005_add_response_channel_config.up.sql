-- Create table for storing response channel configuration per guild
CREATE TABLE IF NOT EXISTS response_channel_config (
    guild_id TEXT NOT NULL,
    command_name TEXT DEFAULT '', -- Empty string for global config, specific command name for per-command config
    channel_id TEXT NOT NULL,
    configured_by TEXT NOT NULL,
    configured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guild_id, command_name)
);

-- Add index for quick lookups
CREATE INDEX idx_response_channel_config_guild ON response_channel_config(guild_id);

-- Add RLS policies
ALTER TABLE response_channel_config ENABLE ROW LEVEL SECURITY;

-- Only the discord_bot role can access response channel config
CREATE POLICY discord_bot_response_channel_config_all ON response_channel_config
    FOR ALL
    TO discord_bot
    USING (true)
    WITH CHECK (true);