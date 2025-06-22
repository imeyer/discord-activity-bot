# Administrative Commands API Reference

Administrative commands for server management and advanced analytics, restricted to users with proper permissions.

## Permission System

### Permission Requirements
Administrative commands require **one of**:
- Discord **Administrator** permission (always granted)
- Custom admin role configured via `/add-bot-admin-role`

### Role Management
Admin roles are stored per-server in the database, allowing flexible permission delegation to roles like "Moderator" or "Staff" without requiring full Discord Administrator permissions.

**Source Code**: [`internal/bot/admin_commands.go:25`](../../internal/bot/admin_commands.go#L25)  
**Database Table**: [`migrations/000004_add_admin_roles.up.sql`](../../migrations/000004_add_admin_roles.up.sql)

## Server Analytics Commands

### `/inactive-users` - Dormant User Detection

Identifies users who haven't posted messages in a specified number of days.

**Parameters:**
- `days` (required): Number of days of inactivity (range: 1-365)
- `limit` (optional): Maximum users to show (range: 1-50, default: 10)

**Use Cases:**
- Member cleanup and engagement initiatives
- Identifying users for re-engagement campaigns  
- Server activity health assessment

**Response Format:**
```
😴 Inactive Users (No messages in last 30 days)

1. @user1 - Last active: 45 days ago (1,234 total messages)
2. @user2 - Last active: 52 days ago (567 total messages)
3. @user3 - Last active: 67 days ago (89 total messages)
...

Found 25 inactive users total.
💡 Consider re-engagement or cleanup campaigns.
```

**Data Analysis:**
- Includes total historical message count
- Excludes bots from analysis
- Sorts by longest inactivity period first

**Source Code**: [`internal/bot/admin_commands.go:78`](../../internal/bot/admin_commands.go#L78)  
**Database Query**: [`queries/analytics.sql:GetInactiveUsers`](../../queries/analytics.sql)

### `/inactive-channels` - Channel Usage Analysis

Finds channels that haven't had posts in a specified number of weeks.

**Parameters:**
- `weeks` (required): Number of weeks of inactivity (range: 1-52)
- `limit` (optional): Maximum channels to show (range: 1-50, default: 10)

**Use Cases:**
- Channel cleanup and organization
- Identifying unused or abandoned channels
- Server structure optimization

**Response Format:**
```
📭 Inactive Channels (No posts in last 4 weeks)

1. #old-project - Last post: 6 weeks ago (234 total messages)
2. #archived-discussion - Last post: 8 weeks ago (1,567 total messages)  
3. #temp-channel - Last post: 12 weeks ago (45 total messages)
...

Found 8 inactive channels total.
💡 Consider archiving or repurposing these channels.
```

**Analysis Features:**
- Includes historical message counts
- Shows exact time since last activity
- Identifies completely empty channels

**Source Code**: [`internal/bot/admin_commands.go:149`](../../internal/bot/admin_commands.go#L149)  
**Database Query**: [`queries/analytics.sql:GetInactiveChannels`](../../queries/analytics.sql)

### `/server-activity` - Comprehensive Server Report

Provides detailed server activity breakdown with visual chart representation.

**Parameters:** None

**Report Includes:**
- **User Engagement**: Active vs inactive user percentages
- **Channel Usage**: Active vs unused channel statistics  
- **Message Volume**: Daily, weekly, and monthly totals
- **Engagement Score**: Calculated server health metric
- **Visual Chart**: Server activity donut chart

**Response Format:**
```
📊 Server Activity Report

👥 User Engagement (This Week):
• Active Users: 245/890 (27.5%)
• Daily Average: 35 active users
• New Members: 12 this week

📺 Channel Activity:
• Active Channels: 18/25 (72.0%)
• Most Active: #general (2,456 messages)
• Engagement Score: 8.2/10

📈 Message Volume:
• Today: 1,234 messages  
• This Week: 8,567 messages
• This Month: 34,890 messages

[PNG Chart Attachment: User engagement donut chart]
```

**Visual Elements:**
- Donut chart showing active vs inactive user percentages
- Color-coded engagement levels
- Trend indicators and comparisons

**Source Code**: [`internal/bot/admin_commands.go:220`](../../internal/bot/admin_commands.go#L220)  
**Chart Generation**: [`internal/charts/image_graph.go:489`](../../internal/charts/image_graph.go#L489)  
**Database Query**: [`queries/analytics.sql:GetServerActivitySummary`](../../queries/analytics.sql)

## Permission Management Commands

### `/add-bot-admin-role` - Grant Admin Access

Grants a Discord role access to bot administrative commands.

**Parameters:**
- `role` (required): Discord role to grant admin access

**Security Features:**
- Prevents granting access to @everyone role
- Requires existing Administrator permission to use
- Validates role existence and accessibility

**Response Format:**
```
✅ Role @Moderator has been granted bot admin access.

Members with this role can now use:
• /inactive-users • /inactive-channels • /server-activity
• Admin role management commands
```

**Database Storage:**
- Stored per-server in `admin_roles` table
- Includes metadata: added by user ID, timestamp
- Persistent across bot restarts

**Source Code**: [`internal/bot/admin_commands.go:292`](../../internal/bot/admin_commands.go#L292)  
**Database Query**: [`queries/admin_roles.sql:AddAdminRole`](../../queries/admin_roles.sql)

### `/remove-bot-admin-role` - Revoke Admin Access

Removes a Discord role's access to bot administrative commands.

**Parameters:**
- `role` (required): Discord role to remove admin access

**Response Format:**
```
✅ Role @Moderator has been removed from bot admin access.

Members with this role can no longer use administrative commands.
```

**Safety Features:**
- Cannot remove roles that don't have admin access (graceful handling)
- Immediate effect - role members lose access instantly
- Audit trail maintained in database

**Source Code**: [`internal/bot/admin_commands.go:334`](../../internal/bot/admin_commands.go#L334)  
**Database Query**: [`queries/admin_roles.sql:RemoveAdminRole`](../../queries/admin_roles.sql)

### `/list-bot-admin-roles` - Permission Audit

Lists all roles with bot administrative command access.

**Parameters:** None

**Response Format:**
```
🔧 Bot Admin Roles for This Server

Roles with admin access:
• @Moderator (added by @admin_user on 2024-01-15)
• @Staff (added by @server_owner on 2024-01-20)

💡 Note: Users with Discord Administrator permission always have access.
```

**Features:**
- Shows role mention tags for easy identification
- Includes audit information (who added, when)
- Reminds about built-in Administrator access

**Source Code**: [`internal/bot/admin_commands.go:371`](../../internal/bot/admin_commands.go#L371)  
**Database Query**: [`queries/admin_roles.sql:GetAdminRoles`](../../queries/admin_roles.sql)

## Response Channel Configuration

Advanced commands for configuring where bot responses are sent.

### `/set-response-channel` - Configure Response Location

Sets specific channels for bot command responses.

**Parameters:**
- `channel` (required): Channel where responses should be sent
- `command` (optional): Specific command to configure (empty = all commands)

**Use Cases:**
- Centralize bot responses to dedicated channels
- Reduce noise in general conversation channels
- Create analytics dashboards in specific channels

**Configuration Options:**
- **Global**: All commands respond in specified channel
- **Per-Command**: Individual commands can have different response channels
- **Fallback**: Commands without specific configuration use global setting

**Source Code**: [`internal/bot/admin_commands.go:415`](../../internal/bot/admin_commands.go#L415)  
**Database Table**: [`migrations/000005_add_response_channel_config.up.sql`](../../migrations/000005_add_response_channel_config.up.sql)

### `/clear-response-channel` - Remove Configuration

Clears response channel configuration.

**Parameters:**
- `command` (optional): Specific command to clear (empty = clear all)

### `/list-response-channels` - View Configuration

Lists current response channel configurations.

**Parameters:** None

## Administrative Analytics Features

### Advanced Filtering
Admin commands provide enhanced filtering and analysis capabilities:

- **Cross-Server Analysis**: Server owners can analyze patterns across time
- **Bulk Operations**: Higher limits for comprehensive analysis  
- **Historical Data**: Access to extended time ranges for trend analysis
- **Export Capabilities**: Structured data suitable for external analysis

### Audit Trail
All administrative actions are logged with:
- **User ID**: Who performed the action
- **Timestamp**: When the action occurred  
- **Action Details**: What was changed
- **Server Context**: Which server was affected

**Audit Logging**: [`internal/pkg/logger.go`](../../internal/pkg/logger.go)  
**Database Triggers**: [`migrations/`](../../migrations/) - Audit triggers in migration files

## Performance Considerations

### Query Optimization
Administrative commands often involve complex queries:

- **Indexed Queries**: Optimized for common admin query patterns
- **Result Caching**: Temporary caching for expensive analytics
- **Timeout Handling**: Extended timeouts for complex analysis
- **Progress Indicators**: User feedback for long-running operations

### Resource Management
- **Concurrent Limits**: Prevents multiple heavy admin queries simultaneously  
- **Priority Queuing**: User commands prioritized over admin analytics
- **Resource Monitoring**: Admin usage tracked for capacity planning

### Security Safeguards
- **Input Validation**: Enhanced validation for admin command parameters
- **Permission Checks**: Multi-layer verification of admin access
- **Rate Limiting**: Separate rate limits for administrative operations
- **Audit Logging**: All admin actions logged for security review