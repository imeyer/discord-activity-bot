# Discord Activity Bot Documentation

Welcome to the Discord Activity Bot documentation. This service provides comprehensive Discord server analytics through real-time message tracking, visual charts, and detailed activity reports.

## 📚 Documentation Structure

### Quick Start
- [Service Overview](#service-overview) - Architecture and capabilities
- [Configuration Guide](operations/configuration.md) - Environment variables and setup
- [Deployment Guide](operations/deployment.md) - Container and binary deployment

### API Documentation
- [Discord Commands](api/commands.md) - Complete command reference with examples
- [Admin Commands](api/admin-commands.md) - Administrative functionality
- [Chart Generation](api/charts.md) - Visual analytics capabilities

### Operations
- [Monitoring & Metrics](operations/monitoring.md) - Observability and health checks
- [Database Management](operations/database.md) - PostgreSQL/TimescaleDB operations
- [Troubleshooting](operations/troubleshooting.md) - Common issues and solutions

### Development
- [Build Instructions](development/building.md) - Bazel and Go build setup
- [Testing Guide](development/testing.md) - Unit and integration testing
- [Local Development](development/local-setup.md) - Development environment setup
- [Architecture Deep Dive](architecture/overview.md) - Internal architecture details

## Service Overview

**Discord Activity Bot** is a production-ready Discord bot that tracks and visualizes server activity using TimescaleDB for time-series data storage and Gonum for chart generation.

### Core Capabilities
- **Real-time Message Tracking**: Captures all Discord messages with batched database storage
- **Activity Analytics**: 15+ slash commands for user and channel analytics  
- **Visual Charts**: PNG chart generation for activity patterns and trends
- **Admin Management**: Role-based permissions and server administration tools
- **Enterprise Observability**: OpenTelemetry integration with comprehensive metrics

### Key Features
- 📊 **Activity Timeline Charts** - Visual 24-hour activity graphs per channel
- 🏆 **User Rankings** - Top contributors with period-based analysis
- 📈 **Trend Analysis** - Week-over-week growth and activity patterns  
- 🕐 **Peak Hours Heatmaps** - Server activity patterns by hour
- 👤 **User Statistics** - Individual user analytics and profiles
- 🌟 **Rising Stars Detection** - Users with rapidly growing engagement
- 🔧 **Admin Tools** - Inactive user/channel detection and role management

### Architecture Highlights
- **Technology Stack**: Go 1.24, DiscordGo, PostgreSQL/TimescaleDB, Gonum
- **Message Processing**: Buffered batch processing (100 messages/5 seconds)
- **Observability**: OpenTelemetry tracing + custom metrics
- **Reliability**: Circuit breakers, rate limiting, graceful degradation
- **Security**: TLS enforcement, input validation, permission management

### Service Dependencies

| Service | Role | Configuration |
|---------|------|---------------|
| **Discord API** | Primary bot interface | `DISCORD_TOKEN` |
| **PostgreSQL/TimescaleDB** | Time-series data storage | `DATABASE_URL` |
| **OpenTelemetry OTLP** | Distributed tracing/metrics | `OTEL_EXPORTER_OTLP_ENDPOINT` |
| **Internal Metrics Server** | Health checks and metrics | `METRICS_PORT` (8080) |

### Quick Start Commands

```bash
# Build and run locally
make build-bazel
./discord-activity-bot --help

# Container deployment  
docker pull ghcr.io/imeyer/discord-activity-bot:latest
docker run -e DISCORD_TOKEN=xxx -e DATABASE_URL=xxx ghcr.io/imeyer/discord-activity-bot:latest

# Basic Discord commands
/chattiest period:week           # Top message senders this week
/channel-activity               # Visual activity chart for current channel  
/userstats user:@someone        # Individual user analytics
/rising-stars limit:10          # Users with growing activity
```

### Source Code References

- **Main Entry Point**: [`cmd/discord-activity-bot/main.go`](../cmd/discord-activity-bot/main.go) - Service initialization and configuration
- **Bot Core**: [`internal/bot/bot.go`](../internal/bot/bot.go) - Discord integration and message processing
- **Command Handlers**: [`internal/bot/commands.go`](../internal/bot/commands.go) - Public slash command implementation
- **Admin Commands**: [`internal/bot/admin_commands.go`](../internal/bot/admin_commands.go) - Administrative functionality
- **Chart Generation**: [`internal/charts/image_graph.go`](../internal/charts/image_graph.go) - Visual analytics using Gonum
- **Database Layer**: [`db/`](../db/) - SQLC-generated database operations
- **Observability**: [`internal/pkg/otel.go`](../internal/pkg/otel.go) - OpenTelemetry configuration

For detailed information, explore the specific documentation sections linked above.