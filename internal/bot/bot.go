// Package bot wires together every subsystem: the Discord session, the database
// store, the command router and event handlers, and the dashboard server.
package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/salik/specter/internal/access"
	"github.com/salik/specter/internal/automod"
	"github.com/salik/specter/internal/config"
	"github.com/salik/specter/internal/core"
	"github.com/salik/specter/internal/dashboard"
	"github.com/salik/specter/internal/db/queries"
	"github.com/salik/specter/internal/embed"
	"github.com/salik/specter/internal/events"
	"github.com/salik/specter/internal/levels"
	"github.com/salik/specter/internal/modlog"
	"github.com/salik/specter/internal/music"
	"github.com/salik/specter/internal/reactionroles"
	"github.com/salik/specter/internal/voice"

	cmdfun "github.com/salik/specter/internal/commands/fun"
	cmdlevels "github.com/salik/specter/internal/commands/levels"
	cmdmod "github.com/salik/specter/internal/commands/moderation"
	cmdmusic "github.com/salik/specter/internal/commands/music"
	cmdrr "github.com/salik/specter/internal/commands/reactionroles"
	cmdsystem "github.com/salik/specter/internal/commands/system"
	cmduser "github.com/salik/specter/internal/commands/user"
	cmdutils "github.com/salik/specter/internal/commands/utils"
	cmdvoice "github.com/salik/specter/internal/commands/voice"
)

// Bot is the top-level application.
type Bot struct {
	cfg       *config.Config
	session   *discordgo.Session
	store     *queries.Store
	deps      *core.Deps
	router    *core.Router
	dashboard *dashboard.Server
}

// New constructs the bot, its dependencies, and registers all commands/events.
func New(cfg *config.Config, store *queries.Store) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMembers |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsGuildMessageReactions |
		discordgo.IntentMessageContent |
		discordgo.IntentsDirectMessages

	embed.Init(store)

	deps := &core.Deps{
		Session:       session,
		Store:         store,
		Gate:          access.NewGate(store),
		Modlog:        modlog.New(store),
		MessageCache:  modlog.NewMessageCache(),
		Music:         music.NewManager(session, cfg.YTDLPPath, "ffmpeg", "dca"),
		Levels:        levels.NewEngine(store),
		Automod:       automod.NewChecker(),
		ReactionRoles: reactionroles.New(store),
		JTC:           voice.New(store),
		Config:        cfg,
	}

	router := core.NewRouter(deps)
	registerCommands(router)

	session.AddHandler(router.Handle)
	events.Register(session, deps)

	dash, err := dashboard.New(cfg, store, session)
	if err != nil {
		return nil, err
	}

	return &Bot{cfg: cfg, session: session, store: store, deps: deps, router: router, dashboard: dash}, nil
}

func registerCommands(r *core.Router) {
	cmdmod.Register(r)
	cmdlevels.Register(r)
	cmdmusic.Register(r)
	cmdrr.Register(r)
	cmdvoice.Register(r)
	cmdfun.Register(r)
	cmduser.Register(r)
	cmdsystem.Register(r)
	cmdutils.Register(r)
}

// Open connects to the gateway, registers slash commands, performs startup
// maintenance, and starts the dashboard.
func (b *Bot) Open() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("open gateway: %w", err)
	}
	log.Info().Str("user", b.session.State.User.String()).Msg("connected to Discord")

	appID := b.session.State.User.ID
	defs := b.router.Definitions()
	if _, err := b.session.ApplicationCommandBulkOverwrite(appID, b.cfg.DevGuildID, defs); err != nil {
		return fmt.Errorf("register slash commands: %w", err)
	}
	scope := "globally"
	if b.cfg.DevGuildID != "" {
		scope = "to dev guild " + b.cfg.DevGuildID
	}
	log.Info().Int("count", len(defs)).Msgf("registered slash commands %s", scope)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		b.deps.JTC.CleanupStale(ctx, b.session)
	}()

	go func() {
		if err := b.dashboard.Start(); err != nil {
			log.Error().Err(err).Msg("dashboard server stopped")
		}
	}()

	return nil
}

// Close gracefully shuts down all subsystems.
func (b *Bot) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	b.dashboard.Shutdown(ctx)
	b.deps.Music.Shutdown(ctx)
	b.deps.Automod.Close()
	if err := b.session.Close(); err != nil {
		log.Error().Err(err).Msg("error closing discord session")
	}
}
