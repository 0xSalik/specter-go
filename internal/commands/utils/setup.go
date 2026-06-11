// Package utils implements administrative configuration commands: /setup and
// /modlogs.
package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/core"
	"github.com/salik/specter/internal/embed"
	"github.com/salik/specter/internal/guildsetup"
)

const group = "system"

// Register wires /setup and /modlogs into the router.
func Register(r *core.Router) {
	registerSetup(r)
	registerModlogs(r)
}

func registerSetup(r *core.Router) {
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageServer, Handler: handleSetup,
		Def: &discordgo.ApplicationCommand{
			Name: "setup", Description: "Configure Specter for this server",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "color", Description: "Set the embed accent color",
					Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "hex", Description: "#RRGGBB", Required: true}}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "modlogs", Description: "Create or repair the log channels"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "levels", Description: "Configure the level system",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionBoolean, Name: "enabled", Description: "Enable leveling", Required: false},
						{Type: discordgo.ApplicationCommandOptionChannel, Name: "announce_channel", Description: "Level-up announce channel", Required: false},
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "xp_min", Description: "Minimum XP per message", Required: false, MinValue: ptrFloat(1)},
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "xp_max", Description: "Maximum XP per message", Required: false, MinValue: ptrFloat(1)},
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "cooldown", Description: "Cooldown seconds", Required: false, MinValue: ptrFloat(0)},
					}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "voice", Description: "Configure join-to-create",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionBoolean, Name: "enabled", Description: "Enable JTC", Required: false},
						{Type: discordgo.ApplicationCommandOptionChannel, Name: "trigger_channel", Description: "Trigger voice channel", Required: false},
						{Type: discordgo.ApplicationCommandOptionChannel, Name: "category", Description: "Category for new channels", Required: false},
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "default_limit", Description: "Default user limit", Required: false, MinValue: ptrFloat(0), MaxValue: 99},
					}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "fun", Description: "Show fun command availability"},
			},
		},
	})
}

func handleSetup(c *core.Context) {
	switch c.SubCommand {
	case "color":
		setupColor(c)
	case "modlogs":
		setupModlogs(c)
	case "levels":
		setupLevels(c)
	case "voice":
		setupVoice(c)
	case "fun":
		_ = c.Reply(c.Embed().Title("Fun Commands").Description("All fun commands are enabled by default and gated only by access control. Use `/access` rules or the dashboard to restrict them.").Build())
	default:
		_ = c.Errorf("Unknown setup subcommand.", nil)
	}
}

func setupColor(c *core.Context) {
	hex := c.StringOpt("hex", "")
	if !embed.ValidHexColor(hex) {
		_ = c.Errorf("Invalid hex color. Use the format #RRGGBB, e.g. #5865F2.", nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Store.SetEmbedColor(ctx, c.GuildID, hex); err != nil {
		_ = c.Errorf("Failed to update the embed color.", err)
		return
	}
	embed.Invalidate(c.GuildID)
	_ = c.Reply(c.Embed().Title("Color Updated").Description("Embed accent color set to "+hex+".").AsSuccess().Build())
}

func setupModlogs(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	res, err := guildsetup.EnsureLogInfrastructure(ctx, c.Session, c.Store, c.GuildID)
	if err != nil {
		_ = c.Errorf("Failed to provision log channels.", err)
		return
	}
	b := c.Embed().Title("Log Channels").AsSuccess()
	if !res.Created {
		b.Description("Log channels already exist for this server.")
	} else {
		b.Description("Log infrastructure has been created.")
	}
	if len(res.Failed) > 0 {
		b.Field("Could not create", fmt.Sprintf("%v", res.Failed), false).AsError()
	}
	_ = c.Reply(b.Build())
}

func setupLevels(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := c.Store.GetLevelConfig(ctx, c.GuildID)
	if err != nil {
		_ = c.Errorf("Failed to load level configuration.", err)
		return
	}
	if c.HasOpt("enabled") {
		cfg.Enabled = c.BoolOpt("enabled", cfg.Enabled)
	}
	if ch := c.ChannelOpt("announce_channel"); ch != nil {
		cfg.AnnounceChannelID = &ch.ID
	}
	if c.HasOpt("xp_min") {
		cfg.XPMin = c.IntOpt("xp_min", cfg.XPMin)
	}
	if c.HasOpt("xp_max") {
		cfg.XPMax = c.IntOpt("xp_max", cfg.XPMax)
	}
	if c.HasOpt("cooldown") {
		cfg.XPCooldownSecs = c.IntOpt("cooldown", cfg.XPCooldownSecs)
	}
	if cfg.XPMax < cfg.XPMin {
		_ = c.Errorf("xp_max cannot be less than xp_min.", nil)
		return
	}
	if err := c.Store.UpsertLevelConfig(ctx, cfg); err != nil {
		_ = c.Errorf("Failed to save level configuration.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Level System Updated").AsSuccess().
		Field("Enabled", boolStr(cfg.Enabled), true).
		Field("XP Range", fmt.Sprintf("%d-%d", cfg.XPMin, cfg.XPMax), true).
		Field("Cooldown", fmt.Sprintf("%ds", cfg.XPCooldownSecs), true).Build())
}

func setupVoice(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := c.Store.GetJTCConfig(ctx, c.GuildID)
	if err != nil {
		_ = c.Errorf("Failed to load voice configuration.", err)
		return
	}
	if c.HasOpt("enabled") {
		cfg.Enabled = c.BoolOpt("enabled", cfg.Enabled)
	}
	if ch := c.ChannelOpt("trigger_channel"); ch != nil {
		cfg.TriggerChannel = &ch.ID
	}
	if ch := c.ChannelOpt("category"); ch != nil {
		cfg.CategoryID = &ch.ID
	}
	if c.HasOpt("default_limit") {
		cfg.DefaultLimit = c.IntOpt("default_limit", cfg.DefaultLimit)
	}
	if err := c.Store.UpsertJTCConfig(ctx, cfg); err != nil {
		_ = c.Errorf("Failed to save voice configuration.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Join-to-Create Updated").AsSuccess().
		Field("Enabled", boolStr(cfg.Enabled), true).
		Field("Default Limit", fmt.Sprintf("%d", cfg.DefaultLimit), true).Build())
}

func ptrFloat(f float64) *float64 { return &f }

func boolStr(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
