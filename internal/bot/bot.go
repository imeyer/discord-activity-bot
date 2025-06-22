package bot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/imeyer/discord-activity-bot/db"
	"github.com/imeyer/discord-activity-bot/internal/pkg"
	"github.com/imeyer/discord-activity-bot/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Bot struct {
	discord      *discordgo.Session
	discordAPI   *TracedDiscordAPI  // Traced Discord API wrapper
	pool         *pgxpool.Pool
	queries      db.Querier
	msgBuffer    []db.InsertMessageParams
	bufferLock   sync.Mutex
	flushTimer   *time.Timer
	logger       *slog.Logger
	messageCount uint64 // Total messages received
	startTime    time.Time
	rateLimiter  *pkg.RateLimiter
	
	// Goroutine tracking
	wg           sync.WaitGroup
	shutdownCh   chan struct{}
	maxGoroutines int
	activeInserts int32
	
	// Circuit breaker for database operations
	dbCircuitBreaker *pkg.CircuitBreaker
	
	// Admin role management
	adminRoles *AdminRoleManager
	
	// Cached user service
	userService *users.UserService
	
	// Command rate limiter
	commandRateLimiter *pkg.CommandRateLimiter
}

const (
	batchSize     = 100
	flushInterval = 5 * time.Second
)

func NewBot(discordToken string, dbPool *pgxpool.Pool, logger *slog.Logger) (*Bot, error) {
	logger.Debug("creating discord session")
	
	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Set discord logging
	dg.LogLevel = discordgo.LogWarning

	// Configure rate limiting
	rateLimitConfig := pkg.RateLimitConfig{
		UserPerMinute:  60,  // 1 message per second per user
		GuildPerMinute: 600, // 10 messages per second per guild
		WindowSize:     time.Minute,
		CleanupPeriod:  5 * time.Minute,
	}
	
	queries := db.NewTracedQuerier(db.New(dbPool))
	
	bot := &Bot{
		discord:          dg,
		discordAPI:       NewTracedDiscordAPI(dg),
		pool:             dbPool,
		queries:          queries,
		msgBuffer:        make([]db.InsertMessageParams, 0, batchSize),
		logger:           logger,
		startTime:        time.Now(),
		rateLimiter:      pkg.NewRateLimiter(rateLimitConfig),
		shutdownCh:       make(chan struct{}),
		maxGoroutines:    10, // Limit concurrent DB operations
		dbCircuitBreaker:   pkg.NewCircuitBreaker("database", 5, 30*time.Second),
		userService:        users.NewUserService(dg, queries, logger),
		commandRateLimiter: pkg.NewCommandRateLimiter(5 * time.Second), // 5 second cooldown per command
	}

	// Add handlers with panic recovery
	dg.AddHandler(bot.withMessageRecovery(bot.messageCreate))
	dg.AddHandler(bot.withRecovery(bot.handleCommands))
	
	logger.Info("bot created successfully", 
		"batch_size", batchSize,
		"flush_interval", flushInterval,
	)

	return bot, nil
}

