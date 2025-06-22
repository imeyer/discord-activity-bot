# Discord Activity Bot

**A production-ready Discord bot for comprehensive server activity tracking and analytics.**

[![Build Status](https://github.com/imeyer/discord-activity-bot/workflows/Build%20and%20Publish/badge.svg)](https://github.com/imeyer/discord-activity-bot/actions)
[![Container Image](https://ghcr.io/imeyer/discord-activity-bot/badge.svg)](https://github.com/imeyer/discord-activity-bot/pkgs/container/discord-activity-bot)

> **📚 [Complete Documentation](docs/README.md)** | **🚀 [Quick Start](#quick-start)** | **📊 [Commands](#commands)** | **⚙️ [Configuration](docs/operations/configuration.md)**

## Overview

Discord Activity Bot provides real-time message tracking, visual analytics, and detailed activity reports for Discord servers using TimescaleDB for time-series data storage and Gonum for chart generation.

### ✨ Key Features

- **📊 Visual Analytics** - PNG charts for activity patterns, trends, and statistics
- **🏆 User Rankings** - Top contributors with period-based analysis  
- **📈 Trend Analysis** - Week-over-week growth and activity patterns
- **🕐 Peak Hours** - Server activity heatmaps by hour
- **🌟 Rising Stars** - Users with rapidly growing engagement
- **🔧 Admin Tools** - Inactive user/channel detection and role management
- **⚡ High Performance** - Batched processing, circuit breakers, comprehensive observability

### 🎯 Core Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/chattiest` | Top message senders by period | `/chattiest period:week` |
| `/channel-activity` | Visual 24-hour activity chart | `/channel-activity` |
| `/userstats` | Individual user analytics | `/userstats user:@someone` |
| `/rising-stars` | Users with growing activity | `/rising-stars limit:10` |
| `/peak-hours` | Activity heatmap by hour | `/peak-hours` |
| `/trending` | Week-over-week activity trends | `/trending` |

**Admin Commands**: `/inactive-users`, `/inactive-channels`, `/server-activity`, role management

📖 **[Complete Command Reference](docs/api/commands.md)**

## Prerequisites

- Go 1.24+
- PostgreSQL with TimescaleDB extension
- Discord Bot Token

## Setup

### 1. Install TimescaleDB

**macOS:**
```bash
brew tap timescale/tap
brew install timescaledb
brew services start postgresql@16
```

**Docker:**
```bash
docker run -d --name timescaledb -p 5432:5432 \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=discord_activity \
  timescale/timescaledb:latest-pg16
```

### 2. Install Development Tools

```bash
make install-tools
```

### 3. Configure Environment

```bash
export DATABASE_URL="postgres://localhost/discord_activity?sslmode=disable"
export DISCORD_TOKEN="your-bot-token-here"

# Optional logging configuration
export LOG_LEVEL="info"  # debug, info, warn, error
export LOG_FORMAT="text" # text or json

# Optional: Pre-configure admin roles (otherwise use /add-bot-admin-role)
# export ADMIN_ROLES_<GUILD_ID>="role_id1,role_id2"
```

### 4. Database Setup

```bash
# Create database (if needed)
createdb discord_activity

# Run migrations
make migrate-up

# Generate sqlc code
make sqlc-generate
```

### 5. Discord Bot Setup

1. Go to https://discord.com/developers/applications
2. Create "New Application"
3. Go to "Bot" tab, get token
4. Enable "MESSAGE CONTENT INTENT" in Privileged Gateway Intents
5. Go to OAuth2 → URL Generator:
   - Scopes: `bot`
   - Permissions: `Read Messages`, `Use Slash Commands`, `Administrator` (for admin commands)
6. Use generated URL to invite bot to servers

## Quick Start

### 🐳 Container Deployment (Recommended)

```bash
# Pull latest image from GitHub Container Registry
docker pull ghcr.io/imeyer/discord-activity-bot:latest

# Run with environment variables
docker run -d \
  -e DISCORD_TOKEN="your-bot-token" \
  -e DATABASE_URL="postgres://user:pass@host:5432/discord_activity?sslmode=require" \
  -e LOG_LEVEL="info" \
  -e ENVIRONMENT="production" \
  ghcr.io/imeyer/discord-activity-bot:latest
```

### 🔧 Local Development

```bash
# Clone repository
git clone https://github.com/imeyer/discord-activity-bot.git
cd discord-activity-bot

# Install dependencies and setup database
make install-tools
make migrate-up

# Configure environment
export DISCORD_TOKEN="your-bot-token"
export DATABASE_URL="postgres://localhost/discord_activity?sslmode=disable"

# Build and run
make build-bazel  # or: make build
./discord-activity-bot
```

### 📋 Discord Bot Setup

1. **Create Application**: [Discord Developer Portal](https://discord.com/developers/applications)
2. **Bot Configuration**:
   - Enable **MESSAGE CONTENT INTENT** in Privileged Gateway Intents
   - Copy bot token for `DISCORD_TOKEN`
3. **Invite Bot**: OAuth2 → URL Generator
   - **Scopes**: `bot`, **Permissions**: `Read Messages`, `Use Slash Commands`
4. **Database**: PostgreSQL 16+ with TimescaleDB extension

📖 **[Detailed Setup Guide](docs/operations/configuration.md)**

## Building

### Using Bazel (Recommended)
```bash
# Install Bazelisk (if not already installed)
# On macOS: brew install bazelisk
# On Linux: Download from https://github.com/bazelbuild/bazelisk

# Build with version injection
make build-bazel

# Build for Linux
make build-bazel-linux

# Test
make test-bazel

# Show version info
./discord-activity-bot --version
```

### Using Go directly
```bash
# Build with version injection
make build

# Build for Linux
make build-linux

# Test
make test
```

## Development

```bash
make help              # Show all commands
make db-reset         # Reset database (WARNING: destroys data)
make migrate-create   # Create new migration
make version          # Show version info that would be built
make test-bazel       # Run tests with Bazel (recommended)
make test             # Run tests with Go
```

## Architecture

- **Go + discordgo**: Discord bot implementation
- **TimescaleDB**: Time-series data with automatic partitioning
- **sqlc**: Type-safe SQL queries
- **golang-migrate**: Database migrations
- **pgx**: PostgreSQL driver with connection pooling

### Data Flow

1. Bot receives Discord messages
2. Messages batched in memory (100 messages or 5 seconds)
3. Batch inserted into TimescaleDB hypertable
4. Continuous aggregates maintain hourly/daily statistics
5. Slash commands query optimized views

## Visual Analytics

The bot provides several visual representations of server activity:

### Channel Activity Timeline
```
/channel-activity [channel]
```
Displays a 24-hour timeline graph showing message activity in 15-minute intervals with different symbols for each active user. Perfect for understanding conversation patterns.

### Rising Stars
```
/rising-stars [limit]
```
Identifies users with 20%+ growth in activity week-over-week. Great for recognizing emerging community contributors and potential moderators.

### Peak Hours Heatmap
```
/peak-hours [channel]
```
Shows a visual heatmap of server/channel activity by hour over the last 30 days. Helps identify:
- Best times to post announcements
- When to schedule events
- Moderator coverage needs

## Admin Commands

### Permission Model

By default, only users with Discord's Administrator permission can use admin commands. Server administrators can extend access to other roles (like Moderators) using the role management commands.

### Available Admin Commands

#### Analytics Commands
- **`/inactive-users`** - Find users who haven't posted recently
  - `days`: Number of days of inactivity (1-365)
  - `limit`: Max users to show (1-50, default: 10)

- **`/inactive-channels`** - Find channels with no recent activity
  - `weeks`: Number of weeks of inactivity (1-52)
  - `limit`: Max channels to show (1-50, default: 10)

- **`/server-activity`** - Comprehensive activity report showing:
  - Active users/channels (today, week, month)
  - Message volume statistics
  - Engagement scores

#### Role Management Commands
- **`/add-bot-admin-role`** - Grant a role access to bot admin commands
  - `role`: Role to grant admin access (cannot be @everyone)
  
- **`/remove-bot-admin-role`** - Remove a role's admin access
  - `role`: Role to remove from admin access
  
- **`/list-bot-admin-roles`** - View all roles with admin access
  - Shows configured roles and reminds that Administrators always have access

### Example Usage

1. Server admin grants Moderator role access to admin commands:
   ```
   /add-bot-admin-role role:@Moderator
   ```

2. Moderator can now check inactive users:
   ```
   /inactive-users days:7 limit:20
   ```

3. View current admin roles:
   ```
   /list-bot-admin-roles
   ```

## Security Features

- Rate limiting (60 msgs/min per user, 600/min per guild)
- Circuit breaker for database operations
- Row-level security in PostgreSQL
- GDPR compliance endpoints
- Comprehensive audit logging

## 📊 Monitoring & Operations

### Health Checks
```bash
# Health endpoint
curl http://localhost:8080/health

# Metrics (with optional authentication)
curl -H "Authorization: Bearer token" http://localhost:8080/metrics
```

### OpenTelemetry Integration
```bash
# Enable distributed tracing
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
export OTEL_SERVICE_NAME="discord-activity-bot"
```

**📈 [Monitoring Guide](docs/operations/monitoring.md)** | **🛠️ [Troubleshooting](docs/operations/troubleshooting.md)**

## 🏗️ Architecture

- **Go 1.24** with DiscordGo for Discord API integration
- **TimescaleDB** for time-series message storage and analytics
- **Gonum Plot** for PNG chart generation with Discord theming
- **OpenTelemetry** for distributed tracing and metrics
- **Bazel** for reproducible builds and dependency management

**🔧 [Architecture Deep Dive](docs/architecture/overview.md)** | **🛠️ [Development Guide](docs/development/building.md)**

## 📄 Documentation

- **[API Reference](docs/api/commands.md)** - Complete command documentation
- **[Admin Commands](docs/api/admin-commands.md)** - Administrative functionality  
- **[Configuration](docs/operations/configuration.md)** - Environment variables and setup
- **[Monitoring](docs/operations/monitoring.md)** - Metrics, tracing, and observability
- **[Development](docs/development/building.md)** - Build instructions and local setup
- **[Architecture](docs/architecture/overview.md)** - Internal design and patterns

## 🤝 Contributing

1. **Fork** the repository
2. **Create** your feature branch (`git checkout -b feature/amazing-feature`)
3. **Test** your changes (`make test-bazel`)
4. **Commit** your changes (`git commit -m 'Add amazing feature'`)
5. **Push** to the branch (`git push origin feature/amazing-feature`)
6. **Open** a Pull Request

**[Development Setup Guide](docs/development/building.md)**

## 📋 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.