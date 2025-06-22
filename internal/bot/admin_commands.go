package bot

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/imeyer/discord-activity-bot/db"
	"github.com/imeyer/discord-activity-bot/internal/charts"
)

var adminCommands = []*discordgo.ApplicationCommand{
	{
		Name:                     "inactive-users",
		Description:              "Show users who haven't posted in N days (admin only)",
		DefaultMemberPermissions: &adminPermission,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "days",
				Description: "Number of days of inactivity",
				Required:    true,
				MinValue:    &minDays,
				MaxValue:    365,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "limit",
				Description: "Maximum number of users to show (default: 10)",
				Required:    false,
				MinValue:    &minLimit,
				MaxValue:    50,
			},
		},
	},
	{
		Name:                     "inactive-channels",
		Description:              "Show channels that haven't had posts in N weeks (admin only)",
		DefaultMemberPermissions: &adminPermission,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "weeks",
				Description: "Number of weeks of inactivity",
				Required:    true,
				MinValue:    &minWeeks,
				MaxValue:    52,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "limit",
				Description: "Maximum number of channels to show (default: 10)",
				Required:    false,
				MinValue:    &minLimit,
				MaxValue:    50,
			},
		},
	},
	{
		Name:                     "server-activity",
		Description:              "Show comprehensive server activity report (admin only)",
		DefaultMemberPermissions: &adminPermission,
	},
	{
		Name:                     "add-bot-admin-role",
		Description:              "Grant a role access to bot admin commands",
		DefaultMemberPermissions: &adminPermission,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "role",
				Description: "Role to grant admin access",
				Required:    true,
			},
		},
	},
	{
		Name:                     "remove-bot-admin-role",
		Description:              "Remove a role's access to bot admin commands",
		DefaultMemberPermissions: &adminPermission,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "role",
				Description: "Role to remove admin access from",
				Required:    true,
			},
		},
	},
	{
		Name:                     "list-bot-admin-roles",
		Description:              "List roles that have access to bot admin commands",
		DefaultMemberPermissions: &adminPermission,
	},
	{
		Name:                     "set-response-channel",
		Description:              "Configure where bot responses are sent (admin only)",
		DefaultMemberPermissions: &adminPermission,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Channel to send responses to",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "command",
				Description: "Specific command (leave empty for all commands)",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "All Commands", Value: ""},
					{Name: "chattiest", Value: "chattiest"},
					{Name: "userstats", Value: "userstats"},
					{Name: "trending", Value: "trending"},
					{Name: "channel-activity", Value: "channel-activity"},
					{Name: "rising-stars", Value: "rising-stars"},
					{Name: "peak-hours", Value: "peak-hours"},
				},
			},
		},
	},
	{
		Name:                     "clear-response-channel",
		Description:              "Clear response channel configuration (admin only)",
		DefaultMemberPermissions: &adminPermission,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "command",
				Description: "Specific command to clear (leave empty to clear all)",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "All Commands", Value: "all"},
					{Name: "Global Default", Value: ""},
					{Name: "chattiest", Value: "chattiest"},
					{Name: "userstats", Value: "userstats"},
					{Name: "trending", Value: "trending"},
					{Name: "channel-activity", Value: "channel-activity"},
					{Name: "rising-stars", Value: "rising-stars"},
					{Name: "peak-hours", Value: "peak-hours"},
				},
			},
		},
	},
	{
		Name:                     "list-response-channels",
		Description:              "List current response channel configuration (admin only)",
		DefaultMemberPermissions: &adminPermission,
	},
}

var (
	// Permission constants
	adminPermission = int64(discordgo.PermissionAdministrator)
	ownerPermission = int64(discordgo.PermissionManageGuild)

	// Validation constants
	minDays  = 1.0
	minWeeks = 1.0
	minLimit = 1.0
)

// AdminRoleManager manages custom admin roles per server
type AdminRoleManager struct {
	roles   map[string][]string // guildID -> roleIDs (cache)
	logger  *slog.Logger
	queries db.Querier
}

func NewAdminRoleManager(logger *slog.Logger, queries db.Querier) *AdminRoleManager {
	return &AdminRoleManager{
		roles:   make(map[string][]string),
		logger:  logger,
		queries: queries,
	}
}

