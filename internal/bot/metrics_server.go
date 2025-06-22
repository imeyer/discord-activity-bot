package bot

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/imeyer/discord-activity-bot/internal/pkg"
)

// StartMetricsServer starts the HTTP metrics endpoint
func StartMetricsServer(ctx context.Context, port string, logger *slog.Logger, bot *Bot) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", pkg.MetricsHandler)
	mux.HandleFunc("/health", pkg.HealthHandler)
	
	// Only bind to localhost if not explicitly configured
	bindAddr := "127.0.0.1"
	if os.Getenv("METRICS_BIND_ALL") == "true" {
		bindAddr = ""
		logger.Warn("Metrics server binding to all interfaces - ensure firewall rules are in place")
	}
	
	server := &http.Server{
		Addr:         bindAddr + ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	
	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()
	
	return server.ListenAndServe()
}