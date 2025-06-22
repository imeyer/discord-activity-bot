-- Disable RLS
ALTER TABLE discord_messages DISABLE ROW LEVEL SECURITY;

-- Drop policies
DROP POLICY IF EXISTS server_isolation ON discord_messages;
DROP POLICY IF EXISTS bot_bypass ON discord_messages;
