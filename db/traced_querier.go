package db

import (
	"context"
	"fmt"
	"time"

	"github.com/imeyer/discord-activity-bot/internal/pkg"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// TracedQuerier wraps a database querier with OpenTelemetry tracing and metrics
type TracedQuerier struct {
	wrapped Querier
}

// NewTracedQuerier creates a new traced querier wrapper
func NewTracedQuerier(querier Querier) *TracedQuerier {
	return &TracedQuerier{
		wrapped: querier,
	}
}

// recordMetrics records database operation metrics
func (t *TracedQuerier) recordMetrics(ctx context.Context, operation string, duration time.Duration, err error) {
	pkg.EnsureInitialized()
	
	pkg.DatabaseOperationTimer.Record(ctx, duration.Milliseconds(),
		metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.Bool("success", err == nil),
		),
	)
}

// startSpan starts a new span for a database operation
func (t *TracedQuerier) startSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	pkg.EnsureInitialized()
	return pkg.StartSpan(ctx, ""+operation,
		attribute.String("operation", operation),
		attribute.String("system", "postgresql"),
	)
}

// AddAdminRole adds an admin role with tracing
func (t *TracedQuerier) AddAdminRole(ctx context.Context, arg AddAdminRoleParams) error {
	ctx, span := t.startSpan(ctx, "AddAdminRole")
	defer span.End()

	start := time.Now()
	err := t.wrapped.AddAdminRole(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("guild_id", arg.GuildID),
		attribute.String("role_id", arg.RoleID),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "AddAdminRole", duration, err)
	return err
}

// ClearAllResponseChannels clears all response channels with tracing
func (t *TracedQuerier) ClearAllResponseChannels(ctx context.Context, guildID string) error {
	ctx, span := t.startSpan(ctx, "ClearAllResponseChannels")
	defer span.End()

	start := time.Now()
	err := t.wrapped.ClearAllResponseChannels(ctx, guildID)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("guild_id", guildID),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "ClearAllResponseChannels", duration, err)
	return err
}

// ClearResponseChannel clears a response channel with tracing
func (t *TracedQuerier) ClearResponseChannel(ctx context.Context, arg ClearResponseChannelParams) error {
	ctx, span := t.startSpan(ctx, "ClearResponseChannel")
	defer span.End()

	start := time.Now()
	err := t.wrapped.ClearResponseChannel(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("guild_id", arg.GuildID),
		attribute.String("command_name", arg.CommandName),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "ClearResponseChannel", duration, err)
	return err
}

// GetAdminRoles gets admin roles with tracing
func (t *TracedQuerier) GetAdminRoles(ctx context.Context, guildID string) ([]string, error) {
	ctx, span := t.startSpan(ctx, "GetAdminRoles")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetAdminRoles(ctx, guildID)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("guild_id", guildID),
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetAdminRoles", duration, err)
	return result, err
}

// GetAllAdminRoles gets all admin roles with tracing
func (t *TracedQuerier) GetAllAdminRoles(ctx context.Context) ([]GetAllAdminRolesRow, error) {
	ctx, span := t.startSpan(ctx, "GetAllAdminRoles")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetAllAdminRoles(ctx)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetAllAdminRoles", duration, err)
	return result, err
}

