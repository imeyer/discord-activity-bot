package bot

import (
	"bytes"
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/imeyer/discord-activity-bot/db"
	"github.com/imeyer/discord-activity-bot/internal/charts"
	"github.com/imeyer/discord-activity-bot/internal/pkg"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	minOne = 1.0
)

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "chattiest",
		Description: "Show who sent the most messages",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "period",
				Description: "Time period",
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "Today", Value: "today"},
					{Name: "Yesterday", Value: "yesterday"},
					{Name: "This Week", Value: "week"},
					{Name: "This Month", Value: "month"},
				},
				Required: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Specific channel (optional)",
				Required:    false,
			},
		},
	},
	{
		Name:        "userstats",
		Description: "Show stats for a specific user",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "User to check",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "period",
				Description: "Time period",
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "Today", Value: "today"},
					{Name: "This Week", Value: "week"},
					{Name: "This Month", Value: "month"},
				},
				Required: false,
			},
		},
	},
	{
		Name:        "trending",
		Description: "Show users trending up or down in activity",
		Options: []*discordgo.ApplicationCommandOption{
		},
	},
	{
		Name:        "channel-activity",
		Description: "Show visual graph of channel activity today",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Channel to analyze (defaults to current)",
				Required:    false,
			},
		},
	},
	{
		Name:        "rising-stars",
		Description: "Show users with rapidly growing activity",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "limit",
				Description: "Number of users to show (default: 10)",
				Required:    false,
				MinValue:    &minOne,
				MaxValue:    25,
			},
		},
	},
	{
		Name:        "peak-hours",
		Description: "Show server activity heatmap by hour",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Specific channel (optional, defaults to whole server)",
				Required:    false,
			},
		},
	},
}

func (b *Bot) registerCommands() {
	startTime := time.Now()
	b.logger.Info("registering slash commands", "count", len(commands))
	
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	var totalRegisterDuration time.Duration
	successCount := 0
	
	for i, v := range commands {
		cmdStart := time.Now()
		cmd, err := b.discord.ApplicationCommandCreate(b.discord.State.User.ID, "", v)
		cmdDuration := time.Since(cmdStart)
		totalRegisterDuration += cmdDuration
		
		if err != nil {
			b.logger.Error("failed to create command",
				"command", v.Name,
				"error", err,
				"duration_sec", cmdDuration.Seconds(),
			)
		} else {
			successCount++
			b.logger.Debug("registered command", 
				"command", v.Name,
				"id", cmd.ID,
				"duration_sec", cmdDuration.Seconds(),
			)
		}
		registeredCommands[i] = cmd
	}
	
	totalDuration := time.Since(startTime)
	avgDuration := float64(totalRegisterDuration.Nanoseconds()) / float64(len(commands)) / 1e9
	
	b.logger.Info("slash command registration complete", 
		"total_commands", len(commands),
		"successful", successCount,
		"failed", len(commands)-successCount,
		"total_duration_sec", totalDuration.Seconds(),
		"avg_per_command_sec", avgDuration,
		"api_time_sec", totalRegisterDuration.Seconds(),
	)
}

func (b *Bot) handleCommands(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	cmdName := i.ApplicationCommandData().Name
	userID := i.Member.User.ID
	
	// Check rate limiting
	if !b.commandRateLimiter.CanExecute(userID, cmdName) {
		remaining := b.commandRateLimiter.GetTimeRemaining(userID, cmdName)
		
		// Update rate limit metrics
		atomic.AddUint64(&pkg.Metrics.CommandsRateLimited, 1)
		pkg.CommandRateLimitCounter.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("command", cmdName),
		))
		
		b.logger.Warn("command rate limited",
			"command", cmdName,
			"user_id", userID,
			"remaining_seconds", remaining.Seconds(),
		)
		
		// Send rate limit message
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("⏱️ Please wait %.0f more seconds before using `%s` again.", 
					remaining.Seconds(), cmdName),
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			b.logger.Error("failed to send rate limit message", "error", err)
		}
		return
	}
	
	// Update command execution metrics
	atomic.AddUint64(&pkg.Metrics.CommandsExecuted, 1)
	pkg.CommandCounter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("command", cmdName),
	))
	
	b.logger.Info("slash command invoked",
		"command", cmdName,
		"user_id", userID,
		"username", i.Member.User.Username,
		"guild_id", i.GuildID,
		"channel_id", i.ChannelID,
		"member_permissions", i.Member.Permissions,
	)

	switch cmdName {
	case "chattiest":
		b.handleChattiest(s, i)
	case "userstats":
		b.handleUserStats(s, i)
	case "trending":
		b.handleTrending(s, i)
	case "channel-activity":
		b.handleChannelActivity(s, i)
	case "rising-stars":
		b.handleRisingStars(s, i)
	case "peak-hours":
		b.handlePeakHours(s, i)
	case "inactive-users", "inactive-channels", "server-activity", "add-bot-admin-role", "remove-bot-admin-role", "list-bot-admin-roles", "set-response-channel", "clear-response-channel", "list-response-channels":
		b.logger.Info("routing to admin command handler", "command", cmdName)
		b.handleAdminCommands(s, i)
	default:
		b.logger.Warn("unknown command", "command", cmdName)
	}
}

