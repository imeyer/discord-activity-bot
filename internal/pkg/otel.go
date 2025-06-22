package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Global tracer and meter
	Tracer trace.Tracer
	Meter  metric.Meter
	
	// Custom metrics
	CommandCounter         metric.Int64Counter
	CommandRateLimitCounter metric.Int64Counter
	ImageGenerationTimer   metric.Int64Histogram
	DatabaseOperationTimer metric.Int64Histogram
	MessageProcessedCounter metric.Int64Counter
	ErrorCounter           metric.Int64Counter
)

// EnsureInitialized makes sure all OTel globals are initialized with noop versions if needed
func EnsureInitialized() {
	if Tracer == nil {
		Tracer = trace.NewNoopTracerProvider().Tracer("activity")
	}
	if Meter == nil {
		Meter = otel.Meter("activity/bot")
	}
	
	// Initialize metrics with noop versions if not already done
	if CommandCounter == nil {
		CommandCounter, _ = Meter.Int64Counter("bot.commands.total")
	}
	if CommandRateLimitCounter == nil {
		CommandRateLimitCounter, _ = Meter.Int64Counter("bot.commands.rate_limited")
	}
	if ImageGenerationTimer == nil {
		ImageGenerationTimer, _ = Meter.Int64Histogram("bot.image_generation.duration")
	}
	if DatabaseOperationTimer == nil {
		DatabaseOperationTimer, _ = Meter.Int64Histogram("bot.database.operation_duration")
	}
	if MessageProcessedCounter == nil {
		MessageProcessedCounter, _ = Meter.Int64Counter("bot.messages.processed_total")
	}
	if ErrorCounter == nil {
		ErrorCounter, _ = Meter.Int64Counter("bot.errors.total")
	}
}

// InitOTel initializes OpenTelemetry tracing and metrics
func InitOTel(ctx context.Context, logger *slog.Logger) (func(), error) {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "activity"
	}
	
	serviceVersion := os.Getenv("SERVICE_VERSION")
	if serviceVersion == "" {
		serviceVersion = "1.0.0-rc0"
	}
	
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	logger.Info("initializing OpenTelemetry",
		"service", serviceName,
		"version", serviceVersion,
		"environment", environment,
	)

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
			semconv.DeploymentEnvironmentKey.String(environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize tracer provider
	traceCleanup, err := initTracing(ctx, res, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	// Initialize metrics provider
	metricsCleanup, err := initMetrics(ctx, res, logger)
	if err != nil {
		traceCleanup()
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}
	
	// Combined cleanup function
	cleanup := func() {
		traceCleanup()
		metricsCleanup()
	}

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("OpenTelemetry initialized successfully")
	return cleanup, nil
}

func initTracing(ctx context.Context, res *resource.Resource, logger *slog.Logger) (func(), error) {
	// Check if OTLP endpoint is configured
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	}
	
	if otlpEndpoint == "" {
		logger.Info("no OTLP endpoint configured, using noop tracer")
		// Initialize with noop tracer provider for development
		tp := trace.NewNoopTracerProvider()
		otel.SetTracerProvider(tp)
		Tracer = otel.Tracer("activity")
		return func() {}, nil
	}

	logger.Info("configuring OTLP trace exporter", "endpoint", otlpEndpoint)

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(), // Use HTTPS in production
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Adjust sampling in production
	)

	otel.SetTracerProvider(tp)
	Tracer = otel.Tracer("activity")

	cleanup := func() {
		logger.Info("shutting down trace provider")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			logger.Error("failed to shutdown trace provider", "error", err)
		}
	}

	return cleanup, nil
}

func initMetrics(ctx context.Context, res *resource.Resource, logger *slog.Logger) (func(), error) {
	// Check if OTLP metrics endpoint is configured
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
	}
	
	var mp *sdkmetric.MeterProvider
	var cleanup func()
	
	if otlpEndpoint == "" {
		logger.Info("no OTLP metrics endpoint configured, using noop meter provider")
		// Use noop meter provider for development
		mp = sdkmetric.NewMeterProvider()
		cleanup = func() {}
	} else {
		logger.Info("configuring OTLP metrics exporter", "endpoint", otlpEndpoint)
		
		// Create OTLP HTTP metrics exporter
		exporter, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpoint(otlpEndpoint),
			otlpmetrichttp.WithInsecure(), // Use HTTPS in production
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP metrics exporter: %w", err)
		}

		// Create metrics provider with periodic reader
		mp = sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
				sdkmetric.WithInterval(30*time.Second), // Export every 30 seconds
			)),
		)
		
		cleanup = func() {
			logger.Info("shutting down metrics provider")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := mp.Shutdown(ctx); err != nil {
				logger.Error("failed to shutdown metrics provider", "error", err)
			}
		}
	}

	// Set global meter provider
	otel.SetMeterProvider(mp)
	
	// Initialize meter with bot namespace
	Meter = otel.Meter("activity/bot")

	var err error

	// Create custom metrics with bot namespace
	CommandCounter, err = Meter.Int64Counter(
		"bot.commands.total",
		metric.WithDescription("Total number of Discord commands processed"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create command counter: %w", err)
	}

	CommandRateLimitCounter, err = Meter.Int64Counter(
		"bot.commands.rate_limited",
		metric.WithDescription("Total number of Discord commands rate limited"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create command rate limit counter: %w", err)
	}

	ImageGenerationTimer, err = Meter.Int64Histogram(
		"bot.image_generation.duration",
		metric.WithDescription("Duration of image generation operations"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create image generation timer: %w", err)
	}

	DatabaseOperationTimer, err = Meter.Int64Histogram(
		"bot.database.operation_duration",
		metric.WithDescription("Duration of database operations"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create database operation timer: %w", err)
	}

	MessageProcessedCounter, err = Meter.Int64Counter(
		"bot.messages.processed_total",
		metric.WithDescription("Total number of Discord messages processed"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create message counter: %w", err)
	}

	ErrorCounter, err = Meter.Int64Counter(
		"bot.errors.total",
		metric.WithDescription("Total number of errors encountered"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create error counter: %w", err)
	}

	logger.Info("OpenTelemetry metrics initialized with bot namespace")
	return cleanup, nil
}

// StartSpan is a convenience function to start a span with common attributes
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	EnsureInitialized()
	return Tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// RecordError records an error in the current span and increments error counter
func RecordError(ctx context.Context, err error, description string) {
	EnsureInitialized()
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.RecordError(err, trace.WithAttributes(
			attribute.String("error.description", description),
		))
		span.SetStatus(codes.Error, description)
	}
	
	ErrorCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("error.type", description),
	))
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	EnsureInitialized()
	span := trace.SpanFromContext(ctx)
	if span != nil && !span.SpanContext().Equal(trace.SpanContext{}) {
		span.SetAttributes(attrs...)
	}
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	EnsureInitialized()
	span := trace.SpanFromContext(ctx)
	if span != nil && !span.SpanContext().Equal(trace.SpanContext{}) {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}