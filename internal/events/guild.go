package events

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/0xSalik/specter/internal/embed"
	"github.com/0xSalik/specter/internal/guildsetup"
)

func (h *Handlers) onGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Prime invite-usage tracking and snapshot known members on every
	// GuildCreate (including reconnects) so joins can be attributed to an
	// invite and leaves can report tenure. A missing Manage Server permission
	// is expected for some guilds and only degrades inviter resolution.
	if err := h.deps.Invites.Prime(s, g.ID); err != nil {
		log.Debug().Err(err).Str("guild", g.ID).Msg("could not prime invites (Manage Server permission?)")
	}
	h.deps.Invites.SnapshotGuild(g.ID, g.Members)

	isNew, err := h.deps.Store.EnsureGuild(ctx, g.ID)
	if err != nil {
		log.Error().Err(err).Str("guild", g.ID).Msg("guild_create: ensure guild")
		return
	}

	// Bot-level join log: only for genuine new additions, not the GuildCreate
	// replay that fires for every existing guild on startup/reconnect. The
	// bot's own JoinedAt is "now" only when freshly added.
	if h.deps.Config.GuildJoinLogChannelID != "" && recentlyJoined(g) {
		h.logBotGuildJoin(s, g)
	}

	if !isNew {
		return // reconnect / already known
	}

	log.Info().Str("guild", g.ID).Str("name", g.Name).Msg("joined new guild")

	res, err := guildsetup.EnsureLogInfrastructure(ctx, s, h.deps.Store, g.ID)
	if err != nil {
		log.Error().Err(err).Str("guild", g.ID).Msg("guild_create: provision logs")
		return
	}

	if res.Config == nil || res.Config.GeneralLogID == nil || *res.Config.GeneralLogID == "" {
		return
	}

	desc := strings.Join([]string{
		"Specter is a professional administration and utility bot.",
		"",
		"Run `/setup` to configure leveling, automod, join-to-create voice, and the embed color.",
		"All moderation and event log channels have been created under the **Specter Logs** category.",
		"Use `/help` to explore available commands.",
	}, "\n")

	b := embed.New(s, g.ID).Title("Specter is online.").Description(desc).
		Field("Server", g.Name, true).
		Field("Members", fmt.Sprintf("%d", g.MemberCount), true).
		Field("Date", time.Now().Format("2006-01-02"), true).
		Timestamp()
	if len(res.Failed) > 0 {
		b.Field("Channels not created", fmt.Sprintf("%v (check bot permissions)", res.Failed), false)
	}
	if _, err := s.ChannelMessageSendEmbed(*res.Config.GeneralLogID, b.Build()); err != nil {
		log.Warn().Err(err).Msg("guild_create: welcome message")
	}
}

