package events

import "github.com/bwmarrin/discordgo"

func (h *Handlers) onReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.GuildID == "" {
		return
	}
	h.deps.ReactionRoles.HandleAdd(s, r)
}

func (h *Handlers) onReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	if r.GuildID == "" {
		return
	}
	h.deps.ReactionRoles.HandleRemove(s, r)
}

func (h *Handlers) onVoiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	if v.GuildID == "" {
		return
	}
	h.deps.JTC.HandleVoiceUpdate(s, v)
}
