-- Enable RLS on discord_messages table
ALTER TABLE discord_messages ENABLE ROW LEVEL SECURITY;

-- Simple server-level isolation policy
-- Any query can only see messages from the current server context
CREATE POLICY server_isolation ON discord_messages
    FOR ALL
    USING (server_id = current_setting('app.current_server_id', true)::text);

-- Policy for the bot service (bypass RLS)
-- The bot needs to see all data to insert messages from any server
-- Only specific database role can bypass RLS
CREATE POLICY bot_bypass ON discord_messages
    FOR ALL
    USING (current_user = 'discord_bot');

-- Note: TimescaleDB continuous aggregates will respect RLS from the base table
