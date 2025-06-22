package pkg

import (
	"encoding/json"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

// MetricsData holds application metrics
type MetricsData struct {
	// Message Metrics
	MessagesReceived     uint64
	MessagesProcessed    uint64
	MessagesDropped      uint64
	MessagesRateLimited  uint64
	
	// Database Metrics
	DBInsertSuccess      uint64
	DBInsertErrors       uint64
	DBConnectionErrors   uint64
	
	// Performance Metrics
	LastBatchDuration    int64 // milliseconds
	AverageBatchDuration int64 // milliseconds
	ActiveGoroutines     int32
	
	// Command Metrics
	CommandsExecuted     uint64
	CommandsRateLimited  uint64
	
	// System info
	StartTime            time.Time
	Version              string
}

// Global Metrics instance
var Metrics = &MetricsData{
	StartTime: time.Now(),
	Version:   "1.0.0-rc0",
}

// MetricsHandler serves Metrics in JSON format
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Simple bearer token authentication
	authHeader := r.Header.Get("Authorization")
	expectedToken := os.Getenv("METRICS_AUTH_TOKEN")
	
	// If token is set, require authentication
	if expectedToken != "" {
		if authHeader != "Bearer "+expectedToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}
	
	// Calculate uptime
	uptime := time.Since(Metrics.StartTime)
	
	// Build response
	response := map[string]interface{}{
		"version":               Metrics.Version,
		"uptime_seconds":        uptime.Seconds(),
		"messages": map[string]uint64{
			"received":      atomic.LoadUint64(&Metrics.MessagesReceived),
			"processed":     atomic.LoadUint64(&Metrics.MessagesProcessed),
			"dropped":       atomic.LoadUint64(&Metrics.MessagesDropped),
			"rate_limited":  atomic.LoadUint64(&Metrics.MessagesRateLimited),
		},
		"database": map[string]uint64{
			"insert_success":     atomic.LoadUint64(&Metrics.DBInsertSuccess),
			"insert_errors":      atomic.LoadUint64(&Metrics.DBInsertErrors),
			"connection_errors":  atomic.LoadUint64(&Metrics.DBConnectionErrors),
		},
		"performance": map[string]interface{}{
			"last_batch_duration_ms":    atomic.LoadInt64(&Metrics.LastBatchDuration),
			"average_batch_duration_ms": atomic.LoadInt64(&Metrics.AverageBatchDuration),
			"active_goroutines":         atomic.LoadInt32(&Metrics.ActiveGoroutines),
		},
		"commands": map[string]uint64{
			"executed":      atomic.LoadUint64(&Metrics.CommandsExecuted),
			"rate_limited":  atomic.LoadUint64(&Metrics.CommandsRateLimited),
		},
		"rates": map[string]float64{
			"messages_per_second": float64(atomic.LoadUint64(&Metrics.MessagesReceived)) / uptime.Seconds(),
			"success_rate":        calculateSuccessRate(),
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func calculateSuccessRate() float64 {
	processed := atomic.LoadUint64(&Metrics.MessagesProcessed)
	dropped := atomic.LoadUint64(&Metrics.MessagesDropped)
	total := processed + dropped
	
	if total == 0 {
		return 100.0
	}
	
	return float64(processed) / float64(total) * 100.0
}


// HealthHandler provides a simple health check endpoint
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Simple health check - could be enhanced to check DB connectivity
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}