// GetChannelActivityTimeline gets channel activity timeline with tracing
func (t *TracedQuerier) GetChannelActivityTimeline(ctx context.Context, arg GetChannelActivityTimelineParams) ([]GetChannelActivityTimelineRow, error) {
	ctx, span := t.startSpan(ctx, "GetChannelActivityTimeline")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetChannelActivityTimeline(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("channel_id", arg.ChannelID),
		attribute.String("server_id", arg.ServerID),
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetChannelActivityTimeline", duration, err)
	return result, err
}

// GetChannelPeakHours gets channel peak hours with tracing
func (t *TracedQuerier) GetChannelPeakHours(ctx context.Context, arg GetChannelPeakHoursParams) ([]GetChannelPeakHoursRow, error) {
	ctx, span := t.startSpan(ctx, "GetChannelPeakHours")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetChannelPeakHours(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("server_id", arg.ServerID),
		attribute.String("channel_id", arg.ChannelID),
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetChannelPeakHours", duration, err)
	return result, err
}

// GetChattiestUsers gets chattiest users with tracing
func (t *TracedQuerier) GetChattiestUsers(ctx context.Context, arg GetChattiestUsersParams) ([]GetChattiestUsersRow, error) {
	ctx, span := t.startSpan(ctx, "GetChattiestUsers")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetChattiestUsers(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("server_id", arg.ServerID),
		attribute.String("channel_id", arg.ChannelID),
		attribute.Int64("limit_count", int64(arg.LimitCount)),
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetChattiestUsers", duration, err)
	return result, err
}

// GetGlobalResponseChannel gets global response channel with tracing
func (t *TracedQuerier) GetGlobalResponseChannel(ctx context.Context, guildID string) (GetGlobalResponseChannelRow, error) {
	ctx, span := t.startSpan(ctx, "GetGlobalResponseChannel")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetGlobalResponseChannel(ctx, guildID)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("guild_id", guildID),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetGlobalResponseChannel", duration, err)
	return result, err
}

// GetInactiveChannels gets inactive channels with tracing
func (t *TracedQuerier) GetInactiveChannels(ctx context.Context, arg GetInactiveChannelsParams) ([]GetInactiveChannelsRow, error) {
	ctx, span := t.startSpan(ctx, "GetInactiveChannels")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetInactiveChannels(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("server_id", arg.ServerID),
		attribute.Int64("weeks", int64(arg.Weeks)),
		attribute.Int64("limit_count", int64(arg.LimitCount)),
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetInactiveChannels", duration, err)
	return result, err
}

// GetInactiveUsers gets inactive users with tracing
func (t *TracedQuerier) GetInactiveUsers(ctx context.Context, arg GetInactiveUsersParams) ([]GetInactiveUsersRow, error) {
	ctx, span := t.startSpan(ctx, "GetInactiveUsers")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetInactiveUsers(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("server_id", arg.ServerID),
		attribute.Int64("days", int64(arg.Days)),
		attribute.Int64("limit_count", int64(arg.LimitCount)),
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetInactiveUsers", duration, err)
	return result, err
}

// GetResponseChannel gets response channel with tracing
func (t *TracedQuerier) GetResponseChannel(ctx context.Context, arg GetResponseChannelParams) (GetResponseChannelRow, error) {
	ctx, span := t.startSpan(ctx, "GetResponseChannel")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetResponseChannel(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("guild_id", arg.GuildID),
		attribute.String("command_name", arg.CommandName),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetResponseChannel", duration, err)
	return result, err
}

// GetRisingStars gets rising stars with tracing
func (t *TracedQuerier) GetRisingStars(ctx context.Context, arg GetRisingStarsParams) ([]GetRisingStarsRow, error) {
	ctx, span := t.startSpan(ctx, "GetRisingStars")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetRisingStars(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("server_id", arg.ServerID),
		attribute.Int64("limit_count", int64(arg.LimitCount)),
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetRisingStars", duration, err)
	return result, err
}

// GetServerActivitySummary gets server activity summary with tracing
func (t *TracedQuerier) GetServerActivitySummary(ctx context.Context, serverID string) (GetServerActivitySummaryRow, error) {
	ctx, span := t.startSpan(ctx, "GetServerActivitySummary")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetServerActivitySummary(ctx, serverID)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("server_id", serverID),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetServerActivitySummary", duration, err)
	return result, err
}

// GetTrendingUsers gets trending users with tracing
func (t *TracedQuerier) GetTrendingUsers(ctx context.Context, arg GetTrendingUsersParams) ([]GetTrendingUsersRow, error) {
	ctx, span := t.startSpan(ctx, "GetTrendingUsers")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetTrendingUsers(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("server_id", arg.ServerID),
		attribute.Int64("limit_count", int64(arg.LimitCount)),
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetTrendingUsers", duration, err)
	return result, err
}

// GetUserDailyActivity gets user daily activity with tracing
func (t *TracedQuerier) GetUserDailyActivity(ctx context.Context, arg GetUserDailyActivityParams) ([]GetUserDailyActivityRow, error) {
	ctx, span := t.startSpan(ctx, "GetUserDailyActivity")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetUserDailyActivity(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("user_id", arg.UserID),
		attribute.String("server_id", arg.ServerID),
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetUserDailyActivity", duration, err)
	return result, err
}

// GetUserStats gets user stats with tracing
func (t *TracedQuerier) GetUserStats(ctx context.Context, arg GetUserStatsParams) (GetUserStatsRow, error) {
	ctx, span := t.startSpan(ctx, "GetUserStats")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.GetUserStats(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("user_id", arg.UserID),
		attribute.String("period", fmt.Sprintf("%v", arg.Period)),
		attribute.String("server_id", arg.ServerID),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "GetUserStats", duration, err)
	return result, err
}

// InsertMessage inserts a message with tracing
func (t *TracedQuerier) InsertMessage(ctx context.Context, arg InsertMessageParams) error {
	ctx, span := t.startSpan(ctx, "InsertMessage")
	defer span.End()

	start := time.Now()
	err := t.wrapped.InsertMessage(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("user_id", arg.UserID),
		attribute.String("channel_id", arg.ChannelID),
		attribute.String("server_id", arg.ServerID),
		attribute.String("message_id", arg.MessageID),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "InsertMessage", duration, err)
	return err
}

// ListResponseChannels lists response channels with tracing
func (t *TracedQuerier) ListResponseChannels(ctx context.Context, guildID string) ([]ListResponseChannelsRow, error) {
	ctx, span := t.startSpan(ctx, "ListResponseChannels")
	defer span.End()

	start := time.Now()
	result, err := t.wrapped.ListResponseChannels(ctx, guildID)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("guild_id", guildID),
		attribute.Int("result_count", len(result)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "ListResponseChannels", duration, err)
	return result, err
}

// RemoveAdminRole removes an admin role with tracing
func (t *TracedQuerier) RemoveAdminRole(ctx context.Context, arg RemoveAdminRoleParams) error {
	ctx, span := t.startSpan(ctx, "RemoveAdminRole")
	defer span.End()

	start := time.Now()
	err := t.wrapped.RemoveAdminRole(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("guild_id", arg.GuildID),
		attribute.String("role_id", arg.RoleID),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "RemoveAdminRole", duration, err)
	return err
}

// SetResponseChannel sets a response channel with tracing
func (t *TracedQuerier) SetResponseChannel(ctx context.Context, arg SetResponseChannelParams) error {
	ctx, span := t.startSpan(ctx, "SetResponseChannel")
	defer span.End()

	start := time.Now()
	err := t.wrapped.SetResponseChannel(ctx, arg)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("guild_id", arg.GuildID),
		attribute.String("command_name", arg.CommandName),
		attribute.String("channel_id", arg.ChannelID),
		attribute.String("configured_by", arg.ConfiguredBy),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	t.recordMetrics(ctx, "SetResponseChannel", duration, err)
	return err
}

// WithTx creates a new traced querier with a transaction
func (t *TracedQuerier) WithTx(tx pgx.Tx) *TracedQuerier {
	// We need to cast the wrapped querier back to *Queries to call WithTx
	if queries, ok := t.wrapped.(*Queries); ok {
		return NewTracedQuerier(queries.WithTx(tx))
	}
	// If it's already a TracedQuerier, unwrap it first
	if tracedQuerier, ok := t.wrapped.(*TracedQuerier); ok {
		return tracedQuerier.WithTx(tx)
	}
	// This shouldn't happen, but return a new queries instance as fallback
	return NewTracedQuerier(New(tx))
}

// User management methods

func (t *TracedQuerier) BatchUpsertUser(ctx context.Context, arg BatchUpsertUserParams) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "BatchUpsertUser", duration, nil)
	}()

	return t.wrapped.BatchUpsertUser(ctx, arg)
}

func (t *TracedQuerier) CleanupOldUsernameChanges(ctx context.Context) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "CleanupOldUsernameChanges", duration, nil)
	}()

	return t.wrapped.CleanupOldUsernameChanges(ctx)
}

func (t *TracedQuerier) GetActiveUsers(ctx context.Context, limit int32) ([]User, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "GetActiveUsers", duration, nil)
	}()

	return t.wrapped.GetActiveUsers(ctx, limit)
}

func (t *TracedQuerier) GetRecentUsernameChanges(ctx context.Context, limit int32) ([]GetRecentUsernameChangesRow, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "GetRecentUsernameChanges", duration, nil)
	}()

	return t.wrapped.GetRecentUsernameChanges(ctx, limit)
}

