package pkg

import (
	"sync"
	"time"
)

// CommandRateLimiter implements rate limiting for Discord slash commands
type CommandRateLimiter struct {
	mu           sync.Mutex
	lastUsed     map[string]time.Time // key: "userID:commandName"
	cooldownTime time.Duration
}

// NewCommandRateLimiter creates a new command rate limiter
func NewCommandRateLimiter(cooldown time.Duration) *CommandRateLimiter {
	return &CommandRateLimiter{
		lastUsed:     make(map[string]time.Time),
		cooldownTime: cooldown,
	}
}

// CanExecute checks if a user can execute a command
func (crl *CommandRateLimiter) CanExecute(userID, commandName string) bool {
	crl.mu.Lock()
	defer crl.mu.Unlock()
	
	key := userID + ":" + commandName
	lastUsed, exists := crl.lastUsed[key]
	
	if !exists {
		crl.lastUsed[key] = time.Now()
		return true
	}
	
	if time.Since(lastUsed) >= crl.cooldownTime {
		crl.lastUsed[key] = time.Now()
		return true
	}
	
	return false
}

// GetTimeRemaining returns how much time is left before the command can be used again
func (crl *CommandRateLimiter) GetTimeRemaining(userID, commandName string) time.Duration {
	crl.mu.Lock()
	defer crl.mu.Unlock()
	
	key := userID + ":" + commandName
	lastUsed, exists := crl.lastUsed[key]
	
	if !exists {
		return 0
	}
	
	elapsed := time.Since(lastUsed)
	if elapsed >= crl.cooldownTime {
		return 0
	}
	
	return crl.cooldownTime - elapsed
}

// Cleanup removes old entries to prevent memory leaks
func (crl *CommandRateLimiter) Cleanup() {
	crl.mu.Lock()
	defer crl.mu.Unlock()
	
	cutoff := time.Now().Add(-crl.cooldownTime * 2) // Keep entries for 2x cooldown time
	
	for key, lastUsed := range crl.lastUsed {
		if lastUsed.Before(cutoff) {
			delete(crl.lastUsed, key)
		}
	}
}

// StartCleanupRoutine starts a background routine to clean up old entries
func (crl *CommandRateLimiter) StartCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute) // Cleanup every 10 minutes
		defer ticker.Stop()
		
		for range ticker.C {
			crl.Cleanup()
		}
	}()
}