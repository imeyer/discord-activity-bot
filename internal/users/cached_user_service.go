package users

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/imeyer/discord-activity-bot/db"
	"github.com/imeyer/discord-activity-bot/internal/pkg"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"
)

// CachedUser represents a user with caching metadata
type CachedUser struct {
	User       *db.User
	CachedAt   time.Time
	ExpireTime time.Time
}

// UserService provides cached user lookups with username change tracking
type UserService struct {
	discordSession *discordgo.Session
	queries        db.Querier
	logger         *slog.Logger
	
	// In-memory cache
	cache     map[string]*CachedUser
	cacheMu   sync.RWMutex
	cacheSize int
	
	// Configuration
	cacheExpiry    time.Duration
	maxCacheSize   int
	refreshWorkers int
}

// NewUserService creates a new cached user service
func NewUserService(discordSession *discordgo.Session, queries db.Querier, logger *slog.Logger) *UserService {
	return &UserService{
		discordSession: discordSession,
		queries:        queries,
		logger:         logger,
		cache:          make(map[string]*CachedUser),
		cacheExpiry:    1 * time.Hour,  // Cache users for 1 hour
		maxCacheSize:   10000,          // Maximum 10k users in memory
		refreshWorkers: 5,              // 5 concurrent refresh workers
	}
}

// GetUser retrieves a user with caching and automatic refresh
func (us *UserService) GetUser(ctx context.Context, userID string) (*db.User, error) {
	ctx, span := pkg.StartSpan(ctx, "user_service.get_user",
		attribute.String("user_id", userID),
	)
	defer span.End()

	// Check cache first
	us.cacheMu.RLock()
	cached, exists := us.cache[userID]
	us.cacheMu.RUnlock()

	if exists && time.Now().Before(cached.ExpireTime) {
		pkg.AddSpanAttributes(ctx, attribute.Bool("cache_hit", true))
		return cached.User, nil
	}

	pkg.AddSpanAttributes(ctx, attribute.Bool("cache_hit", false))

	// Cache miss or expired - fetch from database first
	dbUser, err := us.queries.GetUser(ctx, userID)
	if err == nil && dbUser.LastUpdatedAt.After(time.Now().Add(-us.cacheExpiry)) {
		// Database has recent data, use it
		pkg.AddSpanAttributes(ctx, attribute.String("source", "database"))
		us.updateCache(userID, &dbUser)
		return &dbUser, nil
	}

	// Need to fetch from Discord API
	pkg.AddSpanAttributes(ctx, attribute.String("source", "discord_api"))
	return us.fetchAndUpdateUser(ctx, userID)
}

// GetUserByUsername searches for users by username (cached and database)
func (us *UserService) GetUserByUsername(ctx context.Context, username string) ([]db.User, error) {
	ctx, span := pkg.StartSpan(ctx, "user_service.get_user_by_username",
		attribute.String("username", username),
	)
	defer span.End()

	// For username searches, always go to database as cache doesn't index by username
	users, err := us.queries.GetUserByUsername(ctx, username)
	if err != nil {
		pkg.RecordError(ctx, err, "username_search_failed")
		return nil, fmt.Errorf("failed to search users by username: %w", err)
	}

	pkg.AddSpanAttributes(ctx, attribute.Int("results_count", len(users)))
	
	// Update cache with found users
	for i := range users {
		us.updateCache(users[i].UserID, &users[i])
	}

	return users, nil
}

// RecordUserActivity updates the last message time for a user
func (us *UserService) RecordUserActivity(ctx context.Context, userID string, messageTime time.Time) error {
	ctx, span := pkg.StartSpan(ctx, "user_service.record_activity",
		attribute.String("user_id", userID),
	)
	defer span.End()

	// Update database
	err := us.queries.UpdateUserLastMessage(ctx, db.UpdateUserLastMessageParams{
		UserID:        userID,
		LastMessageAt: pgtype.Timestamptz{Time: messageTime, Valid: true},
	})
	if err != nil {
		pkg.RecordError(ctx, err, "update_last_message_failed")
		return fmt.Errorf("failed to update user last message: %w", err)
	}

	// Update cache if present
	us.cacheMu.Lock()
	if cached, exists := us.cache[userID]; exists {
		cached.User.LastMessageAt = pgtype.Timestamptz{Time: messageTime, Valid: true}
		cached.User.LastUpdatedAt = time.Now()
	}
	us.cacheMu.Unlock()

	return nil
}

