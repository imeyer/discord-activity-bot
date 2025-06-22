package pkg

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a sliding window rate limiter
type RateLimiter struct {
	mu          sync.Mutex
	userLimits  map[string]*userLimit
	guildLimits map[string]*guildLimit
	config      RateLimitConfig
	ctx         context.Context
	cancel      context.CancelFunc
	maxEntries  int
}

type RateLimitConfig struct {
	UserPerMinute  int
	GuildPerMinute int
	WindowSize     time.Duration
	CleanupPeriod  time.Duration
}

type userLimit struct {
	timestamps []time.Time
	lastClean  time.Time
}

type guildLimit struct {
	timestamps []time.Time
	lastClean  time.Time
}

func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	if config.WindowSize == 0 {
		config.WindowSize = time.Minute
	}
	if config.CleanupPeriod == 0 {
		config.CleanupPeriod = 5 * time.Minute
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	rl := &RateLimiter{
		userLimits:  make(map[string]*userLimit),
		guildLimits: make(map[string]*guildLimit),
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		maxEntries:  10000, // Prevent unbounded growth
	}
	
	// Start cleanup goroutine with context
	go rl.cleanupLoop()
	
	return rl
}

// Stop gracefully shuts down the rate limiter
func (rl *RateLimiter) Stop() {
	rl.cancel()
}

// CheckUser returns true if the user is within rate limits
func (rl *RateLimiter) CheckUser(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Prevent memory exhaustion
	if len(rl.userLimits) >= rl.maxEntries {
		// Evict oldest entry
		rl.evictOldestUser()
	}
	
	now := time.Now()
	limit, exists := rl.userLimits[userID]
	if !exists {
		limit = &userLimit{
			timestamps: make([]time.Time, 0, rl.config.UserPerMinute),
			lastClean:  now,
		}
		rl.userLimits[userID] = limit
	}
	
	// Remove old timestamps
	cutoff := now.Add(-rl.config.WindowSize)
	validTimestamps := make([]time.Time, 0, len(limit.timestamps))
	for _, ts := range limit.timestamps {
		if ts.After(cutoff) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	limit.timestamps = validTimestamps
	
	// Check if within limit
	if len(limit.timestamps) >= rl.config.UserPerMinute {
		return false
	}
	
	// Add current timestamp
	limit.timestamps = append(limit.timestamps, now)
	return true
}

// CheckGuild returns true if the guild is within rate limits
func (rl *RateLimiter) CheckGuild(guildID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	limit, exists := rl.guildLimits[guildID]
	if !exists {
		limit = &guildLimit{
			timestamps: make([]time.Time, 0, rl.config.GuildPerMinute),
			lastClean:  now,
		}
		rl.guildLimits[guildID] = limit
	}
	
	// Remove old timestamps
	cutoff := now.Add(-rl.config.WindowSize)
	validTimestamps := make([]time.Time, 0, len(limit.timestamps))
	for _, ts := range limit.timestamps {
		if ts.After(cutoff) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	limit.timestamps = validTimestamps
	
	// Check if within limit
	if len(limit.timestamps) >= rl.config.GuildPerMinute {
		return false
	}
	
	// Add current timestamp
	limit.timestamps = append(limit.timestamps, now)
	return true
}

// cleanupLoop periodically removes inactive users/guilds from memory
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.config.CleanupPeriod)
	defer ticker.Stop()
	
	for {
		select {
		case <-rl.ctx.Done():
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	inactiveThreshold := now.Add(-rl.config.CleanupPeriod)
	
	// Cleanup user limits
	for userID, limit := range rl.userLimits {
		if len(limit.timestamps) == 0 && limit.lastClean.Before(inactiveThreshold) {
			delete(rl.userLimits, userID)
		}
	}
	
	// Cleanup guild limits
	for guildID, limit := range rl.guildLimits {
		if len(limit.timestamps) == 0 && limit.lastClean.Before(inactiveThreshold) {
			delete(rl.guildLimits, guildID)
		}
	}
}

// evictOldestUser removes the user with the oldest activity
func (rl *RateLimiter) evictOldestUser() {
	var oldestUser string
	var oldestTime time.Time
	
	for userID, limit := range rl.userLimits {
		if oldestUser == "" || limit.lastClean.Before(oldestTime) {
			oldestUser = userID
			oldestTime = limit.lastClean
		}
	}
	
	if oldestUser != "" {
		delete(rl.userLimits, oldestUser)
	}
}