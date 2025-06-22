package bot

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/imeyer/discord-activity-bot/internal/pkg"
	"go.opentelemetry.io/otel/attribute"
)

// TracedDiscordAPI wraps Discord API calls with tracing
type TracedDiscordAPI struct {
	session *discordgo.Session
}

// NewTracedDiscordAPI creates a traced Discord API wrapper
func NewTracedDiscordAPI(session *discordgo.Session) *TracedDiscordAPI {
	return &TracedDiscordAPI{session: session}
}

// User fetches user info with tracing
func (t *TracedDiscordAPI) User(ctx context.Context, userID string) (*discordgo.User, error) {
	ctx, span := pkg.StartSpan(ctx, "discord_api.get_user",
		attribute.String("user_id", userID),
	)
	defer span.End()

	start := time.Now()
	user, err := t.session.User(userID)
	duration := time.Since(start)

	if err != nil {
		pkg.RecordError(ctx, err, "discord_api_get_user_failed")
	}

	pkg.AddSpanAttributes(ctx,
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)
	
	if user != nil {
		pkg.AddSpanAttributes(ctx,
			attribute.String("username", user.Username),
			attribute.Bool("bot", user.Bot),
		)
	}

	return user, err
}

// Channel fetches channel info with tracing
func (t *TracedDiscordAPI) Channel(ctx context.Context, channelID string) (*discordgo.Channel, error) {
	ctx, span := pkg.StartSpan(ctx, "discord_api.get_channel",
		attribute.String("channel_id", channelID),
	)
	defer span.End()

	start := time.Now()
	channel, err := t.session.Channel(channelID)
	duration := time.Since(start)

	if err != nil {
		pkg.RecordError(ctx, err, "discord_api_get_channel_failed")
	}

	pkg.AddSpanAttributes(ctx,
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)
	
	if channel != nil {
		pkg.AddSpanAttributes(ctx,
			attribute.String("channel_name", channel.Name),
			attribute.Int("channel_type", int(channel.Type)),
		)
	}

	return channel, err
}

// InteractionRespond sends an interaction response with tracing
func (t *TracedDiscordAPI) InteractionRespond(ctx context.Context, interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	ctx, span := pkg.StartSpan(ctx, "discord_api.interaction_respond",
		attribute.Int("response_type", int(resp.Type)),
	)
	defer span.End()

	start := time.Now()
	err := t.session.InteractionRespond(interaction, resp)
	duration := time.Since(start)

	if err != nil {
		pkg.RecordError(ctx, err, "discord_api_interaction_respond_failed")
	}

	pkg.AddSpanAttributes(ctx,
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	return err
}

// InteractionResponseEdit edits an interaction response with tracing
func (t *TracedDiscordAPI) InteractionResponseEdit(ctx context.Context, interaction *discordgo.Interaction, edit *discordgo.WebhookEdit) (*discordgo.Message, error) {
	ctx, span := pkg.StartSpan(ctx, "discord_api.interaction_edit",
		attribute.Bool("has_content", edit.Content != nil),
		attribute.Bool("has_embeds", edit.Embeds != nil),
		attribute.Bool("has_files", edit.Files != nil),
	)
	defer span.End()

	start := time.Now()
	msg, err := t.session.InteractionResponseEdit(interaction, edit)
	duration := time.Since(start)

	if err != nil {
		pkg.RecordError(ctx, err, "discord_api_interaction_edit_failed")
	}

	pkg.AddSpanAttributes(ctx,
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)
	
	if edit.Embeds != nil {
		pkg.AddSpanAttributes(ctx,
			attribute.Int("embed_count", len(*edit.Embeds)),
		)
	}
	
	if edit.Files != nil {
		pkg.AddSpanAttributes(ctx,
			attribute.Int("file_count", len(edit.Files)),
		)
		
		// Calculate total file size
		totalSize := 0
		for _, file := range edit.Files {
			if file.Reader != nil {
				// We can't easily measure the size without consuming the reader,
				// but we can at least record that files were attached
				totalSize++ // Just count files
			}
		}
		pkg.AddSpanAttributes(ctx,
			attribute.Int("total_files", totalSize),
		)
	}

	return msg, err
}

// GuildRoles fetches guild roles with tracing
func (t *TracedDiscordAPI) GuildRoles(ctx context.Context, guildID string) ([]*discordgo.Role, error) {
	ctx, span := pkg.StartSpan(ctx, "discord_api.get_guild_roles",
		attribute.String("guild_id", guildID),
	)
	defer span.End()

	start := time.Now()
	roles, err := t.session.GuildRoles(guildID)
	duration := time.Since(start)

	if err != nil {
		pkg.RecordError(ctx, err, "discord_api_get_guild_roles_failed")
	}

	pkg.AddSpanAttributes(ctx,
		attribute.Int64("duration_ms", duration.Milliseconds()),
		attribute.Int("role_count", len(roles)),
	)

	return roles, err
}