func (arm *AdminRoleManager) AddRole(ctx context.Context, guildID, roleID, addedBy string) error {
	// Add to database
	err := arm.queries.AddAdminRole(ctx, db.AddAdminRoleParams{
		GuildID: guildID,
		RoleID:  roleID,
		AddedBy: addedBy,
	})
	if err != nil {
		return err
	}

	// Update cache
	if arm.roles[guildID] == nil {
		arm.roles[guildID] = []string{}
	}

	// Check if role already exists in cache
	for _, r := range arm.roles[guildID] {
		if r == roleID {
			return nil
		}
	}

	arm.roles[guildID] = append(arm.roles[guildID], roleID)
	arm.logger.Info("added admin role",
		"guild_id", guildID,
		"role_id", roleID,
		"added_by", addedBy,
	)
	return nil
}

func (arm *AdminRoleManager) RemoveRole(ctx context.Context, guildID, roleID string) error {
	// Remove from database
	err := arm.queries.RemoveAdminRole(ctx, db.RemoveAdminRoleParams{
		GuildID: guildID,
		RoleID:  roleID,
	})
	if err != nil {
		return err
	}

	// Update cache
	if arm.roles[guildID] == nil {
		return nil
	}

	newRoles := []string{}
	for _, r := range arm.roles[guildID] {
		if r != roleID {
			newRoles = append(newRoles, r)
		}
	}

	arm.roles[guildID] = newRoles
	arm.logger.Info("removed admin role",
		"guild_id", guildID,
		"role_id", roleID,
	)
	return nil
}

func (arm *AdminRoleManager) HasAdminRole(member *discordgo.Member, guildID string) bool {
	// Check custom admin roles
	adminRoles := arm.roles[guildID]
	
	arm.logger.Info("checking custom admin roles",
		"user_id", member.User.ID,
		"username", member.User.Username,
		"guild_id", guildID,
		"member_roles", member.Roles,
		"configured_admin_roles", adminRoles,
	)
	
	if len(adminRoles) == 0 {
		arm.logger.Info("no custom admin roles configured for guild", "guild_id", guildID)
		return false
	}
	
	for _, memberRole := range member.Roles {
		for _, adminRole := range adminRoles {
			if memberRole == adminRole {
				arm.logger.Info("user has matching admin role",
					"user_id", member.User.ID,
					"username", member.User.Username,
					"role_id", memberRole,
				)
				return true
			}
		}
	}

	arm.logger.Info("user does not have any matching admin roles",
		"user_id", member.User.ID,
		"username", member.User.Username,
	)
	return false
}

// Add admin role manager to Bot struct
func (b *Bot) setupAdminCommands() {
	b.adminRoles = NewAdminRoleManager(b.logger, b.queries)

	// Load admin roles from database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	roles, err := b.queries.GetAllAdminRoles(ctx)
	if err != nil {
		b.logger.Error("failed to load admin roles from database", "error", err)
	} else {
		// Populate cache
		for _, role := range roles {
			if b.adminRoles.roles[role.GuildID] == nil {
				b.adminRoles.roles[role.GuildID] = []string{}
			}
			b.adminRoles.roles[role.GuildID] = append(b.adminRoles.roles[role.GuildID], role.RoleID)
		}
		b.logger.Info("loaded admin roles from database", "count", len(roles))
	}

	// Load admin roles from environment as fallback
	// Format: ADMIN_ROLES_<GUILD_ID>=role1,role2,role3
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "ADMIN_ROLES_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				guildID := strings.TrimPrefix(parts[0], "ADMIN_ROLES_")
				roles := strings.Split(parts[1], ",")
				for _, role := range roles {
					roleID := strings.TrimSpace(role)
					// Add to database if not already there
					b.adminRoles.AddRole(ctx, guildID, roleID, "environment")
				}
			}
		}
	}
}

func (b *Bot) registerAdminCommands() {
	b.logger.Info("registering admin commands", "count", len(adminCommands))

	for _, cmd := range adminCommands {
		_, err := b.discord.ApplicationCommandCreate(b.discord.State.User.ID, "", cmd)
		if err != nil {
			b.logger.Error("failed to create admin command",
				"command", cmd.Name,
				"error", err,
			)
		} else {
			b.logger.Debug("registered admin command", "command", cmd.Name)
		}
	}
}

