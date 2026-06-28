// Package starboard reposts messages that reach a configurable reaction
// threshold into a dedicated starboard channel, keeping the entry's star count
// in sync as reactions are added and removed.
package starboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/0xSalik/specter/internal/db"
	"github.com/0xSalik/specter/internal/db/queries"
	"github.com/0xSalik/specter/internal/embed"
)

// Handler evaluates star reactions against the per-guild starboard config.
type Handler struct {
	store *queries.Store
}

// New constructs a starboard handler.
func New(store *queries.Store) *Handler { return &Handler{store: store} }

// HandleReactionAdd is invoked for every reaction add in a guild.
func (h *Handler) HandleReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	h.sync(s, r.GuildID, r.ChannelID, r.MessageID, r.Emoji)
}

// HandleReactionRemove is invoked for every reaction remove in a guild.
func (h *Handler) HandleReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	h.sync(s, r.GuildID, r.ChannelID, r.MessageID, r.Emoji)
}

func (h *Handler) sync(s *discordgo.Session, guildID, channelID, messageID string, emoji discordgo.Emoji) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cfg, err := h.store.GetStarboardConfig(ctx, guildID)
	if err != nil || !cfg.Enabled || cfg.ChannelID == nil || *cfg.ChannelID == "" {
		return
	}
	// Never starboard messages already in the starboard channel.
	if channelID == *cfg.ChannelID {
		return
	}
	if !emojiMatches(emoji, cfg.Emoji) {
		return
	}

	msg, err := s.ChannelMessage(channelID, messageID)
	if err != nil {
		return
	}

	count := h.countStars(s, cfg, msg)
	entry, entryErr := h.store.GetStarboardEntry(ctx, guildID, messageID)
	hasEntry := entryErr == nil
	if entryErr != nil && !errors.Is(entryErr, db.ErrNotFound) {
		log.Warn().Err(entryErr).Str("guild", guildID).Msg("starboard: load entry")
		return
	}

	if count < cfg.Threshold {
		if hasEntry {
			_ = s.ChannelMessageDelete(*cfg.ChannelID, entry.StarboardMessageID)
			_ = h.store.DeleteStarboardEntry(ctx, guildID, messageID)
		}
		return
	}

	content := fmt.Sprintf("%s **%d** · <#%s>", cfg.Emoji, count, channelID)
	e := buildEntryEmbed(s, guildID, msg, channelID)

	if hasEntry {
		_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel: *cfg.ChannelID,
			ID:      entry.StarboardMessageID,
			Content: &content,
			Embeds:  &[]*discordgo.MessageEmbed{e},
		})
		if err != nil {
			// The starboard post was likely deleted manually; recreate it.
			if posted, perr := s.ChannelMessageSendComplex(*cfg.ChannelID, &discordgo.MessageSend{Content: content, Embed: e}); perr == nil {
				entry.StarboardMessageID = posted.ID
			}
		}
		entry.StarCount = count
		_ = h.store.UpsertStarboardEntry(ctx, entry)
		return
	}

	posted, err := s.ChannelMessageSendComplex(*cfg.ChannelID, &discordgo.MessageSend{Content: content, Embed: e})
	if err != nil {
		log.Warn().Err(err).Str("guild", guildID).Msg("starboard: post entry")
		return
	}
	_ = h.store.UpsertStarboardEntry(ctx, &queries.StarboardEntry{
		GuildID:            guildID,
		MessageID:          messageID,
		ChannelID:          channelID,
		StarboardMessageID: posted.ID,
		StarCount:          count,
	})
}

// countStars counts qualifying reactors for the starboard emoji: bots are never
// counted, and the message author is excluded unless self-starring is allowed.
func (h *Handler) countStars(s *discordgo.Session, cfg *queries.StarboardConfig, msg *discordgo.Message) int {
	var apiName string
	for _, react := range msg.Reactions {
		if react.Emoji != nil && emojiMatches(*react.Emoji, cfg.Emoji) {
			apiName = react.Emoji.APIName()
			break
		}
	}
	if apiName == "" {
		return 0
	}
	users, err := s.MessageReactions(msg.ChannelID, msg.ID, apiName, 100, "", "")
	if err != nil {
		return 0
	}
	count := 0
	for _, u := range users {
		if u.Bot {
			continue
		}
		if !cfg.SelfStar && msg.Author != nil && u.ID == msg.Author.ID {
			continue
		}
		count++
	}
	return count
}

func buildEntryEmbed(s *discordgo.Session, guildID string, msg *discordgo.Message, channelID string) *discordgo.MessageEmbed {
	b := embed.New(s, guildID)
	if msg.Author != nil {
		b.Author(msg.Author.Username, msg.Author.AvatarURL("128"))
	}
	desc := msg.Content
	if strings.TrimSpace(desc) == "" {
		desc = "*(no text content)*"
	}
	b.Description(desc)
	b.Field("Source", fmt.Sprintf("[Jump to message](https://discord.com/channels/%s/%s/%s)", guildID, channelID, msg.ID), false)

	// Attach the first image attachment, if any, as the embed image.
	for _, a := range msg.Attachments {
		if a.Width > 0 && a.Height > 0 {
			b.Image(a.URL)
			break
		}
	}
	b.Timestamp()
	return b.Build()
}

// emojiMatches reports whether a reacted emoji corresponds to the configured
// starboard emoji (unicode by name, custom by name or API name).
func emojiMatches(e discordgo.Emoji, configured string) bool {
	if configured == "" {
		return false
	}
	configured = strings.TrimSpace(configured)
	if e.Name == configured || e.APIName() == configured {
		return true
	}
	// Allow a custom emoji configured as <:name:id> or <a:name:id>.
	if e.ID != "" && strings.Contains(configured, e.ID) {
		return true
	}
	return false
}