func (b *Bot) handleChattiest(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	
	// Generate unique request ID for coverage tracking
	requestID := fmt.Sprintf("chattiest_%s_%d", i.Member.User.ID, time.Now().UnixNano())
	pkg.StartRequestTracking(requestID)
	
	ctx, span := pkg.StartSpan(ctx, "bot.handleChattiest",
		attribute.String("command", "chattiest"),
		attribute.String("user_id", i.Member.User.ID),
		attribute.String("guild_id", i.GuildID),
		attribute.String("request_id", requestID),
	)
	defer func() {
		span.End()
		pkg.LogCoverageForRequest(requestID, "chattiest")
	}()
	
	start := time.Now()
	
	// Defer response to give us time to query
	err := b.discordAPI.InteractionRespond(ctx, i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		b.logger.Error("failed to defer interaction response", "error", err)
		return
	}

	options := i.ApplicationCommandData().Options
	period := options[0].StringValue()
	
	// Default to current channel if no channel specified
	channelID := i.ChannelID
	for _, option := range options {
		if option.Name == "channel" {
			channelID = option.ChannelValue(s).ID
		}
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.String("period", period),
		attribute.String("channel_id", channelID),
	)

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	params := db.GetChattiestUsersParams{
		ServerID:   i.GuildID,
		ChannelID:  channelID,
		Period:     period,
		LimitCount: 10,
	}

	pkg.AddSpanEvent(ctx, "executing_database_query")
	
	dbStart := time.Now()
	users, err := b.queries.GetChattiestUsers(dbCtx, params)
	
	// Record database operation metrics
	dbDuration := time.Since(dbStart)
	pkg.EnsureInitialized()
	pkg.DatabaseOperationTimer.Record(ctx, dbDuration.Milliseconds(),
		metric.WithAttributes(
			attribute.String("operation", "GetChattiestUsers"),
			attribute.String("period", period),
		),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "database_query_failed")
		b.logger.Error("failed to query chattiest users",
			"error", err,
			"guild_id", i.GuildID,
			"period", period,
		)
		b.editResponse(s, i, "Error querying database. Please try again later.")
		return
	}

	
	pkg.AddSpanAttributes(ctx,
		attribute.Int("db_result_count", len(users)),
		attribute.Int64("db_duration_ms", dbDuration.Milliseconds()),
	)

	// Format period for display
	var displayPeriod string
	switch period {
	case "today":
		displayPeriod = "Today"
	case "yesterday":
		displayPeriod = "Yesterday"
	case "week":
		displayPeriod = "This Week"
	case "month":
		displayPeriod = "This Month"
	default:
		displayPeriod = period
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("🏆 Chattiest Users - %s", displayPeriod),
		Color:     0x00ff00,
		Timestamp: time.Now().Format(time.RFC3339),
		Fields:    []*discordgo.MessageEmbedField{},
	}

	if len(users) == 0 {
		embed.Description = "No messages found for this period. Start chatting to see stats!"
		embed.Color = 0xffcc00 // Yellow for warning
		b.logger.Info("no users found for chattiest query",
			"guild_id", i.GuildID,
			"period", period,
			"channel_id", channelID,
		)
	} else {
		pkg.AddSpanEvent(ctx, "building_user_list", attribute.Int("user_count", len(users)))
		for idx, row := range users {
			user, err := b.userService.GetUser(ctx, row.UserID)
			if err != nil {
				b.logger.Warn("failed to fetch user info",
					"user_id", row.UserID,
					"error", err,
				)
				// Still show the user ID if we can't get the username
				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:   fmt.Sprintf("%d. User %s", idx+1, row.UserID),
					Value:  fmt.Sprintf("%d messages", row.MessageCount),
					Inline: false,
				})
				continue
			}

			medal := ""
			switch idx + 1 {
			case 1:
				medal = "🥇"
			case 2:
				medal = "🥈"
			case 3:
				medal = "🥉"
			}

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%s %d. %s", medal, idx+1, user.Username),
				Value:  fmt.Sprintf("%d messages", row.MessageCount),
				Inline: false,
			})
		}
		
		// Add footer with query info
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Showing top %d users", len(users)),
		}
	}

	// Record command metrics
	pkg.EnsureInitialized()
	pkg.CommandCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("command", "chattiest"),
		attribute.String("period", period),
	))
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int("result_count", len(users)),
		attribute.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	// Send final response
	_, err = b.discordAPI.InteractionResponseEdit(ctx, i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		b.logger.Error("failed to edit interaction response with embed", 
			"error", err,
			"title", embed.Title,
		)
	} else {
		b.logger.Debug("interaction response sent",
			"title", embed.Title,
			"fields", len(embed.Fields),
		)
	}
}