func (b *Bot) handleAdminCommands(s *discordgo.Session, i *discordgo.InteractionCreate) {
	cmdName := i.ApplicationCommandData().Name

	// Check permissions for admin commands
	if !b.checkAdminPermission(i.Member) {
		b.respondError(s, i, "You don't have permission to use this command. This command requires Administrator permission or a configured admin role.")
		return
	}

	switch cmdName {
	case "inactive-users":
		b.handleInactiveUsers(s, i)
	case "inactive-channels":
		b.handleInactiveChannels(s, i)
	case "server-activity":
		b.handleServerActivity(s, i)
	case "add-bot-admin-role":
		b.handleAddBotAdminRole(s, i)
	case "remove-bot-admin-role":
		b.handleRemoveBotAdminRole(s, i)
	case "list-bot-admin-roles":
		b.handleListBotAdminRoles(s, i)
	case "set-response-channel":
		b.handleSetResponseChannel(s, i)
	case "clear-response-channel":
		b.handleClearResponseChannel(s, i)
	case "list-response-channels":
		b.handleListResponseChannels(s, i)
	}
}

func (b *Bot) checkAdminPermission(member *discordgo.Member) bool {
	b.logger.Info("checking admin permission",
		"user_id", member.User.ID,
		"username", member.User.Username,
		"guild_id", member.GuildID,
		"permissions", member.Permissions,
		"roles", member.Roles,
	)

	// Always allow Discord Administrators
	permissions := member.Permissions
	if permissions&discordgo.PermissionAdministrator != 0 {
		b.logger.Info("user has administrator permission", 
			"user_id", member.User.ID,
			"username", member.User.Username,
		)
		return true
	}

	b.logger.Info("user does not have administrator permission, checking custom admin roles",
		"user_id", member.User.ID,
		"username", member.User.Username,
	)

	// Check custom admin roles configured by the server
	hasCustomAdminRole := b.adminRoles.HasAdminRole(member, member.GuildID)
	
	b.logger.Info("admin role check result",
		"user_id", member.User.ID,
		"username", member.User.Username,
		"has_custom_admin_role", hasCustomAdminRole,
	)
	
	return hasCustomAdminRole
}

func (b *Bot) handleInactiveUsers(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	options := i.ApplicationCommandData().Options
	days := int(options[0].IntValue())
	limit := 10

	if len(options) > 1 {
		limit = int(options[1].IntValue())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	users, err := b.queries.GetInactiveUsers(ctx, db.GetInactiveUsersParams{
		ServerID:   i.GuildID,
		Days:       int32(days),
		LimitCount: int32(limit),
	})
	if err != nil {
		b.logger.Error("failed to query inactive users", "error", err)
		b.editResponse(s, i, "Error querying database")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("👻 Inactive Users (No posts in %d+ days)", days),
		Color:     0xff6b6b,
		Timestamp: time.Now().Format(time.RFC3339),
		Fields:    []*discordgo.MessageEmbedField{},
	}

	if len(users) == 0 {
		embed.Description = fmt.Sprintf("Great news! No users have been inactive for %d+ days.", days)
		embed.Color = 0x00ff00
	} else {
		for idx, row := range users {
			user, err := b.userService.GetUser(context.Background(), row.UserID)
			username := "Unknown User"
			if err == nil {
				username = user.Username
			}

			lastActive := "Never"
			if row.LastMessageTime != nil {
				if msgTime, ok := row.LastMessageTime.(time.Time); ok {
					lastActive = msgTime.Format("Jan 2, 2006")
				}
			}

			daysInactive := int(row.DaysInactive)

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name: fmt.Sprintf("%d. %s", idx+1, username),
				Value: fmt.Sprintf("Last active: %s (%d days ago)\nTotal messages: %d",
					lastActive, daysInactive, row.TotalMessages),
				Inline: false,
			})
		}

		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Showing %d of potentially more inactive users", len(users)),
		}
	}

	b.editResponseEmbed(s, i, embed)
}