func (t *TracedQuerier) GetStaleUsers(ctx context.Context, limit int32) ([]GetStaleUsersRow, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "GetStaleUsers", duration, nil)
	}()

	return t.wrapped.GetStaleUsers(ctx, limit)
}

func (t *TracedQuerier) GetUser(ctx context.Context, userID string) (User, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "GetUser", duration, nil)
	}()

	return t.wrapped.GetUser(ctx, userID)
}

func (t *TracedQuerier) GetUserByUsername(ctx context.Context, username string) ([]User, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "GetUserByUsername", duration, nil)
	}()

	return t.wrapped.GetUserByUsername(ctx, username)
}

func (t *TracedQuerier) GetUserProfile(ctx context.Context, userID string) (GetUserProfileRow, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "GetUserProfile", duration, nil)
	}()

	return t.wrapped.GetUserProfile(ctx, userID)
}

func (t *TracedQuerier) GetUsernameHistory(ctx context.Context, arg GetUsernameHistoryParams) ([]GetUsernameHistoryRow, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "GetUsernameHistory", duration, nil)
	}()

	return t.wrapped.GetUsernameHistory(ctx, arg)
}

func (t *TracedQuerier) GetUsersNeedingRefresh(ctx context.Context, limit int32) ([]string, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "GetUsersNeedingRefresh", duration, nil)
	}()

	return t.wrapped.GetUsersNeedingRefresh(ctx, limit)
}

