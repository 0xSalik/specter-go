package events

import (
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/modlog"
)

func (h *Handlers) onChannelCreate(s *discordgo.Session, c *discordgo.ChannelCreate) {
	if c.GuildID == "" {
		return
	}
	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID: c.GuildID, EventType: modlog.EventChannelUpdate, ActorID: "System",
		Extra: map[string]string{"Action": "Created", "Channel": c.Name}, Timestamp: time.Now(),
	})
}

func (h *Handlers) onChannelDelete(s *discordgo.Session, c *discordgo.ChannelDelete) {
	if c.GuildID == "" {
		return
	}
	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID: c.GuildID, EventType: modlog.EventChannelUpdate, ActorID: "System",
		Extra: map[string]string{"Action": "Deleted", "Channel": c.Name}, Timestamp: time.Now(),
	})
}

func (h *Handlers) onChannelUpdate(s *discordgo.Session, c *discordgo.ChannelUpdate) {
	if c.GuildID == "" {
		return
	}
	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID: c.GuildID, EventType: modlog.EventChannelUpdate, ActorID: "System",
		Extra: map[string]string{"Action": "Updated", "Channel": c.Name}, Timestamp: time.Now(),
	})
}

func (h *Handlers) onGuildUpdate(s *discordgo.Session, g *discordgo.GuildUpdate) {
	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID: g.ID, EventType: modlog.EventGuildUpdate, ActorID: "System",
		Extra: map[string]string{"Name": g.Name}, Timestamp: time.Now(),
	})
}
