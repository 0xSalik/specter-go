// Package events registers all gateway event handlers and routes them to the
// appropriate subsystems (modlog, levels, automod, reaction roles, JTC).
package events

import (
	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/core"
)

// Handlers bundles the dependency container for event callbacks.
type Handlers struct {
	deps *core.Deps
}

// Register attaches all event handlers to the session.
func Register(s *discordgo.Session, deps *core.Deps) *Handlers {
	h := &Handlers{deps: deps}

	s.AddHandler(h.onGuildCreate)
	s.AddHandler(h.onGuildDelete)
	s.AddHandler(h.onMessageCreate)
	s.AddHandler(h.onMessageDelete)
	s.AddHandler(h.onMessageUpdate)
	s.AddHandler(h.onGuildMemberAdd)
	s.AddHandler(h.onGuildMemberRemove)
	s.AddHandler(h.onGuildMemberUpdate)
	s.AddHandler(h.onGuildUpdate)
	s.AddHandler(h.onChannelCreate)
	s.AddHandler(h.onChannelDelete)
	s.AddHandler(h.onChannelUpdate)
	s.AddHandler(h.onReactionAdd)
	s.AddHandler(h.onReactionRemove)
	s.AddHandler(h.onVoiceStateUpdate)

	return h
}