// GetUsernameHistory retrieves the username change history for a user
func (us *UserService) GetUsernameHistory(ctx context.Context, userID string, limit int32) ([]db.GetUsernameHistoryRow, error) {
	ctx, span := pkg.StartSpan(ctx, "user_service.get_username_history",
		attribute.String("user_id", userID),
		attribute.Int("limit", int(limit)),
	)
	defer span.End()

	history, err := us.queries.GetUsernameHistory(ctx, db.GetUsernameHistoryParams{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		pkg.RecordError(ctx, err, "get_username_history_failed")
		return nil, fmt.Errorf("failed to get username history: %w", err)
	}

	pkg.AddSpanAttributes(ctx, attribute.Int("history_count", len(history)))
	return history, nil
}

// GetRecentUsernameChanges retrieves recent username changes across all users
func (us *UserService) GetRecentUsernameChanges(ctx context.Context, limit int32) ([]db.GetRecentUsernameChangesRow, error) {
	ctx, span := pkg.StartSpan(ctx, "user_service.get_recent_changes",
		attribute.Int("limit", int(limit)),
	)
	defer span.End()

	changes, err := us.queries.GetRecentUsernameChanges(ctx, limit)
	if err != nil {
		pkg.RecordError(ctx, err, "get_recent_changes_failed")
		return nil, fmt.Errorf("failed to get recent username changes: %w", err)
	}

	pkg.AddSpanAttributes(ctx, attribute.Int("changes_count", len(changes)))
	return changes, nil
}

// fetchAndUpdateUser fetches user data from Discord and updates database/cache
func (us *UserService) fetchAndUpdateUser(ctx context.Context, userID string) (*db.User, error) {
	ctx, span := pkg.StartSpan(ctx, "user_service.fetch_and_update",
		attribute.String("user_id", userID),
	)
	defer span.End()

	start := time.Now()
	discordUser, err := us.discordSession.User(userID)
	if err != nil {
		pkg.RecordError(ctx, err, "discord_api_fetch_failed")
		return nil, fmt.Errorf("failed to fetch user from Discord: %w", err)
	}

	apiDuration := time.Since(start)
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("discord_api_duration_ms", apiDuration.Milliseconds()),
		attribute.String("username", discordUser.Username),
		attribute.Bool("is_bot", discordUser.Bot),
	)

	// Upsert user into database
	dbUser, err := us.queries.UpsertUser(ctx, db.UpsertUserParams{
		UserID:        discordUser.ID,
		Username:      discordUser.Username,
		Discriminator: pgtype.Text{String: discordUser.Discriminator, Valid: discordUser.Discriminator != ""},
		DisplayName:   pgtype.Text{String: discordUser.GlobalName, Valid: discordUser.GlobalName != ""},
		AvatarHash:    pgtype.Text{String: discordUser.Avatar, Valid: discordUser.Avatar != ""},
		Bot:           pgtype.Bool{Bool: discordUser.Bot, Valid: true},
		LastMessageAt: pgtype.Timestamptz{}, // Will be updated separately
	})
	if err != nil {
		pkg.RecordError(ctx, err, "database_upsert_failed")
		return nil, fmt.Errorf("failed to upsert user: %w", err)
	}

	// Update cache
	us.updateCache(userID, &dbUser)

	us.logger.Debug("fetched and cached user",
		"user_id", userID,
		"username", discordUser.Username,
		"discriminator", discordUser.Discriminator,
		"display_name", discordUser.GlobalName,
		"avatar_hash", discordUser.Avatar,
		"is_bot", discordUser.Bot,
		"api_duration_ms", apiDuration.Milliseconds(),
	)

	return &dbUser, nil
}

// updateCache safely updates the in-memory cache
func (us *UserService) updateCache(userID string, user *db.User) {
	us.cacheMu.Lock()
	defer us.cacheMu.Unlock()

	// Evict oldest entries if cache is full
	if len(us.cache) >= us.maxCacheSize {
		us.evictOldestEntry()
	}

	us.cache[userID] = &CachedUser{
		User:       user,
		CachedAt:   time.Now(),
		ExpireTime: time.Now().Add(us.cacheExpiry),
	}
	us.cacheSize = len(us.cache)
}

// evictOldestEntry removes the oldest entry from cache (called with lock held)
func (us *UserService) evictOldestEntry() {
	oldestTime := time.Now()
	oldestKey := ""

	for key, cached := range us.cache {
		if cached.CachedAt.Before(oldestTime) {
			oldestTime = cached.CachedAt
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(us.cache, oldestKey)
	}
}

// GetCacheStats returns cache statistics
func (us *UserService) GetCacheStats() map[string]interface{} {
	us.cacheMu.RLock()
	defer us.cacheMu.RUnlock()

	expired := 0
	now := time.Now()
	for _, cached := range us.cache {
		if now.After(cached.ExpireTime) {
			expired++
		}
	}

	return map[string]interface{}{
		"total_entries":   len(us.cache),
		"expired_entries": expired,
		"max_size":        us.maxCacheSize,
		"cache_expiry":    us.cacheExpiry.String(),
	}
}

// CleanupExpiredEntries removes expired entries from cache
func (us *UserService) CleanupExpiredEntries() int {
	us.cacheMu.Lock()
	defer us.cacheMu.Unlock()

	cleaned := 0
	now := time.Now()
	
	for key, cached := range us.cache {
		if now.After(cached.ExpireTime) {
			delete(us.cache, key)
			cleaned++
		}
	}

	us.cacheSize = len(us.cache)
	return cleaned
}

// RefreshStaleUsers refreshes users that haven't been updated recently
func (us *UserService) RefreshStaleUsers(ctx context.Context, limit int32) error {
	ctx, span := pkg.StartSpan(ctx, "user_service.refresh_stale_users",
		attribute.Int("limit", int(limit)),
	)
	defer span.End()

	staleUsers, err := us.queries.GetUsersNeedingRefresh(ctx, limit)
	if err != nil {
		pkg.RecordError(ctx, err, "get_stale_users_failed")
		return fmt.Errorf("failed to get stale users: %w", err)
	}

	pkg.AddSpanAttributes(ctx, attribute.Int("stale_users_count", len(staleUsers)))

	refreshed := 0
	for _, userID := range staleUsers {
		_, err := us.fetchAndUpdateUser(ctx, userID)
		if err != nil {
			us.logger.Warn("failed to refresh user",
				"user_id", userID,
				"error", err,
			)
			continue
		}
		refreshed++
	}

	pkg.AddSpanAttributes(ctx, attribute.Int("refreshed_count", refreshed))
	
	us.logger.Info("refreshed stale users",
		"total_stale", len(staleUsers),
		"refreshed", refreshed,
		"failed", len(staleUsers)-refreshed,
	)

	return nil
}