func (b *Bot) Start() error {
	startTime := time.Now()
	
	// Discord connection
	b.logger.Info("opening discord websocket connection")
	discordConnectStart := time.Now()
	
	err := b.discord.Open()
	discordConnectDuration := time.Since(discordConnectStart)
	if err != nil {
		return fmt.Errorf("failed to open Discord session after %v: %w", discordConnectDuration, err)
	}

	b.logger.Info("discord connection established",
		"user_id", b.discord.State.User.ID,
		"username", b.discord.State.User.Username,
		"connection_duration_sec", discordConnectDuration.Seconds(),
	)

	// Register slash commands
	cmdRegisterStart := time.Now()
	b.registerCommands()
	cmdRegisterDuration := time.Since(cmdRegisterStart)
	b.logger.Info("slash commands registered",
		"duration_sec", cmdRegisterDuration.Seconds(),
	)
	
	// Setup admin commands
	adminSetupStart := time.Now()
	b.setupAdminCommands()
	adminSetupDuration := time.Since(adminSetupStart)
	b.logger.Info("admin commands setup completed",
		"duration_sec", adminSetupDuration.Seconds(),
	)
	
	// Register admin commands
	adminRegisterStart := time.Now()
	b.registerAdminCommands()
	adminRegisterDuration := time.Since(adminRegisterStart)
	b.logger.Info("admin commands registered",
		"duration_sec", adminRegisterDuration.Seconds(),
	)

	// Start flush timer
	flushTimerStart := time.Now()
	b.startFlushTimer()
	flushTimerDuration := time.Since(flushTimerStart)
	b.logger.Debug("flush timer started",
		"duration_sec", flushTimerDuration.Seconds(),
	)

	// Start background user refresh job
	userRefreshStart := time.Now()
	b.startUserRefreshJob()
	userRefreshDuration := time.Since(userRefreshStart)
	b.logger.Info("user refresh job started",
		"duration_sec", userRefreshDuration.Seconds(),
	)
	
	// Start command rate limiter cleanup
	rateLimitStart := time.Now()
	b.commandRateLimiter.StartCleanupRoutine()
	rateLimitDuration := time.Since(rateLimitStart)
	b.logger.Debug("command rate limiter started",
		"duration_sec", rateLimitDuration.Seconds(),
	)

	totalDuration := time.Since(startTime)
	b.logger.Info("bot is ready to receive messages",
		"guilds", len(b.discord.State.Guilds),
		"total_start_duration_sec", totalDuration.Seconds(),
		"breakdown", map[string]float64{
			"discord_connect_sec": discordConnectDuration.Seconds(),
			"cmd_register_sec": cmdRegisterDuration.Seconds(),
			"admin_setup_sec": adminSetupDuration.Seconds(),
			"admin_register_sec": adminRegisterDuration.Seconds(),
			"flush_timer_sec": flushTimerDuration.Seconds(),
			"user_refresh_sec": userRefreshDuration.Seconds(),
		},
	)
	
	return nil
}

func (b *Bot) startUserRefreshJob() {
	// Run user refresh every 30 minutes
	ticker := time.NewTicker(30 * time.Minute)
	
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer ticker.Stop()
		
		// Initial cleanup
		b.cleanupUserCache()
		
		for {
			select {
			case <-ticker.C:
				b.refreshStaleUsers()
				b.cleanupUserCache()
			case <-b.shutdownCh:
				return
			}
		}
	}()
}

func (b *Bot) refreshStaleUsers() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	// Refresh up to 100 stale users per cycle
	err := b.userService.RefreshStaleUsers(ctx, 100)
	if err != nil {
		b.logger.Error("failed to refresh stale users", "error", err)
		return
	}
	
	// Log cache statistics
	stats := b.userService.GetCacheStats()
	b.logger.Debug("user cache stats", "stats", stats)
}

func (b *Bot) cleanupUserCache() {
	cleaned := b.userService.CleanupExpiredEntries()
	if cleaned > 0 {
		b.logger.Debug("cleaned up expired user cache entries", "count", cleaned)
	}
}

func (b *Bot) Stop() {
	b.logger.Info("stopping bot")
	
	// Signal shutdown
	close(b.shutdownCh)
	
	// Flush remaining messages
	b.logger.Debug("flushing remaining messages", "count", len(b.msgBuffer))
	b.flush()
	
	// Stop timer
	if b.flushTimer != nil {
		b.flushTimer.Stop()
		b.logger.Debug("flush timer stopped")
	}
	
	// Stop rate limiter
	b.rateLimiter.Stop()
	
	// Close Discord session
	b.discord.Close()
	b.logger.Info("discord session closed")
	
	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		b.logger.Info("all background tasks completed")
	case <-time.After(30 * time.Second):
		b.logger.Error("timeout waiting for background tasks to complete")
	}
	
	// Log final circuit breaker state
	b.logger.Info("circuit breaker final state", 
		"metrics", b.dbCircuitBreaker.GetMetrics(),
	)
}

