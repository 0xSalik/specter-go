// Package modlog centralizes dispatch of moderation and audit events to the
// per-guild log channels created on join. It honors per-event overrides and
// never panics if a target channel has been deleted.
package modlog

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/salik/specter/internal/db"
	"github.com/salik/specter/internal/db/queries"
	"github.com/salik/specter/internal/embed"
)

// Event type constants used for routing and override lookups.
const (
	EventMemberJoin    = "member_join"
	EventMemberLeave   = "member_leave"
	EventMemberUpdate  = "member_update"
	EventMessageDelete = "message_delete"
	EventMessageEdit   = "message_edit"
	EventWarn          = "warn"
	EventTimeout       = "timeout"
	EventKick          = "kick"
	EventBan           = "ban"
	EventUnban         = "unban"
	EventChannelUpdate = "channel_update"
	EventGuildUpdate   = "guild_update"
	EventRoleUpdate    = "role_update"
	EventAutomod       = "automod"
)

// ModLogEvent describes a single loggable occurrence.
type ModLogEvent struct {
	GuildID    string
	EventType  string
	ActorID    string
	TargetID   string
	TargetName string
	Reason     string
	Extra      map[string]string
	Timestamp  time.Time
}

// Logger dispatches events to the correct guild log channel.
type Logger struct {
	store *queries.Store
}

// New constructs a Logger.
func New(store *queries.Store) *Logger {
	return &Logger{store: store}
}

// ChannelField returns which guilds-table column an event type routes to by
// default (before any per-event override is applied). Exported for testing.
func ChannelField(g *queries.GuildConfig, eventType string) *string {
	switch eventType {
	case EventMemberJoin, EventMemberLeave, EventMemberUpdate:
		return g.UserLogID
	case EventMessageDelete, EventMessageEdit:
		return g.MessageLogID
	case EventWarn, EventTimeout, EventAutomod:
		return g.WarnLogID
	case EventKick:
		return g.KickLogID
	case EventBan, EventUnban:
		return g.BanLogID
	case EventChannelUpdate, EventGuildUpdate, EventRoleUpdate:
		return g.GeneralLogID
	default:
		return g.GeneralLogID
	}
}

// ResolveChannel returns the target channel for an event, honoring overrides.
// The returned bool reports whether logging is enabled for this event.
func (l *Logger) ResolveChannel(ctx context.Context, guildID, eventType string) (string, bool, error) {
	if ov, err := l.store.GetOverride(ctx, guildID, eventType); err == nil {
		if !ov.Enabled {
			return "", false, nil
		}
		if ov.ChannelID != nil && *ov.ChannelID != "" {
			return *ov.ChannelID, true, nil
		}
	} else if !errors.Is(err, db.ErrNotFound) {
		return "", false, err
	}

	g, err := l.store.GetGuild(ctx, guildID)
	if err != nil {
		return "", false, err
	}
	field := ChannelField(g, eventType)
	if field == nil || *field == "" {
		return "", false, nil
	}
	return *field, true, nil
}

// Log builds and sends an embed describing the event. Failures are logged but
// never propagate as panics.
func (l *Logger) Log(s *discordgo.Session, ev ModLogEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	channelID, enabled, err := l.ResolveChannel(ctx, ev.GuildID, ev.EventType)
	if err != nil {
		log.Error().Err(err).Str("guild", ev.GuildID).Str("event", ev.EventType).Msg("resolve modlog channel")
		return
	}
	if !enabled || channelID == "" {
		return
	}

	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}

	b := embed.New(s, ev.GuildID).
		Title(titleFor(ev.EventType)).
		Timestamp()

	if ev.TargetName != "" || ev.TargetID != "" {
		b.Field("Target", display(ev.TargetName, ev.TargetID), true)
	}
	if ev.ActorID != "" {
		b.Field("Actor", mention(ev.ActorID), true)
	}
	if ev.Reason != "" {
		b.Field("Reason", ev.Reason, false)
	}
	for _, k := range sortedKeys(ev.Extra) {
		b.Field(k, ev.Extra[k], false)
	}

	if _, err := s.ChannelMessageSendEmbed(channelID, b.Build()); err != nil {
		log.Warn().Err(err).Str("guild", ev.GuildID).Str("channel", channelID).
			Str("event", ev.EventType).Msg("failed to send modlog message")
	}
}

func titleFor(eventType string) string {
	switch eventType {
	case EventMemberJoin:
		return "Member Joined"
	case EventMemberLeave:
		return "Member Left"
	case EventMemberUpdate:
		return "Member Updated"
	case EventMessageDelete:
		return "Message Deleted"
	case EventMessageEdit:
		return "Message Edited"
	case EventWarn:
		return "Warning Issued"
	case EventTimeout:
		return "Member Timed Out"
	case EventKick:
		return "Member Kicked"
	case EventBan:
		return "Member Banned"
	case EventUnban:
		return "Member Unbanned"
	case EventChannelUpdate:
		return "Channel Updated"
	case EventGuildUpdate:
		return "Server Updated"
	case EventRoleUpdate:
		return "Role Updated"
	case EventAutomod:
		return "Automod Action"
	default:
		return "Event"
	}
}

func display(name, id string) string {
	if name != "" && id != "" {
		return fmt.Sprintf("%s (`%s`)", name, id)
	}
	if name != "" {
		return name
	}
	return mention(id)
}

func mention(id string) string {
	if id == "" || id == "System" {
		return "System"
	}
	return fmt.Sprintf("<@%s>", id)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