func (b *Bot) handleUserStats(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	
	// Generate unique request ID for coverage tracking
	requestID := fmt.Sprintf("user_stats_%s_%d", i.Member.User.ID, time.Now().UnixNano())
	pkg.StartRequestTracking(requestID)
	
	ctx, span := pkg.StartSpan(ctx, "bot.handleUserStats",
		attribute.String("command", "userstats"),
		attribute.String("user_id", i.Member.User.ID),
		attribute.String("guild_id", i.GuildID),
		attribute.String("request_id", requestID),
	)
	defer func() {
		span.End()
		pkg.LogCoverageForRequest(requestID, "userstats")
	}()
	
	start := time.Now()

	err := b.discordAPI.InteractionRespond(ctx, i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		b.logger.Error("failed to defer interaction response", "error", err)
		return
	}

	options := i.ApplicationCommandData().Options
	userID := options[0].UserValue(s).ID
	
	period := "week"
	for _, option := range options {
		if option.Name == "period" {
			period = option.StringValue()
		}
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.String("target_user_id", userID),
		attribute.String("period", period),
	)

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pkg.AddSpanEvent(ctx, "executing_user_stats_query")
	dbStart := time.Now()
	stats, err := b.queries.GetUserStats(dbCtx, db.GetUserStatsParams{
		UserID:   userID,
		Period:   period,
		ServerID: i.GuildID,
	})
	dbDuration := time.Since(dbStart)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("db_duration_ms", dbDuration.Milliseconds()),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "user_stats_query_failed")
		b.editResponse(s, i, "Error querying database")
		return
	}

	user, _ := b.userService.GetUser(ctx, userID)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("message_count", stats.MessageCount),
		attribute.Int64("channels_active", stats.ChannelsActive),
		attribute.Int64("days_active", stats.DaysActive),
		attribute.Float64("percent_change", stats.PercentChange),
	)

	trend := "📈"
	if stats.PercentChange < 0 {
		trend = "📉"
	} else if stats.PercentChange == 0 {
		trend = "➡️"
	}

	username := "Unknown User"
	if user != nil {
		username = user.Username
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("📊 Stats for %s", username),
		Color:     0x3498db,
		Timestamp: time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Messages This " + period,
				Value:  fmt.Sprintf("%d", stats.MessageCount),
				Inline: true,
			},
			{
				Name:   "Active Channels",
				Value:  fmt.Sprintf("%d", stats.ChannelsActive),
				Inline: true,
			},
			{
				Name:   "Days Active",
				Value:  fmt.Sprintf("%d", stats.DaysActive),
				Inline: true,
			},
			{
				Name:   "Trend",
				Value:  fmt.Sprintf("%s %.1f%% vs previous %s", trend, stats.PercentChange, period),
				Inline: false,
			},
		},
	}

	pkg.AddSpanAttributes(ctx,
		attribute.Int64("total_duration_ms", time.Since(start).Milliseconds()),
	)

	b.editResponseEmbed(s, i, embed)
}

