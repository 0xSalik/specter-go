package events

import (
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/modlog"
)

func (h *Handlers) onReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.GuildID == "" {
		return
	}
	h.deps.ReactionRoles.HandleAdd(s, r)
	h.deps.Starboard.HandleReactionAdd(s, r)
}

func (h *Handlers) onReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	if r.GuildID == "" {
		return
	}
	h.deps.ReactionRoles.HandleRemove(s, r)
	h.deps.Starboard.HandleReactionRemove(s, r)
}

func (h *Handlers) onVoiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	if v.GuildID == "" {
		return
	}
	h.deps.JTC.HandleVoiceUpdate(s, v)
	h.logVoiceActivity(s, v)
}

// logVoiceActivity records channel joins, leaves, moves, server mute/deafen and
// stream toggles to the general log. Self mute/deafen are intentionally ignored
// to avoid spamming the log.
func (h *Handlers) logVoiceActivity(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	beforeCh := ""
	if v.BeforeUpdate != nil {
		beforeCh = v.BeforeUpdate.ChannelID
	}
	afterCh := v.ChannelID

	name, thumb := h.voiceActor(s, v)
	extra := map[string]string{}
	var description string

	switch {
	case beforeCh == "" && afterCh != "":
		description = "joined a voice channel"
		extra["Channel"] = "<#" + afterCh + ">"
	case beforeCh != "" && afterCh == "":
		description = "left a voice channel"
		extra["Channel"] = "<#" + beforeCh + ">"
	case beforeCh != afterCh:
		description = "moved voice channels"
		extra["From"] = "<#" + beforeCh + ">"
		extra["To"] = "<#" + afterCh + ">"
	default:
		// Same channel: report server-side mute/deafen and streaming changes only.
		if v.BeforeUpdate == nil {
			return
		}
		extra["Channel"] = "<#" + afterCh + ">"
		switch {
		case v.BeforeUpdate.Mute != v.Mute:
			description = onOffVerb("was server-muted", "was server-unmuted", v.Mute)
		case v.BeforeUpdate.Deaf != v.Deaf:
			description = onOffVerb("was server-deafened", "was server-undeafened", v.Deaf)
		case v.BeforeUpdate.SelfStream != v.SelfStream:
			description = onOffVerb("started streaming", "stopped streaming", v.SelfStream)
		case v.BeforeUpdate.SelfVideo != v.SelfVideo:
			description = onOffVerb("turned their camera on", "turned their camera off", v.SelfVideo)
		default:
			return
		}
	}

	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:    v.GuildID,
		EventType:  modlog.EventVoiceState,
		TargetID:   v.UserID,
		TargetName: name,
		Reason:     description,
		Thumbnail:  thumb,
		Extra:      extra,
		Timestamp:  time.Now(),
	})
}

func onOffVerb(on, off string, state bool) string {
	if state {
		return on
	}
	return off
}

// voiceActor resolves a display name and avatar for a voice state update.
func (h *Handlers) voiceActor(s *discordgo.Session, v *discordgo.VoiceStateUpdate) (name, thumb string) {
	if v.Member != nil && v.Member.User != nil {
		return userTag(v.Member.User), v.Member.User.AvatarURL("256")
	}
	if m, err := s.State.Member(v.GuildID, v.UserID); err == nil && m.User != nil {
		return userTag(m.User), m.User.AvatarURL("256")
	}
	if u, err := s.User(v.UserID); err == nil {
		return userTag(u), u.AvatarURL("256")
	}
	return v.UserID, ""
}