func (b *Bot) handleInactiveChannels(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	options := i.ApplicationCommandData().Options
	weeks := int(options[0].IntValue())
	limit := 10

	if len(options) > 1 {
		limit = int(options[1].IntValue())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	channels, err := b.queries.GetInactiveChannels(ctx, db.GetInactiveChannelsParams{
		ServerID:   i.GuildID,
		Weeks:      int32(weeks),
		LimitCount: int32(limit),
	})
	if err != nil {
		b.logger.Error("failed to query inactive channels", "error", err)
		b.editResponse(s, i, "Error querying database")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("🏚️ Inactive Channels (No posts in %d+ weeks)", weeks),
		Color:     0x95a5a6,
		Timestamp: time.Now().Format(time.RFC3339),
		Fields:    []*discordgo.MessageEmbedField{},
	}

	if len(channels) == 0 {
		embed.Description = fmt.Sprintf("All channels have been active within the last %d weeks!", weeks)
		embed.Color = 0x00ff00
	} else {
		for idx, row := range channels {
			channel, err := s.Channel(row.ChannelID)
			channelName := "Unknown Channel"
			if err == nil {
				channelName = channel.Name
			}

			weeksInactive := int(row.WeeksInactive)
			lastActive := "Never"
			if msgTime, ok := row.LastMessageTime.(time.Time); ok {
				lastActive = msgTime.Format("Jan 2, 2006")
			}

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name: fmt.Sprintf("%d. #%s", idx+1, channelName),
				Value: fmt.Sprintf("Last active: %s (%d weeks ago)\nTotal messages: %d\nUnique users: %d",
					lastActive, weeksInactive, row.TotalMessages, row.UniqueUsers),
				Inline: false,
			})
		}

		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Showing %d inactive channels", len(channels)),
		}
	}

	b.editResponseEmbed(s, i, embed)
}

func (b *Bot) handleServerActivity(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	summary, err := b.queries.GetServerActivitySummary(ctx, i.GuildID)
	if err != nil {
		b.logger.Error("failed to query server activity", "error", err)
		b.editResponse(s, i, "Error querying database")
		return
	}

	// Calculate percentages
	userActivityToday := float64(summary.ActiveUsersToday) / float64(summary.TotalUsers) * 100
	userActivityWeek := float64(summary.ActiveUsersWeek) / float64(summary.TotalUsers) * 100
	userActivityMonth := float64(summary.ActiveUsersMonth) / float64(summary.TotalUsers) * 100

	channelActivityToday := float64(summary.ActiveChannelsToday) / float64(summary.TotalChannels) * 100
	channelActivityWeek := float64(summary.ActiveChannelsWeek) / float64(summary.TotalChannels) * 100
	channelActivityMonth := float64(summary.ActiveChannelsMonth) / float64(summary.TotalChannels) * 100

	// charts.Generate donut chart
	imageData, err := charts.GenerateServerActivityDonut(ctx, summary)
	if err != nil {
		b.logger.Error("failed to generate activity donut", "error", err)
		// Continue without image
	}

	embed := &discordgo.MessageEmbed{
		Title:     "📊 Server Activity Report",
		Color:     0x3498db,
		Timestamp: time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "👥 User Activity",
				Value: fmt.Sprintf("**Today:** %d/%d (%.1f%%)\n**This Week:** %d/%d (%.1f%%)\n**This Month:** %d/%d (%.1f%%)",
					summary.ActiveUsersToday, summary.TotalUsers, userActivityToday,
					summary.ActiveUsersWeek, summary.TotalUsers, userActivityWeek,
					summary.ActiveUsersMonth, summary.TotalUsers, userActivityMonth),
				Inline: true,
			},
			{
				Name: "💬 Channel Activity",
				Value: fmt.Sprintf("**Today:** %d/%d (%.1f%%)\n**This Week:** %d/%d (%.1f%%)\n**This Month:** %d/%d (%.1f%%)",
					summary.ActiveChannelsToday, summary.TotalChannels, channelActivityToday,
					summary.ActiveChannelsWeek, summary.TotalChannels, channelActivityWeek,
					summary.ActiveChannelsMonth, summary.TotalChannels, channelActivityMonth),
				Inline: true,
			},
			{
				Name: "📈 Message Volume",
				Value: fmt.Sprintf("**Today:** %s\n**This Week:** %s\n**This Month:** %s\n**All Time:** %s",
					formatNumber(summary.MessagesToday),
					formatNumber(summary.MessagesWeek),
					formatNumber(summary.MessagesMonth),
					formatNumber(summary.TotalMessages)),
				Inline: false,
			},
			{
				Name: "📊 Daily Averages",
				Value: fmt.Sprintf("**Messages/Day (Week):** %.0f\n**Messages/Day (Month):** %.0f",
					float64(summary.MessagesWeek)/7,
					float64(summary.MessagesMonth)/30),
				Inline: true,
			},
			{
				Name:   "🎯 Engagement Score",
				Value:  calculateEngagementScore(userActivityWeek, channelActivityWeek),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Total tracked: %d users, %d channels", summary.TotalUsers, summary.TotalChannels),
		},
	}

	if imageData != nil {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: "attachment://activity_donut.png",
		}

		_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{embed},
			Files: []*discordgo.File{
				{
					Name:        "activity_donut.png",
					ContentType: "image/png",
					Reader:      bytes.NewReader(imageData),
				},
			},
		})
		if err != nil {
			b.logger.Error("failed to send activity report with image", "error", err)
			b.editResponseEmbed(s, i, embed)
		}
	} else {
		b.editResponseEmbed(s, i, embed)
	}
}

