package server

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
	"github.com/0xSalik/specter/internal/db/queries"
)

func registerModNotify(r *core.Router) {
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageServer, Handler: handleModNotify,
		Def: &discordgo.ApplicationCommand{
			Name: "modnotify", Description: "Configure DM notifications for moderation actions",
			Options: []*discordgo.ApplicationCommandOption{
				sub("show", "Show the current DM notification settings"),
				sub("set", "Toggle DMs for each action type (omit to leave unchanged)",
					optBool("warn", "DM members when warned", false),
					optBool("timeout", "DM members when timed out", false),
					optBool("kick", "DM members when kicked", false),
					optBool("ban", "DM members when banned", false)),
				sub("appeal", "Set the appeal note appended to moderation DMs",
					optString("message", "Appeal text (leave empty to clear)", false)),
			},
		},
	})
}

func handleModNotify(c *core.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := c.Store.GetModSettings(ctx, c.GuildID)
	if err != nil {
		_ = c.Errorf("Failed to load moderation settings.", err)
		return
	}

	switch c.SubCommand {
	case "show":
		_ = c.Reply(modNotifyEmbed(c, cfg))
	case "set":
		cfg.DMOnWarn = c.BoolOpt("warn", cfg.DMOnWarn)
		cfg.DMOnTimeout = c.BoolOpt("timeout", cfg.DMOnTimeout)
		cfg.DMOnKick = c.BoolOpt("kick", cfg.DMOnKick)
		cfg.DMOnBan = c.BoolOpt("ban", cfg.DMOnBan)
		saveModNotify(c, ctx, cfg)
	case "appeal":
		if m := c.StringOpt("message", ""); m != "" {
			cfg.AppealMessage = &m
		} else {
			cfg.AppealMessage = nil
		}
		saveModNotify(c, ctx, cfg)
	default:
		_ = c.Errorf("Unknown modnotify subcommand.", nil)
	}
}

func saveModNotify(c *core.Context, ctx context.Context, cfg *queries.ModSettings) {
	if err := c.Store.UpsertModSettings(ctx, cfg); err != nil {
		_ = c.Errorf("Failed to save moderation settings.", err)
		return
	}
	_ = c.Reply(modNotifyEmbed(c, cfg))
}

func modNotifyEmbed(c *core.Context, cfg *queries.ModSettings) *discordgo.MessageEmbed {
	b := c.Embed().Title("Moderation DM Notifications").
		Field("On Warn", onOff(cfg.DMOnWarn), true).
		Field("On Timeout", onOff(cfg.DMOnTimeout), true).
		Field("On Kick", onOff(cfg.DMOnKick), true).
		Field("On Ban", onOff(cfg.DMOnBan), true)
	if cfg.AppealMessage != nil && *cfg.AppealMessage != "" {
		b.Field("Appeal Note", *cfg.AppealMessage, false)
	}
	return b.Timestamp().Build()
}
