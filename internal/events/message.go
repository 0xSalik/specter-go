package events

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/0xSalik/specter/internal/automod"
	"github.com/0xSalik/specter/internal/db"
	"github.com/0xSalik/specter/internal/embed"
	"github.com/0xSalik/specter/internal/modlog"
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
	extra := map[string]string{"Channel": "<#" + m.ChannelID + ">"}
	author := ""

	cached, ok := h.deps.MessageCache.Get(m.ID)
	switch {
	case !ok:
		extra["Content"] = "*Not cached (sent before Specter started or evicted).*"
	default:
		author = cached.AuthorID
		if cached.Content != "" {
			extra["Content"] = truncate(cached.Content, 1000)
		} else {
			extra["Content"] = "*(no text content)*"
		}
		if att := formatAttachments(cached.Attachments); att != "" {
			extra["Attachments"] = att
		}
		if cached.EmbedCount > 0 {
			extra["Embeds"] = fmt.Sprintf("%d embed(s) attached to the message", cached.EmbedCount)
		}
	}

	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:   m.GuildID,
		EventType: modlog.EventMessageDelete,
		ActorID:   author,
		Extra:     extra,
		Timestamp: time.Now(),
	})
}

func (h *Handlers) onMessageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if m.GuildID == "" || m.Message == nil || m.Author == nil || m.Author.Bot {
		return
	}

	var cached *modlog.CachedMessage
	if c, ok := h.deps.MessageCache.Get(m.ID); ok {
		cached = c
	}
	before := ""
	beforeAttachments := 0
	if cached != nil {
		before = cached.Content
		beforeAttachments = len(cached.Attachments)
	}

	// Detect attachment removals (the only attachment change an edit allows).
	removedAttachments := beforeAttachments - len(m.Attachments)

	// Refresh the cache with the new state.
	h.deps.MessageCache.Put(m.Message)

	// Ignore no-op updates (e.g. Discord adding a link-preview embed): nothing
	// the user actually changed.
	if before == m.Content && removedAttachments <= 0 {
		return
	}

	extra := map[string]string{
		"Channel": "<#" + m.ChannelID + ">",
		"Before":  orNone(truncate(before, 500), cached == nil),
		"After":   orEmpty(truncate(m.Content, 500)),
	}
	if removedAttachments > 0 {
		extra["Attachments Removed"] = fmt.Sprintf("%d", removedAttachments)
	}

	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:    m.GuildID,
		EventType:  modlog.EventMessageEdit,
		ActorID:    m.Author.ID,
		TargetName: m.Author.Username,
		Extra:      extra,
		Timestamp:  time.Now(),
	})
}

// FormatAttachmentsForTest exposes formatAttachments to tests.
func FormatAttachmentsForTest(atts []modlog.CachedAttachment) string { return formatAttachments(atts) }

// HumanizeBytesForTest exposes humanizeBytes to tests.
func HumanizeBytesForTest(n int) string { return humanizeBytes(n) }

// formatAttachments renders cached attachments as markdown links (best-effort,
// since the CDN URL may expire shortly after deletion) plus file size.
func formatAttachments(atts []modlog.CachedAttachment) string {
	if len(atts) == 0 {
		return ""
	}
	lines := make([]string, 0, len(atts))
	for i, a := range atts {
		if i >= 10 {
			lines = append(lines, fmt.Sprintf("…and %d more", len(atts)-i))
			break
		}
		name := a.Filename
		if name == "" {
			name = "attachment"
		}
		url := a.URL
		if url == "" {
			url = a.ProxyURL
		}
		entry := name + " (" + humanizeBytes(a.Size) + ")"
		if url != "" {
			entry = fmt.Sprintf("[%s](%s)", name, url) + " (" + humanizeBytes(a.Size) + ")"
		}
		lines = append(lines, entry)
	}
	return truncate(strings.Join(lines, "\n"), 1000)
}

func humanizeBytes(n int) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := int64(n) / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// orNone returns a placeholder when the value is empty, distinguishing an empty
// edit from an uncached original.
func orNone(s string, uncached bool) string {
	if s != "" {
		return s
	}
	if uncached {
		return "*Not cached*"
	}
	return "*(no text content)*"
}

func orEmpty(s string) string {
	if s == "" {
		return "*(no text content)*"
	}
	return s
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