func (b *Bot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all bot messages (including our own bot and other bots)
	if m.Author.Bot {
		return
	}

	// Increment received counter
	atomic.AddUint64(&pkg.Metrics.MessagesReceived, 1)
	
	// Apply rate limiting
	if !b.rateLimiter.CheckGuild(m.GuildID) {
		b.logger.Warn("guild rate limit exceeded",
			"guild_id", m.GuildID,
		)
		atomic.AddUint64(&pkg.Metrics.MessagesRateLimited, 1)
		return
	}
	
	if !b.rateLimiter.CheckUser(m.Author.ID) {
		b.logger.Debug("user rate limit exceeded",
			"user_id", m.Author.ID,
			"username", m.Author.Username,
		)
		atomic.AddUint64(&pkg.Metrics.MessagesRateLimited, 1)
		return
	}

	b.logger.Debug("message received",
		"user_id", m.Author.ID,
		"username", m.Author.Username,
		"channel_id", m.ChannelID,
		"server_id", m.GuildID,
		"message_id", m.ID,
	)

	// Validate IDs before processing
	if err := ValidateMessageParams(m.Author.ID, m.ChannelID, m.GuildID, m.ID); err != nil {
		b.logger.Error("invalid message parameters",
			"error", err,
			"user_id", m.Author.ID,
			"channel_id", m.ChannelID,
			"server_id", m.GuildID,
			"message_id", m.ID,
		)
		return
	}
	
	messageTime := time.Now()
	params := db.InsertMessageParams{
		Time:      messageTime,
		UserID:    m.Author.ID,
		ChannelID: m.ChannelID,
		ServerID:  m.GuildID,
		MessageID: m.ID,
	}

	// Record user activity (non-blocking)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := b.userService.RecordUserActivity(ctx, m.Author.ID, messageTime); err != nil {
			b.logger.Debug("failed to record user activity",
				"user_id", m.Author.ID,
				"error", err,
			)
		}
	}()

	// Increment total message count
	atomic.AddUint64(&b.messageCount, 1)
	
	b.bufferLock.Lock()
	b.msgBuffer = append(b.msgBuffer, params)
	bufferLen := len(b.msgBuffer)
	
	// Flush if buffer is full
	if bufferLen >= batchSize {
		b.logger.Debug("buffer full, triggering flush", "buffer_size", bufferLen)
		b.flushLocked()
	}
	b.bufferLock.Unlock()
}

func (b *Bot) startFlushTimer() {
	b.flushTimer = time.AfterFunc(flushInterval, func() {
		b.flush()
		b.startFlushTimer()
	})
	b.logger.Debug("flush timer started", "interval", flushInterval)
}

func (b *Bot) flush() {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()
	b.flushLocked()
}

func (b *Bot) flushLocked() {
	if len(b.msgBuffer) == 0 {
		return
	}

	// Copy buffer and reset
	messages := make([]db.InsertMessageParams, len(b.msgBuffer))
	copy(messages, b.msgBuffer)
	messageCount := len(messages)
	b.msgBuffer = b.msgBuffer[:0]

	b.logger.Debug("flushing message batch", "count", messageCount)

	// Check if we're shutting down
	select {
	case <-b.shutdownCh:
		b.logger.Warn("shutting down, dropping batch", "count", messageCount)
		atomic.AddUint64(&pkg.Metrics.MessagesDropped, uint64(messageCount))
		return
	default:
	}
	
	// Check circuit breaker
	if !b.dbCircuitBreaker.CanCall() {
		b.logger.Warn("circuit breaker open, dropping batch",
			"count", messageCount,
			"state", b.dbCircuitBreaker.GetState().String(),
		)
		atomic.AddUint64(&pkg.Metrics.MessagesDropped, uint64(messageCount))
		return
	}
	
	// Check goroutine limit
	currentInserts := atomic.LoadInt32(&b.activeInserts)
	if currentInserts >= int32(b.maxGoroutines) {
		b.logger.Error("too many active insert operations, dropping batch",
			"active", currentInserts,
			"max", b.maxGoroutines,
			"dropped_messages", messageCount,
		)
		atomic.AddUint64(&pkg.Metrics.MessagesDropped, uint64(messageCount))
		return
	}
	
	// Insert in background with tracking
	atomic.AddInt32(&b.activeInserts, 1)
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer atomic.AddInt32(&b.activeInserts, -1)
		b.insertBatch(messages)
	}()
}