func (b *Bot) handleTrending(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	
	// Generate unique request ID for coverage tracking
	requestID := fmt.Sprintf("trending_%s_%d", i.Member.User.ID, time.Now().UnixNano())
	pkg.StartRequestTracking(requestID)
	
	ctx, span := pkg.StartSpan(ctx, "bot.handleTrending",
		attribute.String("command", "trending"),
		attribute.String("user_id", i.Member.User.ID),
		attribute.String("guild_id", i.GuildID),
		attribute.String("request_id", requestID),
	)
	defer func() {
		span.End()
		pkg.LogCoverageForRequest(requestID, "trending")
	}()
	
	start := time.Now()

	err := b.discordAPI.InteractionRespond(ctx, i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		b.logger.Error("failed to defer interaction response", "error", err)
		return
	}

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pkg.AddSpanEvent(ctx, "executing_trending_query")
	dbStart := time.Now()
	trending, err := b.queries.GetTrendingUsers(dbCtx, db.GetTrendingUsersParams{
		ServerID:   i.GuildID,
		LimitCount: 10,
	})
	dbDuration := time.Since(dbStart)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("db_duration_ms", dbDuration.Milliseconds()),
		attribute.Int("trending_users_count", len(trending)),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "trending_query_failed")
		b.editResponse(s, i, "Error querying database")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:     "📈 Trending Activity (Week over Week)",
		Color:     0xe74c3c,
		Timestamp: time.Now().Format(time.RFC3339),
		Fields:    []*discordgo.MessageEmbedField{},
	}

	pkg.AddSpanEvent(ctx, "processing_trending_users", attribute.Int("user_count", len(trending)))
	userFetchStart := time.Now()
	
	for _, row := range trending {
		user, err := b.userService.GetUser(ctx, row.UserID)
		if err != nil {
			continue
		}

		trend := "🔥"
		if row.PercentChange < -20 {
			trend = "❄️"
		} else if row.PercentChange < 0 {
			trend = "📉"
		} else if row.PercentChange > 50 {
			trend = "🚀"
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s", trend, user.Username),
			Value:  fmt.Sprintf("%+.1f%% (%d → %d messages)", row.PercentChange, row.LastWeek, row.ThisWeek),
			Inline: false,
		})
	}
	
	userFetchDuration := time.Since(userFetchStart)
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("user_fetch_duration_ms", userFetchDuration.Milliseconds()),
		attribute.Int("embed_fields_count", len(embed.Fields)),
		attribute.Int64("total_duration_ms", time.Since(start).Milliseconds()),
	)

	b.editResponseEmbed(s, i, embed)
}

