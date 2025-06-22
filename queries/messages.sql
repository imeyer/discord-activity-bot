-- name: InsertMessage :exec
INSERT INTO discord_messages (
    time, user_id, channel_id, server_id, message_id
) VALUES (
    $1, $2, $3, $4, $5
) ON CONFLICT (message_id, time) DO NOTHING;

-- name: GetChattiestUsers :many
SELECT user_id, COUNT(*) as message_count
FROM discord_messages
WHERE server_id = @server_id
  AND channel_id = @channel_id
  AND time > CASE 
    WHEN @period = 'today' THEN NOW() - INTERVAL '1 day'
    WHEN @period = 'yesterday' THEN NOW() - INTERVAL '2 days'
    WHEN @period = 'week' THEN NOW() - INTERVAL '7 days'
    WHEN @period = 'month' THEN NOW() - INTERVAL '30 days'
    ELSE NOW() - INTERVAL '7 days'
  END
  AND (@period != 'yesterday' OR time <= NOW() - INTERVAL '1 day')
GROUP BY user_id
ORDER BY message_count DESC
LIMIT @limit_count;

-- name: GetUserStats :one
WITH periods AS (
    SELECT 
        CASE 
            WHEN @period = 'today' THEN '1 day'::interval
            WHEN @period = 'week' THEN '7 days'::interval
            ELSE '30 days'::interval
        END as current_period
),
current_stats AS (
    SELECT 
        COUNT(*) as message_count,
        COUNT(DISTINCT discord_messages.channel_id) as channels_active,
        COUNT(DISTINCT DATE_TRUNC('day', discord_messages.time)) as days_active
    FROM discord_messages, periods
    WHERE discord_messages.user_id = @user_id
      AND discord_messages.server_id = @server_id
      AND discord_messages.time > NOW() - periods.current_period
),
previous_stats AS (
    SELECT COUNT(*) as prev_count
    FROM discord_messages, periods
    WHERE discord_messages.user_id = @user_id
      AND discord_messages.server_id = @server_id
      AND discord_messages.time > NOW() - (2 * periods.current_period)
      AND discord_messages.time <= NOW() - periods.current_period
)
SELECT 
    COALESCE(c.message_count, 0)::bigint as message_count,
    COALESCE(c.channels_active, 0)::bigint as channels_active,
    COALESCE(c.days_active, 0)::bigint as days_active,
    COALESCE(p.prev_count, 0)::bigint as prev_count,
    CASE 
        WHEN p.prev_count > 0 THEN 
            ROUND(100.0 * (c.message_count - p.prev_count) / p.prev_count, 1)
        ELSE 0
    END::float8 as percent_change
FROM current_stats c, previous_stats p;

-- name: GetTrendingUsers :many
WITH weekly_stats AS (
    SELECT 
        user_id,
        COUNT(CASE WHEN time > NOW() - INTERVAL '7 days' THEN 1 END) as this_week,
        COUNT(CASE WHEN time > NOW() - INTERVAL '14 days' 
                   AND time <= NOW() - INTERVAL '7 days' THEN 1 END) as last_week
    FROM discord_messages
    WHERE server_id = @server_id
      AND time > NOW() - INTERVAL '14 days'
    GROUP BY user_id
    HAVING COUNT(CASE WHEN time > NOW() - INTERVAL '7 days' THEN 1 END) > 10
)
SELECT 
    user_id,
    this_week::bigint,
    last_week::bigint,
    CASE 
        WHEN last_week > 0 THEN 
            ROUND(100.0 * (this_week - last_week) / last_week, 1)
        ELSE 100
    END::float8 as percent_change
FROM weekly_stats
WHERE this_week != last_week
ORDER BY percent_change DESC
LIMIT @limit_count;

-- name: GetUserDailyActivity :many
SELECT 
    time_bucket('1 day', time) AS day,
    COUNT(*) as message_count
FROM discord_messages
WHERE user_id = @user_id
  AND server_id = @server_id
  AND time > NOW() - INTERVAL '30 days'
GROUP BY day
ORDER BY day DESC;