func (h *Handlers) onGuildDelete(s *discordgo.Session, g *discordgo.GuildDelete) {
	if g.Unavailable {
		return // outage, not a removal
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if h.deps.Config.GuildJoinLogChannelID != "" {
		h.logBotGuildLeave(s, g)
	}

	h.deps.Invites.Forget(g.ID)
	if err := h.deps.Store.DeleteGuild(ctx, g.ID); err != nil {
		log.Error().Err(err).Str("guild", g.ID).Msg("guild_delete: cleanup")
	}
}

// recentlyJoined reports whether the bot was added to the guild within the last
// minute, distinguishing a real join from a startup/reconnect GuildCreate.
func recentlyJoined(g *discordgo.GuildCreate) bool {
	if g.JoinedAt.IsZero() {
		return false
	}
	return time.Since(g.JoinedAt) < time.Minute
}

// logBotGuildJoin posts a detailed embed to the configured channel whenever the
// bot is added to a new server.
func (h *Handlers) logBotGuildJoin(s *discordgo.Session, g *discordgo.GuildCreate) {
	channelID := h.deps.Config.GuildJoinLogChannelID

	owner := fmt.Sprintf("<@%s>", g.OwnerID)
	if u, err := s.User(g.OwnerID); err == nil && u != nil {
		owner = fmt.Sprintf("%s (`%s`)", tagOf(u), u.ID)
	}

	em := buildGuildJoinEmbed(s, colorGuildOf(s, channelID), g, owner, guildTotal(s))
	if _, err := s.ChannelMessageSendEmbed(channelID, em); err != nil {
		log.Warn().Err(err).Str("guild", g.ID).Str("channel", channelID).Msg("guild join log: send failed")
	}
}

// BuildGuildJoinEmbedForTest exposes the pure join-log embed builder to tests.
func BuildGuildJoinEmbedForTest(g *discordgo.GuildCreate, owner string, total int) *discordgo.MessageEmbed {
	return buildGuildJoinEmbed(nil, "", g, owner, total)
}

// buildGuildJoinEmbed renders the bot-join log embed. It is pure (no network)
// so it can be unit-tested; owner and total are resolved by the caller.
func buildGuildJoinEmbed(s *discordgo.Session, colorGuildID string, g *discordgo.GuildCreate, owner string, total int) *discordgo.MessageEmbed {
	created, _ := discordgo.SnowflakeTimestamp(g.ID)
	joined := g.JoinedAt
	if joined.IsZero() {
		joined = time.Now()
	}

	b := embed.New(s, colorGuildID).
		Title("Joined a new server").
		AsSuccess().
		Field("Server", fmt.Sprintf("%s (`%s`)", g.Name, g.ID), false).
		Field("Owner", owner, false).
		Field("Members", fmt.Sprintf("%d", g.MemberCount), true).
		Field("Channels", fmt.Sprintf("%d", len(g.Channels)), true).
		Field("Roles", fmt.Sprintf("%d", len(g.Roles)), true).
		Field("Server Created", fmt.Sprintf("<t:%d:F> (<t:%d:R>)", created.Unix(), created.Unix()), false).
		Field("Bot Joined", fmt.Sprintf("<t:%d:F>", joined.Unix()), true).
		Field("Now Serving", fmt.Sprintf("%d servers", total), true).
		Timestamp()

	if g.Icon != "" {
		b.Thumbnail(g.IconURL("256"))
	}
	if g.Description != "" {
		b.Field("Description", truncate(g.Description, 1000), false)
	}
	if extra := serverExtras(g.Guild); extra != "" {
		b.Field("Details", extra, false)
	}
	return b.Build()
}

// logBotGuildLeave posts a notice when the bot is removed from a server. The
// GuildDelete payload only reliably carries the ID, so detail is best-effort.
func (h *Handlers) logBotGuildLeave(s *discordgo.Session, g *discordgo.GuildDelete) {
	channelID := h.deps.Config.GuildJoinLogChannelID

	name := fmt.Sprintf("`%s`", g.ID)
	memberCount := -1
	if g.BeforeDelete != nil {
		if g.BeforeDelete.Name != "" {
			name = fmt.Sprintf("%s (`%s`)", g.BeforeDelete.Name, g.ID)
		}
		memberCount = g.BeforeDelete.MemberCount
	}

	b := embed.New(s, colorGuildOf(s, channelID)).
		Title("Removed from a server").
		AsError().
		Field("Server", name, false).
		Field("Now Serving", fmt.Sprintf("%d servers", guildTotal(s)), true).
		Timestamp()
	if memberCount >= 0 {
		b.Field("Members", fmt.Sprintf("%d", memberCount), true)
	}
	if g.BeforeDelete != nil && g.BeforeDelete.Icon != "" {
		b.Thumbnail(g.BeforeDelete.IconURL("256"))
	}

	if _, err := s.ChannelMessageSendEmbed(channelID, b.Build()); err != nil {
		log.Warn().Err(err).Str("guild", g.ID).Str("channel", channelID).Msg("guild leave log: send failed")
	}
}

// serverExtras renders optional server metadata (verification level, boost tier)
// when available.
func serverExtras(g *discordgo.Guild) string {
	if g == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	parts = append(parts, fmt.Sprintf("Verification: %s", verificationLevel(g.VerificationLevel)))
	parts = append(parts, fmt.Sprintf("Boost tier: %d", g.PremiumTier))
	if g.PremiumSubscriptionCount > 0 {
		parts = append(parts, fmt.Sprintf("Boosts: %d", g.PremiumSubscriptionCount))
	}
	return strings.Join(parts, " • ")
}

func verificationLevel(v discordgo.VerificationLevel) string {
	switch v {
	case discordgo.VerificationLevelNone:
		return "None"
	case discordgo.VerificationLevelLow:
		return "Low"
	case discordgo.VerificationLevelMedium:
		return "Medium"
	case discordgo.VerificationLevelHigh:
		return "High"
	case discordgo.VerificationLevelVeryHigh:
		return "Very High"
	default:
		return "Unknown"
	}
}

// guildTotal returns how many guilds the bot is currently in, per state.
func guildTotal(s *discordgo.Session) int {
	if s.State == nil {
		return 0
	}
	return len(s.State.Guilds)
}

// colorGuildOf returns the guild ID that owns a channel (for embed accent
// color), or "" to fall back to the default color.
func colorGuildOf(s *discordgo.Session, channelID string) string {
	if s.State == nil {
		return ""
	}
	if ch, err := s.State.Channel(channelID); err == nil && ch != nil {
		return ch.GuildID
	}
	return ""
}

func tagOf(u *discordgo.User) string {
	if u == nil {
		return "Unknown"
	}
	if u.Discriminator == "" || u.Discriminator == "0" {
		return u.Username
	}
	return u.Username + "#" + u.Discriminator
}
