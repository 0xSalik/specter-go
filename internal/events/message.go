package events

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/salik/specter/internal/automod"
	"github.com/salik/specter/internal/db"
	"github.com/salik/specter/internal/embed"
	"github.com/salik/specter/internal/modlog"
)

func (h *Handlers) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot || m.GuildID == "" {
		return
	}

	h.deps.MessageCache.Put(m.Message)

	h.handleAFK(s, m)

	if deleted := h.applyAutomod(s, m); deleted {
		return
	}

	h.deps.Levels.HandleMessage(s, m)
}

// handleAFK clears the author's AFK status and notifies on AFK mentions.
func (h *Handlers) handleAFK(s *discordgo.Session, m *discordgo.MessageCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if entry, err := h.deps.Store.GetAFK(ctx, m.GuildID, m.Author.ID); err == nil {
		if ok, _ := h.deps.Store.ClearAFK(ctx, m.GuildID, m.Author.ID); ok {
			dur := time.Since(entry.SetAt).Round(time.Second)
			e := embed.New(s, m.GuildID).Title("Welcome back").
				Description(fmt.Sprintf("Your AFK status has been removed. You were away for %s.", dur)).AsSuccess().Build()
			if msg, err := s.ChannelMessageSendEmbed(m.ChannelID, e); err == nil {
				go deleteAfter(s, m.ChannelID, msg.ID, 8*time.Second)
			}
		}
	} else if !db.IsNotFound(err) {
		log.Error().Err(err).Msg("afk: lookup")
	}

	for _, u := range m.Mentions {
		if entry, err := h.deps.Store.GetAFK(ctx, m.GuildID, u.ID); err == nil {
			e := embed.New(s, m.GuildID).Title("User is AFK").
				Description(fmt.Sprintf("<@%s> is AFK: %s", u.ID, entry.Reason)).Build()
			_, _ = s.ChannelMessageSendEmbed(m.ChannelID, e)
		}
	}
}

// applyAutomod evaluates automod rules and takes action. Returns true if the
// message was deleted (so XP should be skipped).
func (h *Handlers) applyAutomod(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := h.deps.Store.GetAutomodConfig(ctx, m.GuildID)
	if err != nil || !cfg.Enabled {
		return false
	}

	var roles []string
	if m.Member != nil {
		roles = m.Member.Roles
	}
	key := m.GuildID + ":" + m.Author.ID + ":" + m.ChannelID
	v := h.deps.Automod.Evaluate(cfg, key, m.Content, roles, m.ChannelID)
	if v == nil {
		return false
	}

	deleted := false
	switch v.Action {
	case "delete", "warn", "timeout", "kick", "ban":
		if err := s.ChannelMessageDelete(m.ChannelID, m.ID); err == nil {
			deleted = true
		}
	}

	switch v.Action {
	case "warn":
		_, _ = h.deps.Store.AddWarning(ctx, m.GuildID, m.Author.ID, "System", "Automod: "+v.Reason)
	case "timeout":
		until := time.Now().Add(10 * time.Minute)
		_ = s.GuildMemberTimeout(m.GuildID, m.Author.ID, &until)
	case "kick":
		_ = s.GuildMemberDeleteWithReason(m.GuildID, m.Author.ID, "Automod: "+v.Reason)
	case "ban":
		_ = s.GuildBanCreateWithReason(m.GuildID, m.Author.ID, "Automod: "+v.Reason, 0)
	}

	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:    m.GuildID,
		EventType:  modlog.EventAutomod,
		ActorID:    "System",
		TargetID:   m.Author.ID,
		TargetName: m.Author.Username,
		Reason:     v.Reason,
		Extra:      map[string]string{"Rule": v.Rule, "Action": v.Action},
		Timestamp:  time.Now(),
	})
	return deleted
}

func (h *Handlers) onMessageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	if m.GuildID == "" {
		return
	}
	content := "*unknown (not cached)*"
	author := ""
	if cached, ok := h.deps.MessageCache.Get(m.ID); ok {
		if cached.Content != "" {
			content = cached.Content
		}
		author = cached.AuthorID
	}
	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:   m.GuildID,
		EventType: modlog.EventMessageDelete,
		ActorID:   author,
		Extra:     map[string]string{"Channel": "<#" + m.ChannelID + ">", "Content": truncate(content, 1000)},
		Timestamp: time.Now(),
	})
}

func (h *Handlers) onMessageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if m.GuildID == "" || m.Message == nil || m.Author == nil || m.Author.Bot {
		return
	}
	before := "*unknown (not cached)*"
	if cached, ok := h.deps.MessageCache.Get(m.ID); ok && cached.Content != "" {
		before = cached.Content
	}
	// Refresh the cache with the new content.
	h.deps.MessageCache.Put(m.Message)

	if before == m.Content {
		return
	}
	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:    m.GuildID,
		EventType:  modlog.EventMessageEdit,
		ActorID:    m.Author.ID,
		TargetName: m.Author.Username,
		Extra: map[string]string{
			"Channel": "<#" + m.ChannelID + ">",
			"Before":  truncate(before, 500),
			"After":   truncate(m.Content, 500),
		},
		Timestamp: time.Now(),
	})
}

func deleteAfter(s *discordgo.Session, channelID, messageID string, d time.Duration) {
	time.Sleep(d)
	_ = s.ChannelMessageDelete(channelID, messageID)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

var _ = automod.Violation{}
