-- name: GetChannelActivityTimeline :many
-- Get message counts per user in hourly intervals for a channel in the last 24 hours
SELECT 
    date_trunc('hour', time)::timestamp AS interval_start,
    user_id,
    COUNT(*) as message_count
FROM discord_messages
WHERE channel_id = @channel_id
  AND server_id = @server_id
  AND time >= NOW() - INTERVAL '24 hours'
GROUP BY interval_start, user_id
ORDER BY interval_start, message_count DESC;

-- name: GetRisingStars :many
-- Find users with significant positive activity trends
WITH user_activity AS (
    SELECT 
        user_id,
        COUNT(*) FILTER (WHERE time > NOW() - INTERVAL '7 days') as this_week,
        COUNT(*) FILTER (WHERE time > NOW() - INTERVAL '14 days' AND time <= NOW() - INTERVAL '7 days') as last_week,
        COUNT(*) FILTER (WHERE time > NOW() - INTERVAL '30 days') as last_30_days,
        MIN(time) as first_message_time
    FROM discord_messages
    WHERE server_id = @server_id
    GROUP BY user_id
    HAVING COUNT(*) FILTER (WHERE time > NOW() - INTERVAL '7 days') >= 10 -- Minimum activity threshold
),
growth_metrics AS (
    SELECT 
        user_id,
        this_week,
        last_week,
        last_30_days,
        first_message_time,
        CASE 
            WHEN last_week > 0 THEN ((this_week - last_week)::float / last_week * 100)
            ELSE 100
        END as growth_rate,
        -- Calculate consistency score (messages per day average)
        last_30_days::float / 30 as daily_average
    FROM user_activity
    WHERE first_message_time < NOW() - INTERVAL '14 days' -- Not brand new users
)
SELECT 
    user_id,
    this_week::bigint,
    last_week::bigint,
    last_30_days::bigint,
    growth_rate::float8,
    daily_average::float8,
    first_message_time
FROM growth_metrics
WHERE growth_rate > 20 -- At least 20% growth
  AND this_week > last_week -- Ensure positive growth
ORDER BY growth_rate DESC
LIMIT @limit_count;

-- name: GetChannelPeakHours :many
-- Get hourly activity patterns for a channel over the last 30 days
SELECT 
    EXTRACT(HOUR FROM time)::int as hour_of_day,
    COUNT(*)::bigint as message_count,
    COUNT(DISTINCT user_id)::bigint as unique_users,
    COUNT(DISTINCT DATE(time))::bigint as days_with_activity
FROM discord_messages
WHERE server_id = @server_id
  AND (@channel_id::text IS NULL OR channel_id = @channel_id)
  AND time > NOW() - INTERVAL '30 days'
GROUP BY hour_of_day
ORDER BY hour_of_day;