-- Drop the old unique index if it exists
DROP INDEX IF EXISTS idx_discord_messages_unique;

-- Add proper unique constraint on message_id with time
-- This is required for TimescaleDB hypertables
ALTER TABLE discord_messages 
ADD CONSTRAINT discord_messages_unique_message 
UNIQUE (message_id, time);
