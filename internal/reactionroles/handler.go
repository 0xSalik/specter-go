// Package reactionroles applies role changes in response to reaction add/remove
// events according to the menu type (normal, unique, verify, reverse).
package reactionroles

import (
	"context"
	"regexp"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/salik/specter/internal/db"
	"github.com/salik/specter/internal/db/queries"
)

// Handler resolves reaction events to role mutations.
type Handler struct {
	store *queries.Store
}

// New constructs a Handler.
func New(store *queries.Store) *Handler {
	return &Handler{store: store}
}

var customEmojiRe = regexp.MustCompile(`^<a?:([a-zA-Z0-9_]+):(\d+)>$`)

// NormalizeEmoji converts a raw emoji input (unicode or "<:name:id>") into the
// API name form used for matching and for adding reactions.
func NormalizeEmoji(raw string) string {
	if m := customEmojiRe.FindStringSubmatch(raw); m != nil {
		return m[1] + ":" + m[2]
	}
	return raw
}

// emojiKey extracts the API name from a reaction event emoji.
func emojiKey(e discordgo.Emoji) string {
	if e.ID != "" {
		return e.Name + ":" + e.ID
	}
	return e.Name
}

// HandleAdd processes a reaction add event.
func (h *Handler) HandleAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.UserID == s.State.User.ID {
		return
	}
	menu, entry := h.lookup(r.MessageID, emojiKey(r.Emoji))
	if menu == nil || entry == nil {
		return
	}

	switch menu.Type {
	case "verify":
		h.addRole(s, r.GuildID, r.UserID, entry.RoleID)
	case "unique":
		h.applyUnique(s, menu, r.GuildID, r.UserID, entry.RoleID)
	case "reverse":
		if h.hasRole(s, r.GuildID, r.UserID, entry.RoleID) {
			h.removeRole(s, r.GuildID, r.UserID, entry.RoleID)
		} else {
			h.addRole(s, r.GuildID, r.UserID, entry.RoleID)
		}
	default: // normal
		h.addRole(s, r.GuildID, r.UserID, entry.RoleID)
	}
}

// HandleRemove processes a reaction remove event.
func (h *Handler) HandleRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	if r.UserID == s.State.User.ID {
		return
	}
	menu, entry := h.lookup(r.MessageID, emojiKey(r.Emoji))
	if menu == nil || entry == nil {
		return
	}
	switch menu.Type {
	case "verify", "reverse":
		// One-way grants; removal does nothing.
		return
	default: // normal, unique
		h.removeRole(s, r.GuildID, r.UserID, entry.RoleID)
	}
}

func (h *Handler) lookup(messageID, emoji string) (*queries.ReactionRoleMenu, *queries.ReactionRoleEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	menu, err := h.store.GetMenuByMessage(ctx, messageID)
	if err != nil {
		if !db.IsNotFound(err) {
			log.Error().Err(err).Msg("reactionroles: lookup menu")
		}
		return nil, nil
	}
	entries, err := h.store.ListEntries(ctx, menu.ID)
	if err != nil {
		log.Error().Err(err).Msg("reactionroles: list entries")
		return nil, nil
	}
	for i := range entries {
		if entries[i].Emoji == emoji {
			return menu, &entries[i]
		}
	}
	return menu, nil
}

func (h *Handler) applyUnique(s *discordgo.Session, menu *queries.ReactionRoleMenu, guildID, userID, roleID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entries, err := h.store.ListEntries(ctx, menu.ID)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.RoleID != roleID {
			h.removeRole(s, guildID, userID, e.RoleID)
		}
	}
	h.addRole(s, guildID, userID, roleID)
}

func (h *Handler) hasRole(s *discordgo.Session, guildID, userID, roleID string) bool {
	m, err := s.GuildMember(guildID, userID)
	if err != nil {
		return false
	}
	for _, r := range m.Roles {
		if r == roleID {
			return true
		}
	}
	return false
}

func (h *Handler) addRole(s *discordgo.Session, guildID, userID, roleID string) {
	if err := s.GuildMemberRoleAdd(guildID, userID, roleID); err != nil {
		log.Warn().Err(err).Str("role", roleID).Str("user", userID).Msg("reactionroles: add role")
	}
}

func (h *Handler) removeRole(s *discordgo.Session, guildID, userID, roleID string) {
	if err := s.GuildMemberRoleRemove(guildID, userID, roleID); err != nil {
		log.Warn().Err(err).Str("role", roleID).Str("user", userID).Msg("reactionroles: remove role")
	}
}
