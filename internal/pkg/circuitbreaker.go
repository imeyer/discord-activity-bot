package pkg

import (
	"fmt"
	"sync"
	"time"
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu              sync.RWMutex
	name            string
	maxFailures     uint32
	resetTimeout    time.Duration
	halfOpenMax     uint32
	
	failures        uint32
	lastFailureTime time.Time
	state           State
	halfOpenSuccess uint32
}

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures uint32, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		halfOpenMax:  maxFailures / 2,
		state:        StateClosed,
	}
}

// Call executes the function if the circuit breaker allows it
func (cb *CircuitBreaker) Call(fn func() error) error {
	if !cb.CanCall() {
		return fmt.Errorf("circuit breaker %s is open", cb.name)
	}
	
	err := fn()
	cb.RecordResult(err)
	return err
}

// CanCall returns true if the circuit breaker allows calls
func (cb *CircuitBreaker) CanCall() bool {
	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()
	
	if state == StateClosed {
		return true
	}
	
	if state == StateOpen {
		// Check if we should transition to half-open
		cb.mu.Lock()
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenSuccess = 0
			cb.mu.Unlock()
			return true
		}
		cb.mu.Unlock()
		return false
	}
	
	// Half-open state - allow limited calls
	return true
}

// RecordResult records the result of a call
func (cb *CircuitBreaker) RecordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

func (cb *CircuitBreaker) recordFailure() {
	cb.failures++
	cb.lastFailureTime = time.Now()
	
	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.maxFailures {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		// Single failure in half-open state opens the circuit
		cb.state = StateOpen
		cb.failures = cb.maxFailures
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failures = 0
	case StateHalfOpen:
		cb.halfOpenSuccess++
		if cb.halfOpenSuccess >= cb.halfOpenMax {
			// Enough successes, close the circuit
			cb.state = StateClosed
			cb.failures = 0
			cb.halfOpenSuccess = 0
		}
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetMetrics returns current metrics
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return map[string]interface{}{
		"name":             cb.name,
		"state":            cb.state.String(),
		"failures":         cb.failures,
		"half_open_success": cb.halfOpenSuccess,
		"last_failure":     cb.lastFailureTime.Format(time.RFC3339),
	}
}