-- name: GetInactiveUsers :many
-- Find users who haven't posted in N days
WITH server_users AS (
    -- Get all users who have ever posted in the server
    SELECT DISTINCT user_id
    FROM discord_messages
    WHERE discord_messages.server_id = @server_id
),
recent_activity AS (
    -- Get users who posted within the inactive period
    SELECT DISTINCT user_id
    FROM discord_messages
    WHERE discord_messages.server_id = @server_id
      AND time > NOW() - (@days::int || ' days')::interval
),
last_activity AS (
    -- Get last message time for all users
    SELECT 
        user_id,
        MAX(time) as last_message_time,
        COUNT(*) as total_messages
    FROM discord_messages
    WHERE discord_messages.server_id = @server_id
    GROUP BY user_id
)
SELECT 
    su.user_id,
    COALESCE(la.last_message_time, NULL) as last_message_time,
    COALESCE(la.total_messages, 0)::bigint as total_messages,
    EXTRACT(EPOCH FROM (NOW() - la.last_message_time)) / 86400 as days_inactive
FROM server_users su
LEFT JOIN last_activity la ON su.user_id = la.user_id
WHERE su.user_id NOT IN (SELECT user_id FROM recent_activity)
  AND la.last_message_time IS NOT NULL
ORDER BY la.last_message_time DESC
LIMIT @limit_count;

-- name: GetInactiveChannels :many
-- Find channels that haven't had posts in N weeks
WITH channel_activity AS (
    SELECT 
        channel_id,
        MAX(time) as last_message_time,
        COUNT(*) as total_messages,
        COUNT(DISTINCT user_id) as unique_users
    FROM discord_messages
    WHERE discord_messages.server_id = @server_id
    GROUP BY channel_id
)
SELECT 
    channel_id,
    last_message_time,
    total_messages::bigint,
    unique_users::bigint,
    EXTRACT(EPOCH FROM (NOW() - last_message_time)) / 604800 as weeks_inactive
FROM channel_activity
WHERE last_message_time < NOW() - (@weeks::int || ' weeks')::interval
ORDER BY last_message_time DESC
LIMIT @limit_count;

-- name: GetServerActivitySummary :one
-- Get overall server activity metrics
WITH time_periods AS (
    SELECT 
        COUNT(DISTINCT user_id) FILTER (WHERE time > NOW() - INTERVAL '1 day') as active_users_today,
        COUNT(DISTINCT user_id) FILTER (WHERE time > NOW() - INTERVAL '7 days') as active_users_week,
        COUNT(DISTINCT user_id) FILTER (WHERE time > NOW() - INTERVAL '30 days') as active_users_month,
        COUNT(DISTINCT channel_id) FILTER (WHERE time > NOW() - INTERVAL '1 day') as active_channels_today,
        COUNT(DISTINCT channel_id) FILTER (WHERE time > NOW() - INTERVAL '7 days') as active_channels_week,
        COUNT(DISTINCT channel_id) FILTER (WHERE time > NOW() - INTERVAL '30 days') as active_channels_month,
        COUNT(*) FILTER (WHERE time > NOW() - INTERVAL '1 day') as messages_today,
        COUNT(*) FILTER (WHERE time > NOW() - INTERVAL '7 days') as messages_week,
        COUNT(*) FILTER (WHERE time > NOW() - INTERVAL '30 days') as messages_month
    FROM discord_messages
    WHERE discord_messages.server_id = @server_id
),
total_stats AS (
    SELECT 
        COUNT(DISTINCT user_id) as total_users,
        COUNT(DISTINCT channel_id) as total_channels,
        COUNT(*) as total_messages
    FROM discord_messages
    WHERE discord_messages.server_id = @server_id
)
SELECT 
    COALESCE(tp.active_users_today, 0)::bigint as active_users_today,
    COALESCE(tp.active_users_week, 0)::bigint as active_users_week,
    COALESCE(tp.active_users_month, 0)::bigint as active_users_month,
    COALESCE(tp.active_channels_today, 0)::bigint as active_channels_today,
    COALESCE(tp.active_channels_week, 0)::bigint as active_channels_week,
    COALESCE(tp.active_channels_month, 0)::bigint as active_channels_month,
    COALESCE(tp.messages_today, 0)::bigint as messages_today,
    COALESCE(tp.messages_week, 0)::bigint as messages_week,
    COALESCE(tp.messages_month, 0)::bigint as messages_month,
    COALESCE(ts.total_users, 0)::bigint as total_users,
    COALESCE(ts.total_channels, 0)::bigint as total_channels,
    COALESCE(ts.total_messages, 0)::bigint as total_messages
FROM time_periods tp, total_stats ts;
