package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/imeyer/discord-activity-bot/internal/bot"
	"github.com/imeyer/discord-activity-bot/internal/pkg"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Version information set at build time
var (
	version   = "dev"          // Set by -ldflags "-X main.version=..."
	buildDate = "unknown"      // Set by -ldflags "-X main.buildDate=..."
	gitCommit = "unknown"      // Set by -ldflags "-X main.gitCommit=..."
)

type config struct {
	discordToken string
	databaseURL  string
	metricsPort  string
	logLevel     string
	environment  string
	otelEndpoint string
	serviceName  string
	showHelp     bool
	showVersion  bool
}

func parseFlags() *config {
	cfg := &config{}
	
	flag.StringVar(&cfg.discordToken, "token", os.Getenv("DISCORD_TOKEN"), "Discord bot token (can also use DISCORD_TOKEN env var)")
	flag.StringVar(&cfg.databaseURL, "db-url", os.Getenv("DATABASE_URL"), "PostgreSQL database URL (can also use DATABASE_URL env var)")
	flag.StringVar(&cfg.metricsPort, "metrics-port", getEnvOrDefault("METRICS_PORT", "8080"), "Port for metrics server")
	flag.StringVar(&cfg.logLevel, "log-level", getEnvOrDefault("LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")
	flag.StringVar(&cfg.environment, "env", getEnvOrDefault("ENVIRONMENT", "development"), "Environment (development, production)")
	flag.StringVar(&cfg.otelEndpoint, "otel-endpoint", os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), "OpenTelemetry OTLP endpoint (optional)")
	flag.StringVar(&cfg.serviceName, "service-name", getEnvOrDefault("OTEL_SERVICE_NAME", "discord-activity-bot"), "Service name for OpenTelemetry")
	flag.BoolVar(&cfg.showHelp, "help", false, "Show help message")
	flag.BoolVar(&cfg.showHelp, "h", false, "Show help message (shorthand)")
	flag.BoolVar(&cfg.showVersion, "version", false, "Show version information")
	flag.BoolVar(&cfg.showVersion, "v", false, "Show version information (shorthand)")
	
	flag.Usage = showUsage
	flag.Parse()
	
	return cfg
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func showUsage() {
	fmt.Printf(`Discord Activity Bot %s

A Discord bot that tracks and visualizes server activity with charts and analytics.

USAGE:
  discord-activity-bot [options]

OPTIONS:
`, version)
	flag.PrintDefaults()
	fmt.Printf(`
ENVIRONMENT VARIABLES:
  DISCORD_TOKEN                 Discord bot token (required)
  DATABASE_URL                  PostgreSQL connection string
  METRICS_PORT                  Port for metrics server (default: 8080)
  LOG_LEVEL                     Log level: debug, info, warn, error (default: info)
  ENVIRONMENT                   Environment: development, production (default: development)
  OTEL_EXPORTER_OTLP_ENDPOINT  OpenTelemetry OTLP endpoint (optional)
  OTEL_SERVICE_NAME             Service name for tracing (default: discord-activity-bot)

OPENTELEMETRY:
  To enable OpenTelemetry tracing, set OTEL_EXPORTER_OTLP_ENDPOINT:
    OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 discord-activity-bot
  
  Or use the flag:
    discord-activity-bot --otel-endpoint http://localhost:4318

  The bot will automatically instrument:
    - Discord command processing
    - Database operations
    - Image generation (charts)
    - Error tracking

EXAMPLES:
  # Basic usage with environment variables
  export DISCORD_TOKEN=your_token_here
  export DATABASE_URL=postgres://user:pass@localhost/discord_activity
  discord-activity-bot

  # Using command line flags
  discord-activity-bot --token=your_token --db-url=postgres://localhost/discord_activity

  # Enable OpenTelemetry tracing
  discord-activity-bot --otel-endpoint=http://localhost:4318

  # Production mode with debug logging
  discord-activity-bot --env=production --log-level=debug

`)
}

func main() {
	cfg := parseFlags()
	
	if cfg.showVersion {
		fmt.Printf("Discord Activity Bot\n")
		fmt.Printf("Version:    %s\n", version)
		fmt.Printf("Built:      %s\n", buildDate)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		os.Exit(0)
	}
	
	if cfg.showHelp {
		showUsage()
		os.Exit(0)
	}

	// Set environment variables from flags if provided
	if cfg.otelEndpoint != "" {
		os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", cfg.otelEndpoint)
	}
	if cfg.serviceName != "" {
		os.Setenv("OTEL_SERVICE_NAME", cfg.serviceName)
	}
	if cfg.logLevel != "" {
		os.Setenv("LOG_LEVEL", cfg.logLevel)
	}
	if cfg.environment != "" {
		os.Setenv("ENVIRONMENT", cfg.environment)
	}

	// Setup logger
	startTime := time.Now()
	logger := pkg.SetupLogger()
	loggerDuration := time.Since(startTime)
	
	logger.Info("starting discord activity bot",
		"version", version,
		"build_date", buildDate,
		"git_commit", gitCommit,
		"log_level", cfg.logLevel,
		"environment", cfg.environment,
		"metrics_port", cfg.metricsPort,
		"otel_endpoint", cfg.otelEndpoint,
		"logger_init_duration_sec", loggerDuration.Seconds(),
	)

	// Validate required configuration
	if cfg.discordToken == "" {
		logger.Error("Discord token is required. Use --token flag or DISCORD_TOKEN environment variable")
		fmt.Fprintf(os.Stderr, "\nUse --help for usage information\n")
		os.Exit(1)
	}

	// Initialize OpenTelemetry
	ctx := context.Background()
	otelStart := time.Now()
	otelCleanup, err := pkg.InitOTel(ctx, logger)
	otelDuration := time.Since(otelStart)
	if err != nil {
		logger.Warn("failed to initialize OpenTelemetry, continuing with noop tracer", 
			"error", err,
			"duration_sec", otelDuration.Seconds(),
		)
		// Continue without OTel rather than crashing
		otelCleanup = func() {}
	} else {
		logger.Info("opentelemetry initialized", 
			"duration_sec", otelDuration.Seconds(),
		)
	}
	defer otelCleanup()

	// Get database URL
	dbURL := cfg.databaseURL
	if dbURL == "" {
		dbURL = "postgres://localhost/discord_activity?sslmode=disable"
		logger.Debug("using default database URL")
	}
	
	// Force TLS for production
	if os.Getenv("ENVIRONMENT") == "production" && !strings.Contains(dbURL, "sslmode=require") {
		logger.Warn("forcing TLS for production database connection")
		// Replace any existing sslmode with require
		if strings.Contains(dbURL, "sslmode=") {
			dbURL = strings.ReplaceAll(dbURL, "sslmode=disable", "sslmode=require")
			dbURL = strings.ReplaceAll(dbURL, "sslmode=prefer", "sslmode=require")
			dbURL = strings.ReplaceAll(dbURL, "sslmode=allow", "sslmode=require")
		} else {
			// Add sslmode=require
			if strings.Contains(dbURL, "?") {
				dbURL += "&sslmode=require"
			} else {
				dbURL += "?sslmode=require"
			}
		}
	}

	// Connect to database
	logger.Info("connecting to database")
	dbConnectStart := time.Now()
	
	pool, err := pgxpool.New(ctx, dbURL)
	dbConnectDuration := time.Since(dbConnectStart)
	if err != nil {
		logger.Error("failed to connect to database", 
			"error", err,
			"duration_sec", dbConnectDuration.Seconds(),
		)
		os.Exit(1)
	}
	defer pool.Close()

	// Test connection
	pingStart := time.Now()
	if err := pool.Ping(ctx); err != nil {
		pingDuration := time.Since(pingStart)
		logger.Error("failed to ping database", 
			"error", err,
			"ping_duration_sec", pingDuration.Seconds(),
		)
		os.Exit(1)
	}
	pingDuration := time.Since(pingStart)
	logger.Info("database connection established",
		"connect_duration_sec", dbConnectDuration.Seconds(),
		"ping_duration_sec", pingDuration.Seconds(),
	)

	// Never log the actual token
	if len(cfg.discordToken) > 10 {
		logger.Debug("discord token loaded", "token_prefix", cfg.discordToken[:6]+"...")
	} else {
		logger.Debug("discord token loaded")
	}

	// Create and start bot
	botCreateStart := time.Now()
	botInstance, err := bot.NewBot(cfg.discordToken, pool, logger)
	botCreateDuration := time.Since(botCreateStart)
	if err != nil {
		logger.Error("failed to create bot", 
			"error", err,
			"duration_sec", botCreateDuration.Seconds(),
		)
		os.Exit(1)
	}
	logger.Info("bot instance created",
		"duration_sec", botCreateDuration.Seconds(),
	)

	botStartStart := time.Now()
	if err := botInstance.Start(); err != nil {
		botStartDuration := time.Since(botStartStart)
		logger.Error("failed to start bot", 
			"error", err,
			"duration_sec", botStartDuration.Seconds(),
		)
		os.Exit(1)
	}
	botStartDuration := time.Since(botStartStart)
	
	totalStartupDuration := time.Since(startTime)
	logger.Info("bot startup completed",
		"bot_start_duration_sec", botStartDuration.Seconds(),
		"total_startup_duration_sec", totalStartupDuration.Seconds(),
	)

	// Start metrics server
	
	metricsCtx, metricsCancel := context.WithCancel(context.Background())
	go func() {
		logger.Info("starting metrics server", "port", cfg.metricsPort)
		if err := bot.StartMetricsServer(metricsCtx, cfg.metricsPort, logger, botInstance); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "error", err)
		}
	}()

	// Wait for interrupt signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Shutdown gracefully
	logger.Info("shutting down gracefully")
	metricsCancel()
	botInstance.Stop()
	logger.Info("shutdown complete")
}
