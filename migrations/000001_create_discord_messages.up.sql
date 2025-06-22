-- Create TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Create main messages table
CREATE TABLE IF NOT EXISTS discord_messages (
    time TIMESTAMPTZ NOT NULL,
    user_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    server_id TEXT NOT NULL,
    message_id TEXT NOT NULL
);

-- Convert to hypertable
SELECT create_hypertable('discord_messages', 'time', if_not_exists => TRUE);

-- Add unique constraint AFTER creating hypertable
CREATE UNIQUE INDEX IF NOT EXISTS idx_discord_messages_unique 
    ON discord_messages (message_id, time);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_discord_messages_user_time 
    ON discord_messages (user_id, time DESC);

CREATE INDEX IF NOT EXISTS idx_discord_messages_channel_time 
    ON discord_messages (channel_id, time DESC);

CREATE INDEX IF NOT EXISTS idx_discord_messages_server_time 
    ON discord_messages (server_id, time DESC);

-- Create continuous aggregate for hourly stats
CREATE MATERIALIZED VIEW IF NOT EXISTS user_hourly_stats
WITH (timescaledb.continuous) AS
SELECT 
    time_bucket('1 hour', time) AS hour,
    server_id,
    user_id,
    channel_id,
    COUNT(*) as message_count
FROM discord_messages
GROUP BY hour, server_id, user_id, channel_id
WITH NO DATA;

-- Create continuous aggregate for daily stats
CREATE MATERIALIZED VIEW IF NOT EXISTS user_daily_stats
WITH (timescaledb.continuous) AS
SELECT 
    time_bucket('1 day', time) AS day,
    server_id,
    user_id,
    channel_id,
    COUNT(*) as message_count
FROM discord_messages
GROUP BY day, server_id, user_id, channel_id
WITH NO DATA;

-- Add refresh policies
SELECT add_continuous_aggregate_policy('user_hourly_stats',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);

SELECT add_continuous_aggregate_policy('user_daily_stats',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);

-- Add retention policy - keeps raw data for 90 days by default
-- Can be customized with RETENTION_DAYS environment variable
SELECT add_retention_policy('discord_messages', 
    COALESCE(
        (SELECT setting::interval FROM pg_settings WHERE name = 'discord_activity.retention_days'),
        INTERVAL '90 days'
    ),
    if_not_exists => TRUE
);