func (b *Bot) handleChannelActivity(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	
	// Generate unique request ID for coverage tracking
	requestID := fmt.Sprintf("channel_activity_%s_%d", i.Member.User.ID, time.Now().UnixNano())
	pkg.StartRequestTracking(requestID)
	
	ctx, span := pkg.StartSpan(ctx, "bot.handleChannelActivity",
		attribute.String("command", "channel-activity"),
		attribute.String("user_id", i.Member.User.ID),
		attribute.String("guild_id", i.GuildID),
		attribute.String("request_id", requestID),
	)
	defer func() {
		span.End()
		pkg.LogCoverageForRequest(requestID, "channel-activity")
	}()
	
	start := time.Now()

	err := b.discordAPI.InteractionRespond(ctx, i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		b.logger.Error("failed to defer interaction response", "error", err)
		return
	}

	// Get channel ID
	channelID := i.ChannelID
	options := i.ApplicationCommandData().Options
	for _, option := range options {
		if option.Name == "channel" {
			channelID = option.ChannelValue(s).ID
		}
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.String("target_channel_id", channelID),
	)

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()


	// Get activity data
	pkg.AddSpanEvent(ctx, "querying_channel_activity")
	activity, err := b.queries.GetChannelActivityTimeline(dbCtx, db.GetChannelActivityTimelineParams{
		ChannelID: channelID,
		ServerID:  i.GuildID,
	})
	if err != nil {
		pkg.RecordError(ctx, err, "get_channel_activity_failed")
		b.logger.Error("failed to get channel activity", "error", err)
		b.editResponse(s, i, "Error querying activity data")
		return
	}

	b.logger.Debug("activity query results",
		"activity_count", len(activity),
	)

	// Log first few results
	for i, point := range activity {
		if i >= 3 {
			break
		}
		b.logger.Debug("activity point",
			"interval", point.IntervalStart.Time,
			"user_id", point.UserID,
			"count", point.MessageCount,
		)
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int("activity_points", len(activity)),
	)

	if len(activity) == 0 {
		b.editResponse(s, i, "No activity in this channel today")
		return
	}

	// Get usernames for all users with tracing
	pkg.AddSpanEvent(ctx, "fetching_user_data", attribute.Int("unique_users", len(activity)))
	usernames := make(map[string]string)
	userFetchStart := time.Now()
	
	for _, point := range activity {
		if _, ok := usernames[point.UserID]; !ok {
			user, err := b.userService.GetUser(ctx, point.UserID)
			if err == nil {
				usernames[point.UserID] = user.Username
			} else {
				usernames[point.UserID] = "Unknown"
			}
		}
	}
	
	userFetchDuration := time.Since(userFetchStart)
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("user_fetch_duration_ms", userFetchDuration.Milliseconds()),
		attribute.Int("unique_users_fetched", len(usernames)),
	)

	// Generate image with tracing
	pkg.AddSpanEvent(ctx, "generating_chart")
	chartStart := time.Now()
	imageData, err := charts.GenerateChannelActivityImage(ctx, activity, usernames)
	chartDuration := time.Since(chartStart)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("chart_generation_duration_ms", chartDuration.Milliseconds()),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "chart_generation_failed")
		b.logger.Error("failed to generate activity image", "error", err)
		b.editResponse(s, i, "Error generating activity graph")
		return
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int("chart_data_size_bytes", len(imageData)),
	)

	// Get channel name with tracing
	pkg.AddSpanEvent(ctx, "preparing_response")
	channel, _ := b.discordAPI.Channel(ctx, channelID)
	channelName := "this channel"
	if channel != nil {
		channelName = "#" + channel.Name
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.String("channel_name", channelName),
	)

	// Send image response with tracing
	pkg.AddSpanEvent(ctx, "sending_discord_response")
	responseStart := time.Now()
	_, err = b.discordAPI.InteractionResponseEdit(ctx, i.Interaction, &discordgo.WebhookEdit{
		Content: func() *string { content := fmt.Sprintf("📊 **Activity Timeline for %s**", channelName); return &content }(),
		Files: []*discordgo.File{
			{
				Name:        "channel_activity.png",
				ContentType: "image/png",
				Reader:      bytes.NewReader(imageData),
			},
		},
	})
	responseDuration := time.Since(responseStart)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("discord_response_duration_ms", responseDuration.Milliseconds()),
		attribute.Bool("response_success", err == nil),
		attribute.Int64("total_duration_ms", time.Since(start).Milliseconds()),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "discord_response_failed")
		b.logger.Error("failed to send activity timeline", "error", err)
	}
}

