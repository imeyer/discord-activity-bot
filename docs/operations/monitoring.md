# Monitoring & Observability

Comprehensive guide to monitoring, metrics, tracing, and troubleshooting the Discord Activity Bot.

## Metrics Collection

### Internal Metrics Server

The bot exposes metrics via a built-in HTTP server for monitoring and health checks.

**Default Endpoint**: `http://localhost:8080/metrics`  
**Authentication**: Optional bearer token via `METRICS_AUTH_TOKEN`  
**Configuration**: [`METRICS_PORT`](configuration.md#metrics-server)

**Example Response:**
```json
{
  "status": "healthy",
  "uptime_seconds": 3600,
  "version": "v1.0.0",
  "build_date": "2024-01-15T10:30:00Z",
  "git_commit": "abc123...",
  
  "discord": {
    "connected": true,
    "guilds": 15,
    "latency_ms": 45,
    "events_processed": 125430
  },
  
  "database": {
    "connected": true,
    "pool_active": 8,
    "pool_idle": 17,
    "queries_total": 45230,
    "queries_failed": 12,
    "avg_query_time_ms": 23.5
  },
  
  "commands": {
    "processed_total": 2341,
    "rate_limited": 45,
    "errors": 8,
    "avg_response_time_ms": 245
  },
  
  "charts": {
    "generated_total": 156,
    "generation_errors": 2,
    "avg_generation_time_ms": 1834
  }
}
```

**Authentication Example:**
```bash
curl -H "Authorization: Bearer your-metrics-token" \
  http://localhost:8080/metrics
```

**Source Code**: [`internal/bot/metrics_server.go:69`](../../internal/bot/metrics_server.go#L69)

### Health Check Endpoint

Simple health check for load balancers and orchestrators.

**Endpoint**: `http://localhost:8080/health`  
**Response**: `{"status": "healthy"}` (HTTP 200)  
**No Authentication Required**

**Health Check Logic:**
- Database connectivity verification
- Discord WebSocket connection status
- Circuit breaker state validation
- Memory and goroutine count monitoring

```bash
# Simple health check
curl http://localhost:8080/health

# Exit code based health check
curl -f http://localhost:8080/health || exit 1
```

### OpenTelemetry Metrics

Comprehensive metrics exported via OTLP for external monitoring systems.

**Metrics Categories:**

#### Command Metrics
- `bot.commands.total` - Total commands processed (counter)
- `bot.commands.duration` - Command execution time (histogram)  
- `bot.commands.rate_limited` - Rate limited commands (counter)
- `bot.commands.errors` - Command errors by type (counter)

**Labels:**
- `command` - Command name (chattiest, userstats, etc.)
- `server_id` - Discord server ID
- `error_type` - Error classification

#### Database Metrics  
- `db.queries.total` - Total database queries (counter)
- `db.queries.duration` - Query execution time (histogram)
- `db.connections.active` - Active database connections (gauge)
- `db.connections.idle` - Idle database connections (gauge)

**Labels:**
- `query_type` - Query classification (select, insert, update)
- `table` - Primary table accessed
- `success` - Query success/failure

#### Chart Generation Metrics
- `charts.generated.total` - Charts generated (counter)
- `charts.generation.duration` - Chart generation time (histogram)
- `charts.size_bytes` - Chart output size (histogram)

**Labels:**
- `chart_type` - Chart type (activity, peak_hours, rising_stars)
- `format` - Output format (png)

#### Discord API Metrics
- `discord.events.total` - Discord events received (counter)
- `discord.api.requests` - Discord API requests made (counter)
- `discord.websocket.reconnects` - WebSocket reconnections (counter)

**Configuration**: [`internal/pkg/otel.go:89`](../../internal/pkg/otel.go#L89)

## Distributed Tracing

### OpenTelemetry Traces

Comprehensive tracing of all major operations for performance analysis and debugging.

**Trace Categories:**

#### Command Processing Traces
- **Span**: `discord.command.process`
- **Duration**: Full command lifecycle
- **Attributes**: Command name, user ID, server ID, parameters
- **Child Spans**: Database queries, chart generation, validation

#### Database Operation Traces  
- **Span**: `db.query.execute`
- **Duration**: Query execution time
- **Attributes**: Query type, table, row count, connection info
- **Error Handling**: SQL errors and timeout scenarios

#### Chart Generation Traces
- **Span**: `charts.generate`
- **Duration**: Complete chart rendering pipeline
- **Attributes**: Chart type, data points, output size
- **Child Spans**: Data processing, rendering, file I/O

**Example Trace Structure:**
```
discord.command.process (chattiest)
├── validation.check_permissions
├── db.query.execute (GetChattiestUsers)
├── response.format_message  
└── discord.api.send_response
```

### Trace Sampling

**Sampling Strategy**: 100% sampling (configurable)  
**Rationale**: Full visibility for debugging and performance optimization
**Production Recommendation**: Consider head-based sampling at 10-50% for high volume

### Custom Span Attributes

**Request Context:**
- `discord.user_id` - User who triggered command
- `discord.server_id` - Server where command executed
- `discord.channel_id` - Channel where command executed

**Performance Metrics:**
- `db.query_time_ms` - Database query execution time
- `chart.data_points` - Number of data points processed
- `response.size_bytes` - Response payload size

**Error Information:**
- `error.type` - Error classification
- `error.message` - Error description (sanitized)
- `error.recoverable` - Whether error was recoverable

**Source Code**: [`internal/pkg/tracing_coverage.go`](../../internal/pkg/tracing_coverage.go)

## Logging Strategy

### Structured Logging

The service uses structured logging with JSON output for machine processing.

**Log Levels:**
- **DEBUG**: Detailed debugging information
- **INFO**: General operational messages  
- **WARN**: Warning conditions that don't affect functionality
- **ERROR**: Error conditions requiring attention

**Log Fields:**
- `timestamp` - ISO 8601 timestamp
- `level` - Log level (DEBUG, INFO, WARN, ERROR)
- `msg` - Human-readable message
- `service` - Service name ("discord-activity-bot")
- `version` - Service version
- `request_id` - Unique request identifier for correlation

**Example Log Entry:**
```json
{
  "timestamp": "2024-01-15T10:30:45.123Z",
  "level": "INFO",
  "msg": "command processed successfully",
  "service": "discord-activity-bot",
  "version": "v1.0.0",
  "request_id": "req_abc123",
  "command": "chattiest",
  "user_id": "123456789",
  "server_id": "987654321",
  "duration_ms": 245,
  "success": true
}
```

### Request Correlation

All operations include unique request IDs for end-to-end tracing:

**Request ID Format**: `req_` + 8-character alphanumeric
**Scope**: Entire command processing lifecycle
**Propagation**: Database queries, chart generation, Discord API calls

### Security Considerations

**Sensitive Data Handling:**
- Discord tokens automatically masked in logs
- User content not logged (privacy protection)
- Database credentials sanitized
- Error messages filtered for sensitive information

**Log Retention:**
- Application handles log output only
- Log retention managed by deployment environment
- Recommendation: 30-90 days for operational logs

**Source Code**: [`internal/pkg/logger.go:45`](../../internal/pkg/logger.go#L45)

## Alerting Recommendations

### Critical Alerts

**Service Availability:**
```yaml
# Service down
alert: DiscordBotDown
expr: up{job="discord-activity-bot"} == 0
for: 1m

# Database connectivity
alert: DatabaseConnectionFailed  
expr: discord_bot_database_connected == 0
for: 30s

# Discord API connectivity
alert: DiscordAPIDisconnected
expr: discord_bot_discord_connected == 0  
for: 2m
```

**Performance Degradation:**
```yaml
# High command latency
alert: HighCommandLatency
expr: histogram_quantile(0.95, discord_bot_command_duration_ms) > 5000
for: 5m

# Database query performance
alert: SlowDatabaseQueries
expr: histogram_quantile(0.95, discord_bot_db_query_duration_ms) > 1000
for: 5m

# High error rate
alert: HighErrorRate
expr: rate(discord_bot_commands_errors_total[5m]) > 0.1
for: 3m
```

### Warning Alerts

**Resource Usage:**
```yaml
# High memory usage
alert: HighMemoryUsage
expr: process_resident_memory_bytes > 1000000000  # 1GB
for: 10m

# High goroutine count
alert: HighGoroutineCount
expr: go_goroutines > 1000
for: 10m

# Database connection pool exhaustion
alert: DatabasePoolNearLimit
expr: discord_bot_db_connections_active / discord_bot_db_connections_max > 0.8
for: 5m
```

## Dashboard Recommendations

### Service Overview Dashboard

**Key Metrics:**
- Service uptime and availability
- Command throughput (requests/minute)
- Average response time
- Error rate percentage
- Active Discord servers

**Time Range**: Last 24 hours with 1-hour resolution

### Performance Dashboard

**Database Performance:**
- Query execution time (P50, P95, P99)
- Connection pool utilization
- Query success rate
- Slow query identification

**Chart Generation:**
- Chart generation time by type
- Chart generation success rate
- Peak generation concurrency

### Operational Dashboard

**Discord Integration:**
- WebSocket connection status
- Event processing rate
- API request rate and errors
- Rate limiting statistics

**Resource Utilization:**
- Memory usage trends
- Goroutine count
- CPU utilization
- Network I/O patterns

## Troubleshooting Integration

### Debug Endpoints

When `LOG_LEVEL=debug`, additional debug information is available:

**Goroutine Dump**: Available via metrics endpoint with debug flag
**Memory Statistics**: Detailed memory allocation information
**Connection Status**: Real-time connection pool status

### Log Correlation

**Cross-Reference Methods:**
1. **Request ID**: Trace single command execution across all logs
2. **User ID**: Track specific user's interaction patterns
3. **Server ID**: Analyze server-specific issues
4. **Timestamp Correlation**: Match logs with external monitoring

### Performance Profiling

**pprof Integration**: Available in debug builds
**Profile Types**: CPU, memory, goroutine, block profiles
**Access**: Via metrics server on debug port

```bash
# CPU profile (debug builds only)
go tool pprof http://localhost:8080/debug/pprof/profile

# Memory profile
go tool pprof http://localhost:8080/debug/pprof/heap
```

**Source Code**: [`internal/bot/metrics_server.go:156`](../../internal/bot/metrics_server.go#L156)