# OpenTelemetry Configuration

## Service Information
- **Service Name**: `activity`
- **Version**: `1.0.0-rc0`
- **Meter Namespace**: `activity/bot`

## Metrics Configuration

### Environment Variables
```bash
# OpenTelemetry Service Configuration
OTEL_SERVICE_NAME=activity                    # Override service name (default: activity)
SERVICE_VERSION=1.0.0-rc0                    # Override version (default: 1.0.0-rc0)
ENVIRONMENT=production                        # Environment (default: development)

# OTLP Endpoints
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318        # Combined endpoint
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost:4318/v1/traces
OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=http://localhost:4318/v1/metrics

# Authentication (if required)
OTEL_EXPORTER_OTLP_HEADERS="api-key=your-api-key"
```

### Available Metrics

#### Bot Namespace (`activity/bot`)
All metrics are prefixed with `bot.` and include labels for detailed analysis:

**Command Metrics:**
- `bot.commands.total` - Total commands executed
  - Labels: `command` (command name)
- `bot.commands.rate_limited` - Commands blocked by rate limiting  
  - Labels: `command` (command name)

**Performance Metrics:**
- `bot.image_generation.duration` - Chart generation time (ms)
  - Labels: `chart_type` (e.g., "channel_activity", "peak_hours")
- `bot.database.operation_duration` - Database query time (ms)
  - Labels: `operation` (e.g., "insert", "query")

**System Metrics:**
- `bot.messages.processed_total` - Discord messages processed
- `bot.errors.total` - Total errors encountered
  - Labels: `error.type` (error category)

### Export Configuration
- **Export Interval**: 30 seconds
- **Transport**: OTLP HTTP
- **Format**: Protobuf
- **Fallback**: No-op providers when endpoints not configured

## Example OTLP Collector Configuration

```yaml
# otel-collector.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [jaeger]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```

## Prometheus Metrics Example

When exported to Prometheus, metrics will appear as:
```
# Command executions by type
bot_commands_total{command="channel-activity", service_name="activity"} 42
bot_commands_total{command="chattiest", service_name="activity"} 18

# Rate limiting effectiveness
bot_commands_rate_limited_total{command="channel-activity", service_name="activity"} 3

# Performance monitoring
bot_image_generation_duration_bucket{chart_type="channel_activity", le="1000"} 35
bot_database_operation_duration_bucket{operation="query", le="100"} 89
```

## Development vs Production

**Development (No OTLP endpoint configured):**
- Uses no-op providers
- Metrics still created but not exported
- Logging shows "noop" initialization

**Production (OTLP endpoint configured):**
- Full OTLP export every 30 seconds
- Proper resource attribution
- Graceful shutdown handling

## Monitoring Dashboard Queries

**Command Usage Rate:**
```promql
rate(bot_commands_total[5m])
```

**Rate Limiting Percentage:**
```promql
rate(bot_commands_rate_limited_total[5m]) / rate(bot_commands_total[5m]) * 100
```

**95th Percentile Image Generation Time:**
```promql
histogram_quantile(0.95, rate(bot_image_generation_duration_bucket[5m]))
```