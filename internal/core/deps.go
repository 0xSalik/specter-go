// Package core provides the shared dependency container, per-interaction
// context, response helpers, and the slash-command router used by every command
// handler. Handler subpackages depend on core; core never imports them.
package core

import (
	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/access"
	"github.com/salik/specter/internal/automod"
	"github.com/salik/specter/internal/config"
	"github.com/salik/specter/internal/db/queries"
	"github.com/salik/specter/internal/levels"
	"github.com/salik/specter/internal/modlog"
	"github.com/salik/specter/internal/music"
	"github.com/salik/specter/internal/reactionroles"
	"github.com/salik/specter/internal/voice"
)

// Deps is the dependency container shared across all command handlers and
// event handlers. It is constructed once at startup.
type Deps struct {
	Session      *discordgo.Session
	Store        *queries.Store
	Gate         *access.Gate
	Modlog       *modlog.Logger
	MessageCache *modlog.MessageCache
	Music        *music.Manager
	Levels       *levels.Engine
	Automod      *automod.Checker
	ReactionRoles *reactionroles.Handler
	JTC          *voice.Manager
	Config       *config.Config
}
