-- Drop the constraint
ALTER TABLE discord_messages 
DROP CONSTRAINT IF EXISTS discord_messages_unique_message;

-- Recreate as index
CREATE UNIQUE INDEX idx_discord_messages_unique 
ON discord_messages (message_id, time);