func (b *Bot) handleRisingStars(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	
	// Generate unique request ID for coverage tracking
	requestID := fmt.Sprintf("rising_stars_%s_%d", i.Member.User.ID, time.Now().UnixNano())
	pkg.StartRequestTracking(requestID)
	
	ctx, span := pkg.StartSpan(ctx, "bot.handleRisingStars",
		attribute.String("command", "rising-stars"),
		attribute.String("user_id", i.Member.User.ID),
		attribute.String("guild_id", i.GuildID),
		attribute.String("request_id", requestID),
	)
	defer func() {
		span.End()
		pkg.LogCoverageForRequest(requestID, "rising-stars")
	}()
	
	start := time.Now()

	err := b.discordAPI.InteractionRespond(ctx, i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		b.logger.Error("failed to defer interaction response", "error", err)
		return
	}

	limit := 10
	options := i.ApplicationCommandData().Options
	for _, option := range options {
		if option.Name == "limit" {
			limit = int(option.IntValue())
		}
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int("limit", limit),
	)

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pkg.AddSpanEvent(ctx, "executing_rising_stars_query")
	dbStart := time.Now()
	stars, err := b.queries.GetRisingStars(dbCtx, db.GetRisingStarsParams{
		ServerID:   i.GuildID,
		LimitCount: int32(limit),
	})
	dbDuration := time.Since(dbStart)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("db_duration_ms", dbDuration.Milliseconds()),
		attribute.Int("stars_count", len(stars)),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "rising_stars_query_failed")
		b.logger.Error("failed to get rising stars", "error", err)
		b.editResponse(s, i, "Error querying rising stars")
		return
	}

	if len(stars) == 0 {
		embed := &discordgo.MessageEmbed{
			Title:       "🌟 Rising Stars",
			Description: "No users with significant growth found. Check back when activity increases!",
			Color:       0xffcc00,
			Timestamp:   time.Now().Format(time.RFC3339),
		}
		b.editResponseEmbed(s, i, embed)
		return
	}

	// Get usernames with tracing
	pkg.AddSpanEvent(ctx, "fetching_user_data", attribute.Int("stars_count", len(stars)))
	userFetchStart := time.Now()
	usernames := make(map[string]string)
	for _, star := range stars {
		user, err := b.userService.GetUser(ctx, star.UserID)
		if err == nil {
			usernames[star.UserID] = user.Username
		} else {
			usernames[star.UserID] = "Unknown"
		}
	}
	userFetchDuration := time.Since(userFetchStart)

	// Generate chart image with tracing
	pkg.AddSpanEvent(ctx, "generating_chart")
	chartStart := time.Now()
	imageData, err := charts.GenerateRisingStarsChart(ctx, stars, usernames)
	chartDuration := time.Since(chartStart)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("user_fetch_duration_ms", userFetchDuration.Milliseconds()),
		attribute.Int64("chart_generation_duration_ms", chartDuration.Milliseconds()),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "chart_generation_failed")
		b.logger.Error("failed to generate rising stars chart", "error", err)
		b.editResponse(s, i, "Error generating chart")
		return
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int("chart_data_size_bytes", len(imageData)),
	)

	// Create embed with details
	embed := &discordgo.MessageEmbed{
		Title:       "🌟 Rising Stars - Rapidly Growing Contributors",
		Description: "Users showing significant activity growth (20%+ increase)",
		Color:       0xffd700,
		Timestamp:   time.Now().Format(time.RFC3339),
		Image: &discordgo.MessageEmbedImage{
			URL: "attachment://rising_stars.png",
		},
		Fields: []*discordgo.MessageEmbedField{},
	}

	// Add top 3 details
	for idx, star := range stars {
		if idx >= 3 {
			break
		}
		
		username := usernames[star.UserID]
		
		// Calculate trajectory emoji
		trajectory := "📈"
		if star.GrowthRate > 100 {
			trajectory = "🚀"
		} else if star.GrowthRate > 50 {
			trajectory = "⚡"
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: fmt.Sprintf("%s %s", trajectory, username),
			Value: fmt.Sprintf("+%.0f%% growth | %.1f msgs/day", star.GrowthRate, star.DailyAverage),
			Inline: true,
		})
	}

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Showing %d rising stars | Based on week-over-week growth", len(stars)),
	}

	// Send embed with image with tracing
	pkg.AddSpanEvent(ctx, "sending_discord_response")
	responseStart := time.Now()
	_, err = b.discordAPI.InteractionResponseEdit(ctx, i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
		Files: []*discordgo.File{
			{
				Name:        "rising_stars.png",
				ContentType: "image/png",
				Reader:      bytes.NewReader(imageData),
			},
		},
	})
	responseDuration := time.Since(responseStart)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("discord_response_duration_ms", responseDuration.Milliseconds()),
		attribute.Bool("response_success", err == nil),
		attribute.Int64("total_duration_ms", time.Since(start).Milliseconds()),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "discord_response_failed")
		b.logger.Error("failed to send rising stars chart", "error", err)
	}
}

