// Package voice implements the join-to-create (JTC) voice channel system:
// creating ephemeral channels when users join a trigger channel and cleaning
// them up when empty.
package voice

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/0xSalik/specter/internal/db"
	"github.com/0xSalik/specter/internal/db/queries"
)

// Manager owns join-to-create behavior.
type Manager struct {
	store *queries.Store
}

// New constructs a JTC Manager.
func New(store *queries.Store) *Manager {
	return &Manager{store: store}
}

// HandleVoiceUpdate reacts to a voice state change: creating channels on
// trigger-join and deleting empty JTC channels on leave.
func (m *Manager) HandleVoiceUpdate(s *discordgo.Session, e *discordgo.VoiceStateUpdate) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	cfg, err := m.store.GetJTCConfig(ctx, e.GuildID)
	if err != nil {
		log.Error().Err(err).Str("guild", e.GuildID).Msg("jtc: load config")
		return
	}

	newChannel := e.ChannelID
	oldChannel := ""
	if e.BeforeUpdate != nil {
		oldChannel = e.BeforeUpdate.ChannelID
	}

	if cfg.Enabled && cfg.TriggerChannel != nil && newChannel == *cfg.TriggerChannel && newChannel != "" {
		m.createChannel(ctx, s, cfg, e.GuildID, e.UserID)
	}

	if oldChannel != "" && oldChannel != newChannel {
		m.maybeCleanup(ctx, s, oldChannel)
	}
}

func (m *Manager) createChannel(ctx context.Context, s *discordgo.Session, cfg *queries.JTCConfig, guildID, userID string) {
	username := userID
	if member, err := s.State.Member(guildID, userID); err == nil && member.User != nil {
		username = member.User.Username
	} else if u, err := s.User(userID); err == nil {
		username = u.Username
	}

	name := RenderName(cfg.DefaultName, username)
	data := discordgo.GuildChannelCreateData{
		Name:      name,
		Type:      discordgo.ChannelTypeGuildVoice,
		UserLimit: cfg.DefaultLimit,
	}
	if cfg.CategoryID != nil && *cfg.CategoryID != "" {
		data.ParentID = *cfg.CategoryID
	}

	ch, err := s.GuildChannelCreateComplex(guildID, data)
	if err != nil {
		log.Error().Err(err).Str("guild", guildID).Msg("jtc: create channel")
		return
	}
	if err := s.GuildMemberMove(guildID, userID, &ch.ID); err != nil {
		log.Warn().Err(err).Msg("jtc: move member")
	}
	if err := m.store.AddJTCChannel(ctx, ch.ID, guildID, userID); err != nil {
		log.Error().Err(err).Msg("jtc: record channel")
	}
}

func (m *Manager) maybeCleanup(ctx context.Context, s *discordgo.Session, channelID string) {
	if _, err := m.store.GetJTCChannel(ctx, channelID); err != nil {
		return // not a JTC channel
	}
	if channelHasMembers(s, channelID) {
		return
	}
	if _, err := s.ChannelDelete(channelID); err != nil {
		log.Warn().Err(err).Str("channel", channelID).Msg("jtc: delete channel")
	}
	if err := m.store.RemoveJTCChannel(ctx, channelID); err != nil {
		log.Error().Err(err).Msg("jtc: remove record")
	}
}

func channelHasMembers(s *discordgo.Session, channelID string) bool {
	ch, err := s.State.Channel(channelID)
	if err != nil || ch == nil {
		return false
	}
	g, err := s.State.Guild(ch.GuildID)
	if err != nil || g == nil {
		return false
	}
	for _, vs := range g.VoiceStates {
		if vs.ChannelID == channelID {
			return true
		}
	}
	return false
}

// CleanupStale removes JTC records whose channels no longer exist (run at start).
func (m *Manager) CleanupStale(ctx context.Context, s *discordgo.Session) {
	channels, err := m.store.ListJTCChannels(ctx)
	if err != nil {
		log.Error().Err(err).Msg("jtc: list channels for cleanup")
		return
	}
	for _, c := range channels {
		if _, err := s.Channel(c.ChannelID); err != nil {
			if rmErr := m.store.RemoveJTCChannel(ctx, c.ChannelID); rmErr != nil {
				log.Error().Err(rmErr).Msg("jtc: remove stale record")
			}
		}
	}
}

// RenderName expands a JTC channel-name template against a username.
func RenderName(tmpl, username string) string {
	r := strings.NewReplacer("{username}", username, "{count}", "1", "{game}", "")
	name := strings.TrimSpace(r.Replace(tmpl))
	if name == "" {
		name = username + "'s Channel"
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}

var _ = db.ErrNotFound
