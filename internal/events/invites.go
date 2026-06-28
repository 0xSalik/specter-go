package events

import "github.com/bwmarrin/discordgo"

func (h *Handlers) onInviteCreate(s *discordgo.Session, e *discordgo.InviteCreate) {
	h.deps.Invites.AddInvite(
		e.GuildID,
		e.Code,
		e.Uses,
		e.MaxUses,
		e.Temporary,
		e.ChannelID,
		e.Inviter,
	)
}

func (h *Handlers) onInviteDelete(s *discordgo.Session, e *discordgo.InviteDelete) {
	h.deps.Invites.RemoveInvite(e.GuildID, e.Code)
}
