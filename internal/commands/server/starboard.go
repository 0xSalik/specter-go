package server

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
	"github.com/0xSalik/specter/internal/db/queries"
)

func registerStarboard(r *core.Router) {
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageServer, Handler: handleStarboard,
		Def: &discordgo.ApplicationCommand{
			Name: "starboard", Description: "Highlight popular messages in a starboard channel",
			Options: []*discordgo.ApplicationCommandOption{
				sub("show", "Show the current starboard configuration"),
				sub("setup", "Set the starboard channel and enable it",
					optChannel("channel", "Channel where starred messages are posted", true),
					optInt("threshold", "Stars required (default 3)", false),
					optString("emoji", "Star emoji (default ⭐)", false)),
				sub("toggle", "Enable or disable the starboard", optBool("enabled", "On or off", true)),
				sub("threshold", "Set the number of stars required", optInt("value", "Stars required (1+)", true)),
				sub("emoji", "Set the star emoji", optString("value", "Emoji to use", true)),
				sub("selfstar", "Allow members to star their own messages",
					optBool("enabled", "On = allow self-stars", true)),
			},
		},
	})
}

func handleStarboard(c *core.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := c.Store.GetStarboardConfig(ctx, c.GuildID)
	if err != nil {
		_ = c.Errorf("Failed to load the starboard configuration.", err)
		return
	}

	switch c.SubCommand {
	case "show":
		_ = c.Reply(starboardEmbed(c, cfg))
	case "setup":
		ch := c.ChannelOpt("channel")
		if ch == nil {
			_ = c.Errorf("You must specify a channel.", nil)
			return
		}
		cfg.ChannelID = &ch.ID
		cfg.Enabled = true
		if c.HasOpt("threshold") {
			cfg.Threshold = clampThreshold(c.IntOpt("threshold", cfg.Threshold))
		}
		if e := c.StringOpt("emoji", ""); e != "" {
			cfg.Emoji = e
		}
		saveStarboard(c, ctx, cfg)
	case "toggle":
		cfg.Enabled = c.BoolOpt("enabled", cfg.Enabled)
		saveStarboard(c, ctx, cfg)
	case "threshold":
		cfg.Threshold = clampThreshold(c.IntOpt("value", cfg.Threshold))
		saveStarboard(c, ctx, cfg)
	case "emoji":
		if e := c.StringOpt("value", ""); e != "" {
			cfg.Emoji = e
		}
		saveStarboard(c, ctx, cfg)
	case "selfstar":
		cfg.SelfStar = c.BoolOpt("enabled", cfg.SelfStar)
		saveStarboard(c, ctx, cfg)
	default:
		_ = c.Errorf("Unknown starboard subcommand.", nil)
	}
}

func clampThreshold(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func saveStarboard(c *core.Context, ctx context.Context, cfg *queries.StarboardConfig) {
	if cfg.Enabled && (cfg.ChannelID == nil || *cfg.ChannelID == "") {
		_ = c.Errorf("Set a starboard channel first with `/starboard setup`.", nil)
		return
	}
	if err := c.Store.UpsertStarboardConfig(ctx, cfg); err != nil {
		_ = c.Errorf("Failed to save the starboard configuration.", err)
		return
	}
	_ = c.Reply(starboardEmbed(c, cfg))
}

func starboardEmbed(c *core.Context, cfg *queries.StarboardConfig) *discordgo.MessageEmbed {
	return c.Embed().Title("Starboard Configuration").
		Field("Status", onOff(cfg.Enabled), true).
		Field("Channel", channelMention(cfg.ChannelID), true).
		Field("Emoji", cfg.Emoji, true).
		Field("Threshold", fmt.Sprintf("%d stars", cfg.Threshold), true).
		Field("Self-star", onOff(cfg.SelfStar), true).
		Timestamp().Build()
}
