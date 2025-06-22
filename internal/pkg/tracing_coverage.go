package pkg

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// CoverageTracker tracks tracing coverage for requests
type CoverageTracker struct {
	mu                 sync.RWMutex
	requestStartTimes  map[string]time.Time
	totalDurations     map[string]time.Duration
	tracedDurations    map[string]time.Duration
	activeSpans        map[string]map[trace.SpanID]time.Time
}

var globalCoverageTracker = &CoverageTracker{
	requestStartTimes: make(map[string]time.Time),
	totalDurations:    make(map[string]time.Duration),
	tracedDurations:   make(map[string]time.Duration),
	activeSpans:       make(map[string]map[trace.SpanID]time.Time),
}

// StartRequestTracking begins tracking coverage for a request
func StartRequestTracking(requestID string) {
	globalCoverageTracker.mu.Lock()
	defer globalCoverageTracker.mu.Unlock()
	
	globalCoverageTracker.requestStartTimes[requestID] = time.Now()
	globalCoverageTracker.activeSpans[requestID] = make(map[trace.SpanID]time.Time)
}

// EndRequestTracking ends tracking and calculates coverage
func EndRequestTracking(requestID string) (totalDuration time.Duration, tracedDuration time.Duration, coverage float64) {
	globalCoverageTracker.mu.Lock()
	defer globalCoverageTracker.mu.Unlock()
	
	startTime, exists := globalCoverageTracker.requestStartTimes[requestID]
	if !exists {
		return 0, 0, 0
	}
	
	totalDuration = time.Since(startTime)
	tracedDuration = globalCoverageTracker.tracedDurations[requestID]
	
	if totalDuration > 0 {
		coverage = float64(tracedDuration) / float64(totalDuration) * 100
	}
	
	// Cleanup
	delete(globalCoverageTracker.requestStartTimes, requestID)
	delete(globalCoverageTracker.totalDurations, requestID)
	delete(globalCoverageTracker.tracedDurations, requestID)
	delete(globalCoverageTracker.activeSpans, requestID)
	
	return totalDuration, tracedDuration, coverage
}

// TrackSpan tracks a span for coverage calculation
func TrackSpan(ctx context.Context, requestID string) func() {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return func() {} // noop
	}
	
	spanID := span.SpanContext().SpanID()
	startTime := time.Now()
	
	globalCoverageTracker.mu.Lock()
	if _, exists := globalCoverageTracker.activeSpans[requestID]; exists {
		globalCoverageTracker.activeSpans[requestID][spanID] = startTime
	}
	globalCoverageTracker.mu.Unlock()
	
	return func() {
		duration := time.Since(startTime)
		
		globalCoverageTracker.mu.Lock()
		defer globalCoverageTracker.mu.Unlock()
		
		// Remove from active spans
		if activeSpans, exists := globalCoverageTracker.activeSpans[requestID]; exists {
			delete(activeSpans, spanID)
		}
		
		// Add to traced duration
		globalCoverageTracker.tracedDurations[requestID] += duration
	}
}

// Enhanced StartSpan that includes coverage tracking
func StartSpanWithCoverage(ctx context.Context, requestID string, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span, func()) {
	ctx, span := StartSpan(ctx, name, attrs...)
	
	// Track this span for coverage
	endTracking := TrackSpan(ctx, requestID)
	
	return ctx, span, func() {
		endTracking()
		span.End()
	}
}

// GetGlobalCoverageStats returns aggregated coverage statistics
func GetGlobalCoverageStats() map[string]interface{} {
	globalCoverageTracker.mu.RLock()
	defer globalCoverageTracker.mu.RUnlock()
	
	stats := map[string]interface{}{
		"active_requests":     len(globalCoverageTracker.requestStartTimes),
		"total_spans_tracked": 0,
	}
	
	totalSpans := 0
	for _, spans := range globalCoverageTracker.activeSpans {
		totalSpans += len(spans)
	}
	stats["total_spans_tracked"] = totalSpans
	
	return stats
}

// LogCoverageForRequest logs coverage information for a completed request
func LogCoverageForRequest(requestID string, operation string) {
	totalDuration, tracedDuration, coverage := EndRequestTracking(requestID)
	
	EnsureInitialized()
	
	// Record coverage metrics
	if CommandCounter != nil {
		CommandCounter.Add(context.Background(), 1,
			metric.WithAttributes(
				attribute.String("operation", operation),
				attribute.Float64("coverage_percent", coverage),
				attribute.Int64("total_duration_ms", totalDuration.Milliseconds()),
				attribute.Int64("traced_duration_ms", tracedDuration.Milliseconds()),
			),
		)
	}
	
	// Log if coverage is low
	if coverage < 80.0 && totalDuration > 100*time.Millisecond {
		// This indicates we might be missing important instrumentation
		AddSpanEvent(context.Background(), "low_tracing_coverage",
			attribute.String("request_id", requestID),
			attribute.String("operation", operation),
			attribute.Float64("coverage_percent", coverage),
			attribute.Int64("total_ms", totalDuration.Milliseconds()),
			attribute.Int64("traced_ms", tracedDuration.Milliseconds()),
			attribute.Int64("untraced_ms", (totalDuration-tracedDuration).Milliseconds()),
		)
	}
}