func (b *Bot) handlePeakHours(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	
	// Generate unique request ID for coverage tracking
	requestID := fmt.Sprintf("peak_hours_%s_%d", i.Member.User.ID, time.Now().UnixNano())
	pkg.StartRequestTracking(requestID)
	
	ctx, span := pkg.StartSpan(ctx, "bot.handlePeakHours",
		attribute.String("command", "peak-hours"),
		attribute.String("user_id", i.Member.User.ID),
		attribute.String("guild_id", i.GuildID),
		attribute.String("request_id", requestID),
	)
	defer func() {
		span.End()
		pkg.LogCoverageForRequest(requestID, "peak-hours")
	}()
	
	start := time.Now()

	err := b.discordAPI.InteractionRespond(ctx, i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		b.logger.Error("failed to defer interaction response", "error", err)
		return
	}

	var channelID string
	options := i.ApplicationCommandData().Options
	for _, option := range options {
		if option.Name == "channel" {
			channelID = option.ChannelValue(s).ID
		}
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.String("target_channel_id", channelID),
	)

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pkg.AddSpanEvent(ctx, "executing_peak_hours_query")
	dbStart := time.Now()
	peakData, err := b.queries.GetChannelPeakHours(dbCtx, db.GetChannelPeakHoursParams{
		ServerID:  i.GuildID,
		ChannelID: channelID,
	})
	dbDuration := time.Since(dbStart)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("db_duration_ms", dbDuration.Milliseconds()),
		attribute.Int("peak_data_points", len(peakData)),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "peak_hours_query_failed")
		b.logger.Error("failed to get peak hours", "error", err)
		b.editResponse(s, i, "Error querying peak hours data")
		return
	}

	if len(peakData) == 0 {
		b.editResponse(s, i, "No activity data available for analysis")
		return
	}

	// Convert to format expected by heatmap generator
	var heatmapData []struct {
		Hour         int
		MessageCount int64
	}
	for _, data := range peakData {
		heatmapData = append(heatmapData, struct {
			Hour         int
			MessageCount int64
		}{Hour: int(data.HourOfDay), MessageCount: data.MessageCount})
	}

	// Get scope name with tracing
	pkg.AddSpanEvent(ctx, "processing_scope_info")
	scopeName := "the server"
	if channelID != "" {
		channel, err := b.discordAPI.Channel(ctx, channelID)
		if err == nil {
			scopeName = "#" + channel.Name
		}
	}

	// Calculate peak hour
	var peakHour int
	var peakCount int64
	for _, data := range peakData {
		if data.MessageCount > peakCount {
			peakCount = data.MessageCount
			peakHour = int(data.HourOfDay)
		}
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.String("scope_name", scopeName),
		attribute.Int("peak_hour", peakHour),
		attribute.Int64("peak_count", peakCount),
	)

	// Generate heatmap image with tracing
	pkg.AddSpanEvent(ctx, "generating_heatmap")
	chartStart := time.Now()
	imageData, err := charts.GeneratePeakHoursHeatmap(ctx, peakData)
	chartDuration := time.Since(chartStart)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("chart_generation_duration_ms", chartDuration.Milliseconds()),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "heatmap_generation_failed")
		b.logger.Error("failed to generate heatmap image", "error", err)
		b.editResponse(s, i, "Error generating heatmap")
		return
	}
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int("chart_data_size_bytes", len(imageData)),
	)

	// Create embed with image
	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("🕐 Peak Activity Hours for %s", scopeName),
		Color:     0x9b59b6,
		Timestamp: time.Now().Format(time.RFC3339),
		Image: &discordgo.MessageEmbedImage{
			URL: "attachment://peak_hours.png",
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "🏆 Peak Hour",
				Value:  fmt.Sprintf("%02d:00-%02d:00", peakHour, (peakHour+1)%24),
				Inline: true,
			},
			{
				Name:   "📊 Peak Messages",
				Value:  fmt.Sprintf("%d messages", peakCount),
				Inline: true,
			},
			{
				Name:   "📊 Total Messages",
				Value:  fmt.Sprintf("%d messages", func() int64 {
					total := int64(0)
					for _, d := range peakData {
						total += d.MessageCount
					}
					return total
				}()),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Based on last 30 days of activity",
		},
	}

	// Send embed with image with tracing
	pkg.AddSpanEvent(ctx, "sending_discord_response")
	responseStart := time.Now()
	_, err = b.discordAPI.InteractionResponseEdit(ctx, i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
		Files: []*discordgo.File{
			{
				Name:        "peak_hours.png",
				ContentType: "image/png",
				Reader:      bytes.NewReader(imageData),
			},
		},
	})
	responseDuration := time.Since(responseStart)
	
	pkg.AddSpanAttributes(ctx,
		attribute.Int64("discord_response_duration_ms", responseDuration.Milliseconds()),
		attribute.Bool("response_success", err == nil),
		attribute.Int64("total_duration_ms", time.Since(start).Milliseconds()),
	)
	
	if err != nil {
		pkg.RecordError(ctx, err, "discord_response_failed")
		b.logger.Error("failed to send peak hours heatmap", "error", err)
	}
}


func (b *Bot) editResponse(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
	if err != nil {
		b.logger.Error("failed to edit interaction response", 
			"error", err,
			"content", content,
		)
	}
}

func (b *Bot) editResponseEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		b.logger.Error("failed to edit interaction response with embed", 
			"error", err,
			"title", embed.Title,
		)
	} else {
		b.logger.Debug("interaction response sent",
			"title", embed.Title,
			"fields", len(embed.Fields),
		)
	}
}