func (b *Bot) handleAddBotAdminRole(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	options := i.ApplicationCommandData().Options
	role := options[0].RoleValue(s, i.GuildID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if role is @everyone
	if role.ID == i.GuildID {
		b.editResponse(s, i, "❌ Cannot add @everyone as an admin role for security reasons.")
		return
	}

	err := b.adminRoles.AddRole(ctx, i.GuildID, role.ID, i.Member.User.ID)
	if err != nil {
		b.logger.Error("failed to add admin role", "error", err)
		b.editResponse(s, i, "❌ Failed to add admin role. Please try again.")
		return
	}

	b.editResponse(s, i, fmt.Sprintf("✅ Added **%s** as a bot admin role. Members with this role can now use admin commands.", role.Name))
}

func (b *Bot) handleRemoveBotAdminRole(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	options := i.ApplicationCommandData().Options
	role := options[0].RoleValue(s, i.GuildID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := b.adminRoles.RemoveRole(ctx, i.GuildID, role.ID)
	if err != nil {
		b.logger.Error("failed to remove admin role", "error", err)
		b.editResponse(s, i, "❌ Failed to remove admin role. Please try again.")
		return
	}

	b.editResponse(s, i, fmt.Sprintf("✅ Removed **%s** from bot admin roles. Members with this role can no longer use admin commands.", role.Name))
}

func (b *Bot) handleListBotAdminRoles(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	roles, err := b.queries.GetAdminRoles(ctx, i.GuildID)
	if err != nil {
		b.logger.Error("failed to get admin roles", "error", err)
		b.editResponse(s, i, "❌ Failed to retrieve admin roles. Please try again.")
		return
	}

	if len(roles) == 0 {
		embed := &discordgo.MessageEmbed{
			Title:       "🔐 Bot Admin Roles",
			Description: "No custom admin roles configured. Only Discord Administrators can use admin commands.",
			Color:       0xffa500,
			Timestamp:   time.Now().Format(time.RFC3339),
		}
		b.editResponseEmbed(s, i, embed)
		return
	}

	// Get role names
	var roleList []string
	for _, roleID := range roles {
		// Try to get role name from Discord
		guildRoles, err := s.GuildRoles(i.GuildID)
		if err == nil {
			for _, guildRole := range guildRoles {
				if guildRole.ID == roleID {
					roleList = append(roleList, fmt.Sprintf("• **%s** (<@&%s>)", guildRole.Name, roleID))
					break
				}
			}
		} else {
			roleList = append(roleList, fmt.Sprintf("• <@&%s>", roleID))
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🔐 Bot Admin Roles",
		Description: "The following roles have access to bot admin commands:\n\n" + strings.Join(roleList, "\n"),
		Color:       0x00ff00,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Discord Administrators always have access",
		},
	}

	b.editResponseEmbed(s, i, embed)
}

func (b *Bot) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("❌ %s", message),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return strconv.FormatInt(n, 10)
}

func calculateEngagementScore(userActivity, channelActivity float64) string {
	score := (userActivity + channelActivity) / 2

	var emoji string
	var rating string

	switch {
	case score >= 80:
		emoji = "🔥"
		rating = "Excellent"
	case score >= 60:
		emoji = "✨"
		rating = "Good"
	case score >= 40:
		emoji = "👍"
		rating = "Fair"
	case score >= 20:
		emoji = "📉"
		rating = "Low"
	default:
		emoji = "💤"
		rating = "Very Low"
	}

	return fmt.Sprintf("%s %s (%.1f%%)", emoji, rating, score)
}

// Response channel configuration handlers
func (b *Bot) handleSetResponseChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	options := i.ApplicationCommandData().Options
	channelID := options[0].ChannelValue(s).ID
	commandName := ""
	
	// Get command name if specified
	if len(options) > 1 {
		commandName = options[1].StringValue()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set the response channel configuration
	err := b.queries.SetResponseChannel(ctx, db.SetResponseChannelParams{
		GuildID:      i.GuildID,
		CommandName:  commandName,
		ChannelID:    channelID,
		ConfiguredBy: i.Member.User.ID,
	})
	if err != nil {
		b.logger.Error("failed to set response channel", "error", err)
		b.respondError(s, i, "Failed to set response channel configuration.")
		return
	}

	// Get channel name for confirmation
	channel, _ := s.Channel(channelID)
	channelName := "#" + channel.Name

	var scope string
	if commandName == "" {
		scope = "all commands"
	} else {
		scope = fmt.Sprintf("command `%s`", commandName)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "✅ Response Channel Set",
		Description: fmt.Sprintf("Bot responses for %s will now be sent to %s", scope, channelName),
		Color:       0x00ff00,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	b.editResponseEmbed(s, i, embed)
}

func (b *Bot) handleClearResponseChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	options := i.ApplicationCommandData().Options
	commandName := ""
	
	// Get command name if specified
	if len(options) > 0 {
		commandName = options[0].StringValue()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	var scope string

	if commandName == "all" {
		// Clear all configurations
		err = b.queries.ClearAllResponseChannels(ctx, i.GuildID)
		scope = "all commands"
	} else {
		// Clear specific command or global
		err = b.queries.ClearResponseChannel(ctx, db.ClearResponseChannelParams{
			GuildID:     i.GuildID,
			CommandName: commandName,
		})
		if commandName == "" {
			scope = "global default"
		} else {
			scope = fmt.Sprintf("command `%s`", commandName)
		}
	}

	if err != nil {
		b.logger.Error("failed to clear response channel", "error", err)
		b.respondError(s, i, "Failed to clear response channel configuration.")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "✅ Response Channel Cleared",
		Description: fmt.Sprintf("Response channel configuration for %s has been cleared. Responses will now go to the original channel.", scope),
		Color:       0x00ff00,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	b.editResponseEmbed(s, i, embed)
}

func (b *Bot) handleListResponseChannels(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	configs, err := b.queries.ListResponseChannels(ctx, i.GuildID)
	if err != nil {
		b.logger.Error("failed to list response channels", "error", err)
		b.respondError(s, i, "Failed to retrieve response channel configurations.")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:     "📋 Response Channel Configuration",
		Color:     0x3498db,
		Timestamp: time.Now().Format(time.RFC3339),
		Fields:    []*discordgo.MessageEmbedField{},
	}

	if len(configs) == 0 {
		embed.Description = "No response channels configured. All responses will go to the original channel where commands are invoked."
	} else {
		for _, config := range configs {
			// Get channel name
			channel, err := s.Channel(config.ChannelID)
			channelName := config.ChannelID
			if err == nil {
				channelName = "#" + channel.Name
			}

			// Get user who configured it
			user, _ := b.userService.GetUser(context.Background(), config.ConfiguredBy)
			configuredBy := config.ConfiguredBy
			if user != nil {
				configuredBy = user.Username
			}

			var commandDisplay string
			if config.CommandName == "" {
				commandDisplay = "All Commands (Global)"
			} else {
				commandDisplay = fmt.Sprintf("`%s`", config.CommandName)
			}

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   commandDisplay,
				Value:  fmt.Sprintf("**Channel:** %s\n**Set by:** %s\n**Date:** %s", channelName, configuredBy, config.ConfiguredAt.Format("Jan 2, 2006")),
				Inline: false,
			})
		}
	}

	b.editResponseEmbed(s, i, embed)
}