func (t *TracedQuerier) GetUsersWithoutRecentActivity(ctx context.Context, limit int32) ([]GetUsersWithoutRecentActivityRow, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "GetUsersWithoutRecentActivity", duration, nil)
	}()

	return t.wrapped.GetUsersWithoutRecentActivity(ctx, limit)
}

func (t *TracedQuerier) SearchUsersByPattern(ctx context.Context, arg SearchUsersByPatternParams) ([]User, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "SearchUsersByPattern", duration, nil)
	}()

	return t.wrapped.SearchUsersByPattern(ctx, arg)
}

func (t *TracedQuerier) UpdateUserLastMessage(ctx context.Context, arg UpdateUserLastMessageParams) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "UpdateUserLastMessage", duration, nil)
	}()

	return t.wrapped.UpdateUserLastMessage(ctx, arg)
}

func (t *TracedQuerier) MarkUserInactive(ctx context.Context, userID string) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "MarkUserInactive", duration, nil)
	}()

	return t.wrapped.MarkUserInactive(ctx, userID)
}

func (t *TracedQuerier) UpsertUser(ctx context.Context, arg UpsertUserParams) (User, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.recordMetrics(ctx, "UpsertUser", duration, nil)
	}()

	return t.wrapped.UpsertUser(ctx, arg)
}