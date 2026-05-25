package moderation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/core"
	"github.com/salik/specter/internal/db/queries"
	"github.com/salik/specter/internal/embed"
)

func registerAutomod(r *core.Router) {
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageServer, Handler: handleAutomod,
		Def: &discordgo.ApplicationCommand{
			Name: "automod", Description: "Configure the automod system",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "enable", Description: "Enable automod"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "disable", Description: "Disable automod"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "status", Description: "Show automod status"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "config", Description: "Show config with toggles"},
			},
		},
	})
	r.RegisterComponent("automod", handleAutomodComponent)
}

func handleAutomod(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := c.Store.GetAutomodConfig(ctx, c.GuildID)
	if err != nil {
		_ = c.Errorf("Failed to load automod configuration.", err)
		return
	}

	switch c.SubCommand {
	case "enable":
		cfg.Enabled = true
		if err := c.Store.UpsertAutomodConfig(ctx, cfg); err != nil {
			_ = c.Errorf("Failed to enable automod.", err)
			return
		}
		_ = c.Success("Automod Enabled", "Automod is now active for this server.")
	case "disable":
		cfg.Enabled = false
		if err := c.Store.UpsertAutomodConfig(ctx, cfg); err != nil {
			_ = c.Errorf("Failed to disable automod.", err)
			return
		}
		_ = c.Success("Automod Disabled", "Automod has been turned off for this server.")
	case "status":
		_ = c.Reply(automodEmbed(c, cfg).Build())
	case "config":
		_ = c.ReplyComponents(automodEmbed(c, cfg).Build(), automodComponents())
	default:
		_ = c.Errorf("Unknown automod subcommand.", nil)
	}
}

func automodEmbed(c *core.Context, cfg *queries.AutomodConfig) *embed.EmbedBuilder {
	onoff := func(b bool) string {
		if b {
			return "Enabled"
		}
		return "Disabled"
	}
	b := c.Embed().Title("Automod Configuration").
		Field("Master Switch", onoff(cfg.Enabled), true).
		Field("Action", titleCase(cfg.Action), true).
		Field("Anti-Spam", fmt.Sprintf("%s (%d/%ds)", onoff(cfg.AntiSpamEnabled), cfg.AntiSpamThreshold, cfg.AntiSpamWindowSecs), true).
		Field("Anti-Invite", onoff(cfg.AntiInviteEnabled), true).
		Field("Anti-Link", onoff(cfg.AntiLinkEnabled), true).
		Field("Anti-Caps", fmt.Sprintf("%s (%d%%)", onoff(cfg.AntiCapsEnabled), cfg.CapsThresholdPct), true).
		Field("Bad Words", fmt.Sprintf("%s (%d words)", onoff(cfg.BadWordsEnabled), len(cfg.BadWords)), true).
		Timestamp()
	return b
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func automodComponents() []discordgo.MessageComponent {
	mk := func(label, rule string) discordgo.Button {
		return discordgo.Button{Label: label, Style: discordgo.SecondaryButton, CustomID: "automod:toggle:" + rule}
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			mk("Toggle Spam", "spam"), mk("Toggle Invite", "invite"), mk("Toggle Link", "link"),
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			mk("Toggle Caps", "caps"), mk("Toggle Bad Words", "badwords"), mk("Toggle Master", "master"),
		}},
	}
}

func handleAutomodComponent(c *core.Context, customID string) {
	parts := strings.Split(customID, ":")
	if len(parts) != 3 || parts[1] != "toggle" {
		return
	}
	if c.Interaction.Member == nil || c.Interaction.Member.Permissions&(discordgo.PermissionManageServer|discordgo.PermissionAdministrator) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := c.Store.GetAutomodConfig(ctx, c.GuildID)
	if err != nil {
		return
	}
	switch parts[2] {
	case "spam":
		cfg.AntiSpamEnabled = !cfg.AntiSpamEnabled
	case "invite":
		cfg.AntiInviteEnabled = !cfg.AntiInviteEnabled
	case "link":
		cfg.AntiLinkEnabled = !cfg.AntiLinkEnabled
	case "caps":
		cfg.AntiCapsEnabled = !cfg.AntiCapsEnabled
	case "badwords":
		cfg.BadWordsEnabled = !cfg.BadWordsEnabled
	case "master":
		cfg.Enabled = !cfg.Enabled
	}
	if err := c.Store.UpsertAutomodConfig(ctx, cfg); err != nil {
		return
	}
	_ = c.Session.InteractionRespond(c.Interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{automodEmbed(c, cfg).Build()},
			Components: automodComponents(),
		},
	})
}