func (b *Bot) insertBatch(messages []db.InsertMessageParams) {
	// Use circuit breaker for the entire batch operation
	err := b.dbCircuitBreaker.Call(func() error {
		return b.insertBatchInternal(messages)
	})
	
	if err != nil {
		b.logger.Error("batch insert failed with circuit breaker",
			"error", err,
			"circuit_state", b.dbCircuitBreaker.GetState().String(),
		)
	}
}

func (b *Bot) insertBatchInternal(messages []db.InsertMessageParams) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	b.logger.Debug("starting database transaction", "message_count", len(messages))

	tx, err := b.pool.Begin(ctx)
	if err != nil {
		b.logger.Error("failed to begin transaction", 
			"error", err,
			"message_count", len(messages),
		)
		atomic.AddUint64(&pkg.Metrics.DBConnectionErrors, 1)
		return err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err.Error() != "tx is closed" {
			b.logger.Error("failed to rollback transaction", "error", err)
		}
	}()

	// Cast to TracedQuerier to access WithTx method
	var qtx db.Querier
	if tracedQuerier, ok := b.queries.(*db.TracedQuerier); ok {
		qtx = tracedQuerier.WithTx(tx)
	} else {
		// Fallback for non-traced querier
		qtx = db.New(tx)
	}
	
	insertedCount := 0
	errorCount := 0
	
	for i, msg := range messages {
		if err := qtx.InsertMessage(ctx, msg); err != nil {
			errorCount++
			// Log constraint violations differently
			if err.Error() == "ERROR: there is no unique or exclusion constraint matching the ON CONFLICT specification (SQLSTATE 42P10)" {
				b.logger.Error("database constraint error - migrations may need to be run",
					"error", err,
					"message_id", msg.MessageID,
				)
				atomic.AddUint64(&pkg.Metrics.DBInsertErrors, 1)
			} else {
				b.logger.Warn("failed to insert message",
					"error", err,
					"message_id", msg.MessageID,
					"user_id", msg.UserID,
					"server_id", msg.ServerID,
					"batch_index", i,
				)
				atomic.AddUint64(&pkg.Metrics.DBInsertErrors, 1)
			}
		} else {
			insertedCount++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		b.logger.Error("failed to commit transaction",
			"error", err,
			"attempted_inserts", len(messages),
			"successful_inserts", insertedCount,
			"failed_inserts", errorCount,
		)
		return err
	}

	duration := time.Since(start)
	
	// Calculate a meaningful rate - avoid division by very small numbers
	var insertRate float64
	if duration.Seconds() > 0.001 { // More than 1ms
		insertRate = float64(insertedCount) / duration.Seconds()
	} else {
		insertRate = float64(insertedCount) * 1000 // If under 1ms, show as per millisecond * 1000
	}
	
	// Update metrics
	atomic.StoreInt64(&pkg.Metrics.LastBatchDuration, duration.Milliseconds())
	atomic.AddUint64(&pkg.Metrics.DBInsertSuccess, uint64(insertedCount))
	atomic.AddUint64(&pkg.Metrics.MessagesProcessed, uint64(insertedCount))
	atomic.StoreInt32(&pkg.Metrics.ActiveGoroutines, atomic.LoadInt32(&b.activeInserts))
	
	// Calculate overall message receive rate
	uptime := time.Since(b.startTime)
	totalMessages := atomic.LoadUint64(&b.messageCount)
	overallRate := float64(totalMessages) / uptime.Seconds()
	
	b.logger.Info("batch insert completed",
		"duration_ms", duration.Milliseconds(),
		"batch_size", len(messages),
		"inserted", insertedCount,
		"errors", errorCount,
		"db_insert_rate", fmt.Sprintf("%.0f/sec", insertRate),
		"overall_msg_rate", fmt.Sprintf("%.2f/sec", overallRate),
		"total_messages", totalMessages,
		"uptime_minutes", fmt.Sprintf("%.1f", uptime.Minutes()),
	)
	
	return nil
}
