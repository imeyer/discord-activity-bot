# Discord Commands API Reference

Complete reference for all public Discord slash commands available to users.

## User Analytics Commands

### `/chattiest` - Top Message Senders

Shows users who sent the most messages in specified time periods.

**Parameters:**
- `period` (required): Time period to analyze
  - `today` - Messages from current day (00:00 UTC to now)
  - `yesterday` - Messages from previous day
  - `week` - Messages from last 7 days
  - `month` - Messages from last 30 days
- `channel` (optional): Specific channel to analyze (defaults to current channel)

**Response Format:**
```
🏆 Top Chattiest Users - This Week

🥇 @username1 - 1,234 messages
🥈 @username2 - 987 messages  
🥉 @username3 - 654 messages
4. @username4 - 321 messages
5. @username5 - 198 messages
...
```

**Source Code**: [`internal/bot/commands.go:156`](../../internal/bot/commands.go#L156)  
**Database Query**: [`queries/messages.sql:GetChattiestUsers`](../../queries/messages.sql)

### `/userstats` - Individual User Analytics

Shows detailed statistics for a specific user including activity trends.

**Parameters:**
- `user` (required): Discord user to analyze
- `period` (optional): Analysis period (default: "week")
  - `today` - Current day activity
  - `week` - Last 7 days
  - `month` - Last 30 days

**Response Format:**
```
📊 User Statistics for @username

📈 This Week: 234 messages (+15.2% vs last week)
📅 Active Days: 5/7 days
🔥 Most Active: #general (89 messages)
💬 Daily Average: 33.4 messages/day
📍 Current Streak: 3 days
```

**Features:**
- Week-over-week percentage change calculation
- Most active channel identification
- Activity streak tracking
- Daily average calculations

**Source Code**: [`internal/bot/commands.go:219`](../../internal/bot/commands.go#L219)  
**Database Query**: [`queries/analytics.sql:GetUserStats`](../../queries/analytics.sql)

### `/trending` - Activity Trend Analysis

Shows users trending up or down in activity with week-over-week comparisons.

**Parameters:** None

**Filtering Rules:**
- Minimum 10 messages in current week
- Requires previous week data for comparison
- Shows top 10 trending users

**Response Format:**
```
📈 Trending Users (Week-over-Week)

🔥 @user1 - 345 messages (+127.8%)
🚀 @user2 - 234 messages (+89.4%)
📈 @user3 - 198 messages (+45.2%)
❄️ @user4 - 156 messages (-23.1%)
📉 @user5 - 134 messages (-34.7%)
```

**Trend Icons:**
- 🔥 Growth > 100%
- 🚀 Growth 50-100%  
- 📈 Growth 0-50%
- ❄️ Decline 0-25%
- 📉 Decline > 25%

**Source Code**: [`internal/bot/commands.go:298`](../../internal/bot/commands.go#L298)  
**Database Query**: [`queries/analytics.sql:GetTrendingUsers`](../../queries/analytics.sql)

## Visual Analytics Commands

### `/channel-activity` - Activity Timeline Chart

Generates a visual PNG chart showing channel activity over the last 24 hours.

**Parameters:**
- `channel` (optional): Channel to analyze (defaults to current channel)

**Chart Features:**
- **Time Range**: Last 24 hours in hourly intervals
- **Visualization**: Line chart with smooth interpolation
- **User Separation**: Different colored lines per user
- **Theme**: Discord dark theme with vibrant colors
- **Format**: PNG attachment (typically 1000x700 pixels)

**Example Use Cases:**
- Identify peak conversation times
- See user activity patterns
- Analyze channel engagement over time

**Chart Colors:**
- Discord Blurple (#7289DA)
- Bright Green (#57F287)
- Bright Red (#FF6464) 
- Gold (#FFD700)
- Deep Pink (#FF1493)
- Deep Sky Blue (#00BFFF)

**Source Code**: [`internal/bot/commands.go:369`](../../internal/bot/commands.go#L369)  
**Chart Generation**: [`internal/charts/image_graph.go:26`](../../internal/charts/image_graph.go#L26)  
**Database Query**: [`queries/analytics.sql:GetChannelActivityTimeline`](../../queries/analytics.sql)

### `/rising-stars` - Growth Analytics

Shows users with rapidly growing weekly activity (minimum 20% growth).

**Parameters:**
- `limit` (optional): Number of users to show (1-25, default: 10)

**Selection Criteria:**
- Minimum 20% week-over-week growth
- Minimum 10 messages in current week
- Ranked by growth percentage

**Response Format:**
```
🌟 Rising Stars - Growing Activity

📊 Chart: [PNG attachment showing horizontal bar chart]

1. @user1 (+127.8%) - 234 → 534 messages
2. @user2 (+89.4%) - 156 → 295 messages  
3. @user3 (+67.2%) - 123 → 206 messages
...

💡 These users show rapidly growing engagement this week!
```

**Chart Features:**
- Horizontal bar chart visualization
- Current week message counts
- Growth rate percentages
- User ranking display

**Source Code**: [`internal/bot/commands.go:434`](../../internal/bot/commands.go#L434)  
**Chart Generation**: [`internal/charts/image_graph.go:365`](../../internal/charts/image_graph.go#L365)  
**Database Query**: [`queries/analytics.sql:GetRisingStars`](../../queries/analytics.sql)

### `/peak-hours` - Activity Heatmap

Shows server activity patterns by hour over the last 30 days.

**Parameters:**
- `channel` (optional): Specific channel analysis (defaults to server-wide)

**Analysis Features:**
- **Time Range**: Last 30 days aggregated by hour
- **Visualization**: Heatmap-style bar chart
- **Peak Identification**: Automatically identifies busiest hours
- **Color Coding**: Intensity-based color gradients

**Response Format:**
```
🕐 Peak Hours Analysis (Last 30 Days)

📊 Chart: [PNG heatmap attachment]

🔥 Peak Activity Hours:
• 20:00 UTC - 2,456 messages/hour avg
• 21:00 UTC - 2,234 messages/hour avg  
• 19:00 UTC - 2,108 messages/hour avg

💡 Best times for announcements and events!
```

**Source Code**: [`internal/bot/commands.go:509`](../../internal/bot/commands.go#L509)  
**Chart Generation**: [`internal/charts/image_graph.go:266`](../../internal/charts/image_graph.go#L266)  
**Database Query**: [`queries/analytics.sql:GetChannelPeakHours`](../../queries/analytics.sql)

## Command Rate Limiting

All commands are subject to rate limiting to prevent abuse:

**Rate Limits:**
- **Per User**: 5-second cooldown between commands of the same type
- **Global**: Commands processed concurrently with circuit breaker protection

**Rate Limit Response:**
```
⏱️ Please wait 3 more seconds before using `/chattiest` again.
```

**Implementation**: [`internal/pkg/command_ratelimit.go`](../../internal/pkg/command_ratelimit.go)

## Error Handling

### Input Validation
- **Discord User IDs**: 17-19 digit snowflake format validation
- **Channel References**: Valid channel ID or mention format
- **Parameter Ranges**: Enforced minimums and maximums for numeric inputs
- **SQL Injection Protection**: Parameterized queries with pattern detection

### Graceful Degradation
- **Database Timeouts**: 10-second query timeouts with fallback messages
- **Partial Data**: Commands continue with available data on non-critical errors
- **Chart Generation Failures**: Falls back to text-based responses

### User-Friendly Error Messages
```
❌ Channel not found or not accessible
❌ User not found in this server
❌ No activity data available for this period
⚠️ Analysis in progress, please try again in a moment
```

## Performance Characteristics

### Response Times (Typical)
- **Simple queries** (`/chattiest`, `/userstats`): 200-500ms
- **Chart generation** (`/channel-activity`, `/peak-hours`): 1-3 seconds
- **Complex analytics** (`/rising-stars`, `/trending`): 500ms-2 seconds

### Data Freshness
- **Message tracking**: Real-time (< 5 second delay)
- **Analytics**: Near real-time with 5-second batch processing
- **Charts**: Generated on-demand with current data

### Scalability
- **Message volume**: Tested with 10,000+ messages/hour per server
- **Concurrent commands**: Limited to prevent resource exhaustion
- **Database performance**: Optimized with TimescaleDB hypertables and indexes