package events

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/0xSalik/specter/internal/embed"
)

// defaultJoinMessage and defaultLeaveMessage are used when an admin enables
// welcome/goodbye messages without supplying custom text.
const (
	defaultJoinMessage  = "Welcome to {server}, {user}! You are member #{membercount}."
	defaultLeaveMessage = "{username} has left {server}. We now have {membercount} members."
)

// handleWelcomeJoin posts the configured welcome message to the channel and/or
// DMs the new member. Failures are logged, never fatal.
func (h *Handlers) handleWelcomeJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.User == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := h.deps.Store.GetWelcomeConfig(ctx, m.GuildID)
	if err != nil {
		log.Error().Err(err).Str("guild", m.GuildID).Msg("welcome: load config")
		return
	}

	if cfg.JoinEnabled && cfg.JoinChannelID != nil && *cfg.JoinChannelID != "" {
		text := defaultJoinMessage
		if cfg.JoinMessage != nil && *cfg.JoinMessage != "" {
			text = *cfg.JoinMessage
		}
		sendWelcome(s, m.GuildID, *cfg.JoinChannelID, renderWelcome(s, text, m.GuildID, m.User), cfg.UseEmbed, "Welcome")
	}

	if cfg.JoinDMEnabled {
		text := cfg.JoinDMMessage
		if text != nil && *text != "" {
			if ch, err := s.UserChannelCreate(m.User.ID); err == nil {
				sendWelcome(s, m.GuildID, ch.ID, renderWelcome(s, *text, m.GuildID, m.User), cfg.UseEmbed, "Welcome")
			}
		}
	}
}

// handleWelcomeLeave posts the configured goodbye message.
func (h *Handlers) handleWelcomeLeave(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m.User == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := h.deps.Store.GetWelcomeConfig(ctx, m.GuildID)
	if err != nil {
		log.Error().Err(err).Str("guild", m.GuildID).Msg("goodbye: load config")
		return
	}
	if !cfg.LeaveEnabled || cfg.LeaveChannelID == nil || *cfg.LeaveChannelID == "" {
		return
	}
	text := defaultLeaveMessage
	if cfg.LeaveMessage != nil && *cfg.LeaveMessage != "" {
		text = *cfg.LeaveMessage
	}
	sendWelcome(s, m.GuildID, *cfg.LeaveChannelID, renderWelcome(s, text, m.GuildID, m.User), cfg.UseEmbed, "Goodbye")
}

func sendWelcome(s *discordgo.Session, guildID, channelID, text string, useEmbed bool, title string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if useEmbed {
		e := embed.New(s, guildID).Title(title).Description(text).Build()
		_, _ = s.ChannelMessageSendEmbed(channelID, e)
		return
	}
	_, _ = s.ChannelMessageSend(channelID, text)
}

// renderWelcome substitutes placeholders in a welcome/goodbye template.
func renderWelcome(s *discordgo.Session, tmpl, guildID string, u *discordgo.User) string {
	server := "the server"
	count := 0
	if g, err := s.State.Guild(guildID); err == nil && g != nil {
		server = g.Name
		count = g.MemberCount
	}
	repl := strings.NewReplacer(
		"{user}", "<@"+u.ID+">",
		"{mention}", "<@"+u.ID+">",
		"{username}", u.Username,
		"{tag}", userTag(u),
		"{server}", server,
		"{guild}", server,
		"{membercount}", strconv.Itoa(count),
		"{memberCount}", strconv.Itoa(count),
		"{id}", u.ID,
	)
	return repl.Replace(tmpl)
}

// applyAutorole assigns the configured join roles to a new member (separate
// role sets for humans and bots).
func (h *Handlers) applyAutorole(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.User == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := h.deps.Store.GetAutoroleConfig(ctx, m.GuildID)
	if err != nil || !cfg.Enabled {
		return
	}
	roles := cfg.RoleIDs
	if m.User.Bot {
		roles = cfg.BotRoleIDs
	}
	for _, roleID := range roles {
		if roleID == "" {
			continue
		}
		if err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, roleID); err != nil {
			log.Warn().Err(err).Str("guild", m.GuildID).Str("role", roleID).Msg("autorole: add role")
		}
	}
}
