package core

import (
	"bytes"
	"io"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

// HandlerFunc handles a single slash-command interaction.
type HandlerFunc func(c *Context)

// ComponentHandlerFunc handles a message-component interaction (buttons, menus).
// The customID is the raw component custom_id string.
type ComponentHandlerFunc func(c *Context, customID string)

// Command bundles a slash-command definition with its routing metadata.
type Command struct {
	Def          *discordgo.ApplicationCommand
	Group        string // access-control command group, e.g. "moderation"
	RequiredPerm int64  // built-in Discord permission required (0 = none)
	Handler      HandlerFunc
}

// Router dispatches interactions to registered handlers with panic recovery and
// access-control enforcement.
type Router struct {
	deps       *Deps
	commands   map[string]Command
	components []componentRoute // prefix-matched component handlers
}

type componentRoute struct {
	prefix  string
	handler ComponentHandlerFunc
}

// NewRouter constructs an empty router bound to deps.
func NewRouter(deps *Deps) *Router {
	return &Router{deps: deps, commands: make(map[string]Command)}
}

// Register adds a command to the router.
func (r *Router) Register(cmd Command) {
	if cmd.Def == nil || cmd.Handler == nil {
		log.Warn().Msg("router: ignoring command with nil definition or handler")
		return
	}
	r.commands[cmd.Def.Name] = cmd
}

// RegisterComponent registers a handler matched by custom_id prefix (the prefix
// is the text before the first ':' separator, e.g. "leaderboard").
func (r *Router) RegisterComponent(prefix string, h ComponentHandlerFunc) {
	r.components = append(r.components, componentRoute{prefix: prefix, handler: h})
}

// Definitions returns all registered command definitions for bulk registration.
func (r *Router) Definitions() []*discordgo.ApplicationCommand {
	defs := make([]*discordgo.ApplicationCommand, 0, len(r.commands))
	for _, c := range r.commands {
		defs = append(defs, c.Def)
	}
	return defs
}

// Handle is the discordgo InteractionCreate callback. It routes both
// application commands and message components.
func (r *Router) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		r.handleCommand(i)
	case discordgo.InteractionMessageComponent:
		r.handleComponent(i)
	}
}

func (r *Router) handleCommand(i *discordgo.InteractionCreate) {
	name := i.ApplicationCommandData().Name
	cmd, ok := r.commands[name]
	if !ok {
		return
	}

	c := newContext(r.deps, i)

	defer func() {
		if rec := recover(); rec != nil {
			log.Error().Interface("panic", rec).Str("command", name).
				Str("guild", c.GuildID).Str("user", c.UserID).Msg("recovered from handler panic")
			_ = c.Errorf("An unexpected error occurred while processing your command.", nil)
		}
	}()

	if i.GuildID != "" && r.deps.Gate != nil {
		if allowed, reason := r.deps.Gate.Check(i, cmd.Group, cmd.RequiredPerm); !allowed {
			_ = c.ReplyEphemeral(c.Embed().Title("Access Denied").Description(reason).AsError().Build())
			return
		}
	}

	cmd.Handler(c)
}

func (r *Router) handleComponent(i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID
	c := newContext(r.deps, i)

	defer func() {
		if rec := recover(); rec != nil {
			log.Error().Interface("panic", rec).Str("custom_id", customID).Msg("recovered from component panic")
		}
	}()

	for _, route := range r.components {
		if hasPrefix(customID, route.prefix) {
			route.handler(c, customID)
			return
		}
	}
}

func hasPrefix(customID, prefix string) bool {
	if len(customID) < len(prefix) {
		return false
	}
	return customID[:len(prefix)] == prefix
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
