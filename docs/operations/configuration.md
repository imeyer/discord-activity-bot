# Configuration Guide

Complete reference for all configuration options, environment variables, and deployment settings.

## Required Configuration

### Discord Bot Token
**Environment Variable**: `DISCORD_TOKEN`  
**Required**: Yes  
**Description**: Discord bot token for API authentication

```bash
export DISCORD_TOKEN="your-bot-token-here"
```

**Obtaining Token:**
1. Visit [Discord Developer Portal](https://discord.com/developers/applications)
2. Create "New Application" 
3. Navigate to "Bot" tab
4. Copy token from "Token" section
5. Enable "MESSAGE CONTENT INTENT" in Privileged Gateway Intents

**Security**: Never commit tokens to source code. Token is automatically masked in logs.

**Source Code**: [`cmd/discord-activity-bot/main.go:155`](../../cmd/discord-activity-bot/main.go#L155)

## Database Configuration

### PostgreSQL Connection
**Environment Variable**: `DATABASE_URL`  
**Required**: Yes (falls back to localhost)  
**Default**: `postgres://localhost/discord_activity?sslmode=disable`

```bash
export DATABASE_URL="postgres://user:pass@host:5432/dbname?sslmode=require"
```

**Connection String Format:**
```
postgres://[user[:password]@][host][:port]/dbname[?param1=value1&param2=value2]
```

**Common Parameters:**
- `sslmode=require` - Force TLS encryption (auto-enabled in production)
- `pool_max_conns=25` - Maximum connection pool size
- `pool_min_conns=5` - Minimum connection pool size
- `connect_timeout=10` - Connection timeout in seconds

**Production Security:**
- TLS automatically enforced when `ENVIRONMENT=production`
- SSL mode upgraded from `disable`/`prefer` to `require`
- Connection pooling with pgxpool for efficiency

**Source Code**: [`cmd/discord-activity-bot/main.go:181`](../../cmd/discord-activity-bot/main.go#L181)  
**Pool Configuration**: [`db/db.go:20`](../../db/db.go#L20)

### TimescaleDB Requirements
The service requires PostgreSQL with TimescaleDB extension:

```sql
-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Run migrations to create hypertables
-- See migrations/ directory for complete schema
```

**Database Schema**: [`migrations/`](../../migrations/) - Complete migration history

## Observability Configuration

### OpenTelemetry Integration
**Environment Variable**: `OTEL_EXPORTER_OTLP_ENDPOINT`  
**Required**: No (graceful degradation)  
**Default**: Disabled (noop providers)

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
export OTEL_SERVICE_NAME="discord-activity-bot"
export SERVICE_VERSION="1.0.0"
```

**OTLP Configuration:**
- **HTTP Endpoint**: OTLP over HTTP (recommended)
- **Trace Export**: 30-second intervals
- **Metric Export**: 30-second intervals  
- **Sampling**: 100% sampling (configurable)

**Supported Endpoints:**
- Jaeger: `http://jaeger:14268/api/traces`
- Honeycomb: `https://api.honeycomb.io`
- Local OTEL Collector: `http://localhost:4318`

**Fallback Behavior:**
- Service continues operation if OTLP endpoint unavailable
- Falls back to noop tracer and metrics providers
- Warning logged but no service disruption

**Source Code**: [`internal/pkg/otel.go:45`](../../internal/pkg/otel.go#L45)

### Service Identification
**Environment Variable**: `OTEL_SERVICE_NAME`  
**Default**: `"activity"`  
**Description**: Service name for distributed tracing

**Environment Variable**: `SERVICE_VERSION`  
**Default**: `"1.0.0-rc0"`  
**Description**: Service version for telemetry (auto-injected in builds)

### Metrics Server
**Environment Variable**: `METRICS_PORT`  
**Default**: `"8080"`  
**Description**: HTTP port for internal metrics and health checks

```bash
export METRICS_PORT="9090"
export METRICS_AUTH_TOKEN="secure-bearer-token"  # Optional authentication
export METRICS_BIND_ALL="true"                   # Bind to all interfaces (default: localhost)
```

**Endpoints:**
- `GET /metrics` - JSON metrics (optional bearer auth)
- `GET /health` - Health check (always accessible)

**Security Options:**
- **Bearer Authentication**: Optional `METRICS_AUTH_TOKEN`
- **Interface Binding**: Localhost-only by default
- **No External Dependencies**: Self-contained health checks

**Source Code**: [`internal/bot/metrics_server.go:30`](../../internal/bot/metrics_server.go#L30)

## Operational Configuration

### Logging Configuration
**Environment Variable**: `LOG_LEVEL`  
**Default**: `"info"`  
**Options**: `debug`, `info`, `warn`, `error`

```bash
export LOG_LEVEL="debug"
```

**Log Formats:**
- **Structured Logging**: JSON format with contextual fields
- **Request Tracing**: Unique request IDs for correlation
- **Performance Metrics**: Operation timing and resource usage

**Log Categories:**
- **Discord Events**: Message processing, command execution
- **Database Operations**: Query timing, connection health
- **Chart Generation**: Rendering performance, error handling
- **Admin Actions**: Permission changes, configuration updates

**Source Code**: [`internal/pkg/logger.go:15`](../../internal/pkg/logger.go#L15)

### Environment Settings
**Environment Variable**: `ENVIRONMENT`  
**Default**: `"development"`  
**Options**: `development`, `production`

```bash
export ENVIRONMENT="production"
```

**Production Behavior:**
- **TLS Enforcement**: Database connections require SSL
- **Enhanced Security**: Stricter input validation
- **Performance Optimization**: Optimized for throughput
- **Resource Limits**: Conservative memory and CPU usage

**Development Behavior:**  
- **Relaxed SSL**: Allows unencrypted database connections
- **Debug Logging**: More verbose logging by default
- **Development Tools**: Additional debugging endpoints

## Feature Configuration

### Rate Limiting
Rate limiting is built-in and not configurable via environment variables:

**Per-User Limits:**
- **Command Cooldown**: 5 seconds between same command type
- **Message Rate**: 60 messages per minute per user
- **Burst Allowance**: Up to 10 messages in burst

**Per-Server Limits:**
- **Total Messages**: 600 messages per minute per server
- **Command Concurrency**: Limited concurrent command processing
- **Database Connections**: Circuit breaker protection

**Implementation**: [`internal/pkg/command_ratelimit.go`](../../internal/pkg/command_ratelimit.go)

### Circuit Breaker Settings
Database operations protected by circuit breaker (not user-configurable):

**Settings:**
- **Failure Threshold**: 5 consecutive failures
- **Timeout**: 10 seconds
- **Recovery**: Automatic recovery after success

**Source Code**: [`internal/pkg/circuitbreaker.go`](../../internal/pkg/circuitbreaker.go)

## Configuration Validation

### Startup Validation
The service validates configuration on startup:

**Required Checks:**
- Discord token format and validity
- Database connectivity and schema version
- OpenTelemetry endpoint accessibility (if configured)

**Validation Errors:**
```bash
# Missing required configuration
ERROR Discord token is required. Use --token flag or DISCORD_TOKEN environment variable

# Database connection failure  
ERROR failed to connect to database error="connection refused"

# Invalid configuration values
ERROR invalid log level: "invalid" (valid: debug, info, warn, error)
```

### Runtime Configuration Changes
Some configuration can be updated without restart:

**Dynamic Configuration:**
- **Log Level**: Can be changed via admin commands or signals
- **OpenTelemetry**: Endpoint changes require restart
- **Database**: Connection pooling parameters require restart

## Configuration Examples

### Local Development
```bash
#!/bin/bash
# Local development configuration
export DISCORD_TOKEN="your-dev-bot-token"
export DATABASE_URL="postgres://localhost/discord_activity_dev?sslmode=disable"
export LOG_LEVEL="debug"
export ENVIRONMENT="development"
export METRICS_PORT="8080"

# Optional: Local OpenTelemetry
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"

./discord-activity-bot
```

### Production Docker
```bash
#!/bin/bash
# Production container configuration
export DISCORD_TOKEN="$SECRET_DISCORD_TOKEN"
export DATABASE_URL="postgres://user:$DB_PASSWORD@postgres:5432/discord_activity?sslmode=require"
export LOG_LEVEL="info"
export ENVIRONMENT="production"
export METRICS_PORT="8080"
export METRICS_AUTH_TOKEN="$METRICS_TOKEN"

# Production OpenTelemetry
export OTEL_EXPORTER_OTLP_ENDPOINT="https://api.honeycomb.io"
export OTEL_SERVICE_NAME="discord-activity-bot"

docker run -e DISCORD_TOKEN -e DATABASE_URL -e LOG_LEVEL \
  -e ENVIRONMENT -e OTEL_EXPORTER_OTLP_ENDPOINT \
  ghcr.io/imeyer/discord-activity-bot:latest
```

### Kubernetes Deployment
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: discord-bot-config
data:
  LOG_LEVEL: "info"
  ENVIRONMENT: "production"
  METRICS_PORT: "8080"
  OTEL_SERVICE_NAME: "discord-activity-bot"
---
apiVersion: v1
kind: Secret
metadata:
  name: discord-bot-secrets
type: Opaque
stringData:
  DISCORD_TOKEN: "your-production-token"
  DATABASE_URL: "postgres://user:password@postgres:5432/discord_activity?sslmode=require"
  OTEL_EXPORTER_OTLP_ENDPOINT: "https://api.honeycomb.io"
  METRICS_AUTH_TOKEN: "secure-metrics-token"
```

## Command Line Interface

All configuration can be overridden via command line flags:

```bash
./discord-activity-bot \
  --token="bot-token" \
  --db-url="postgres://localhost/discord_activity" \
  --log-level="debug" \
  --env="development" \
  --metrics-port="9090" \
  --otel-endpoint="http://localhost:4318" \
  --service-name="my-discord-bot"
```

**Help Information:**
```bash
./discord-activity-bot --help
./discord-activity-bot --version
```

**Source Code**: [`cmd/discord-activity-bot/main.go:33`](../../cmd/discord-activity-bot/main.go#L33)