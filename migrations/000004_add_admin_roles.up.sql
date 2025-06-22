-- Create table for storing admin roles per guild
CREATE TABLE IF NOT EXISTS admin_roles (
    guild_id TEXT NOT NULL,
    role_id TEXT NOT NULL,
    added_by TEXT NOT NULL,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guild_id, role_id)
);

-- Add index for quick lookups
CREATE INDEX idx_admin_roles_guild ON admin_roles(guild_id);

-- Add RLS policies
ALTER TABLE admin_roles ENABLE ROW LEVEL SECURITY;

-- Only the discord_bot role can access admin roles
CREATE POLICY discord_bot_admin_roles_all ON admin_roles
    FOR ALL
    TO discord_bot
    USING (true)
    WITH CHECK (true);