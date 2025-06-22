-- Drop continuous aggregate policies
SELECT remove_continuous_aggregate_policy('user_daily_stats', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('user_hourly_stats', if_exists => TRUE);

-- Drop continuous aggregates
DROP MATERIALIZED VIEW IF EXISTS user_daily_stats;
DROP MATERIALIZED VIEW IF EXISTS user_hourly_stats;

-- Drop table
DROP TABLE IF EXISTS discord_messages;
