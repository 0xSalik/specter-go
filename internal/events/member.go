package events

import (
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/modlog"
)

func (h *Handlers) onGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.User == nil {
		return
	}
	created, _ := discordgo.SnowflakeTimestamp(m.User.ID)
	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:    m.GuildID,
		EventType:  modlog.EventMemberJoin,
		TargetID:   m.User.ID,
		TargetName: m.User.Username,
		Extra:      map[string]string{"Account Created": created.Format("2006-01-02")},
		Timestamp:  time.Now(),
	})
}

func (h *Handlers) onGuildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m.User == nil {
		return
	}
	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:    m.GuildID,
		EventType:  modlog.EventMemberLeave,
		TargetID:   m.User.ID,
		TargetName: m.User.Username,
		Timestamp:  time.Now(),
	})
}

func (h *Handlers) onGuildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.Member == nil || m.User == nil {
		return
	}
	extra := map[string]string{}
	if m.BeforeUpdate != nil {
		if m.BeforeUpdate.Nick != m.Nick {
			extra["Nickname"] = display(m.BeforeUpdate.Nick) + " → " + display(m.Nick)
		}
		if len(m.BeforeUpdate.Roles) != len(m.Roles) {
			extra["Roles"] = "Roles updated"
		}
	}
	if len(extra) == 0 {
		return
	}
	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:    m.GuildID,
		EventType:  modlog.EventMemberUpdate,
		TargetID:   m.User.ID,
		TargetName: m.User.Username,
		Extra:      extra,
		Timestamp:  time.Now(),
	})
}

func display(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
