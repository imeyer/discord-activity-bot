package bot

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/bwmarrin/discordgo"
	"github.com/imeyer/discord-activity-bot/internal/pkg"
)

// RecoveryMiddleware wraps Discord handlers with panic recovery
func (b *Bot) withRecovery(handler func(*discordgo.Session, *discordgo.InteractionCreate)) func(*discordgo.Session, *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic with stack trace
				b.logger.Error("panic recovered in Discord handler",
					"panic", r,
					"user_id", getUserID(i),
					"command", getCommandName(i),
					"guild_id", i.GuildID,
					"channel_id", i.ChannelID,
					"stack", string(debug.Stack()),
				)

				// Record error metrics if available
				if pkg.ErrorCounter != nil {
					pkg.ErrorCounter.Add(context.Background(), 1)
				}

				// Try to respond to the user with an error message
				b.respondToPanicError(s, i)
			}
		}()

		// Execute the original handler
		handler(s, i)
	}
}

// withMessageRecovery wraps message handlers with panic recovery
func (b *Bot) withMessageRecovery(handler func(*discordgo.Session, *discordgo.MessageCreate)) func(*discordgo.Session, *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic with stack trace
				b.logger.Error("panic recovered in message handler",
					"panic", r,
					"user_id", m.Author.ID,
					"username", m.Author.Username,
					"guild_id", m.GuildID,
					"channel_id", m.ChannelID,
					"message_id", m.ID,
					"stack", string(debug.Stack()),
				)

				// Record error metrics if available
				if pkg.ErrorCounter != nil {
					pkg.ErrorCounter.Add(context.Background(), 1)
				}
			}
		}()

		// Execute the original handler
		handler(s, m)
	}
}

// Helper functions to safely extract information from interactions
func getUserID(i *discordgo.InteractionCreate) string {
	if i == nil {
		return "unknown"
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return "unknown"
}

func getCommandName(i *discordgo.InteractionCreate) string {
	if i == nil || i.Interaction == nil {
		return "unknown"
	}
	if i.Type == discordgo.InteractionApplicationCommand {
		return i.ApplicationCommandData().Name
	}
	return fmt.Sprintf("interaction_type_%d", i.Type)
}

// respondToPanicError attempts to send an error response to the user
func (b *Bot) respondToPanicError(s *discordgo.Session, i *discordgo.InteractionCreate) {
	defer func() {
		// Don't let error responses panic as well
		if r := recover(); r != nil {
			b.logger.Error("panic in error response handler", "panic", r)
		}
	}()

	if i == nil || i.Interaction == nil {
		return
	}

	// Try different response methods
	errorMessage := "❌ An unexpected error occurred. Please try again later."

	// First try to create a new response (most common case)
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: errorMessage,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		// If that fails, try to edit the response (maybe it was already responded to)
		_, editErr := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &errorMessage,
		})
		if editErr != nil {
			b.logger.Warn("failed to respond to interaction after panic", 
				"respond_error", err, 
				"edit_error", editErr)
		}
	}
}