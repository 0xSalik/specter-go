package server

import (
	"context"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
	"github.com/0xSalik/specter/internal/db/queries"
)

func registerAutorole(r *core.Router) {
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageRoles, Handler: handleAutorole,
		Def: &discordgo.ApplicationCommand{
			Name: "autorole", Description: "Automatically assign roles to new members",
			Options: []*discordgo.ApplicationCommandOption{
				sub("show", "Show the current autorole configuration"),
				sub("toggle", "Enable or disable autorole", optBool("enabled", "On or off", true)),
				sub("add", "Add a role granted automatically on join",
					optRole("role", "Role to grant", true),
					&discordgo.ApplicationCommandOption{
						Type: discordgo.ApplicationCommandOptionString, Name: "target", Description: "Who receives it",
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{Name: "humans", Value: "human"}, {Name: "bots", Value: "bot"},
						},
					}),
				sub("remove", "Stop granting a role on join", optRole("role", "Role to remove", true)),
			},
		},
	})
}

func handleAutorole(c *core.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := c.Store.GetAutoroleConfig(ctx, c.GuildID)
	if err != nil {
		_ = c.Errorf("Failed to load the autorole configuration.", err)
		return
	}

	switch c.SubCommand {
	case "show":
		_ = c.Reply(autoroleEmbed(c, cfg))
	case "toggle":
		cfg.Enabled = c.BoolOpt("enabled", cfg.Enabled)
		saveAutorole(c, ctx, cfg)
	case "add":
		role := c.RoleOpt("role")
		if role == nil {
			_ = c.Errorf("You must specify a role.", nil)
			return
		}
		if c.StringOpt("target", "human") == "bot" {
			cfg.BotRoleIDs = addUnique(cfg.BotRoleIDs, role.ID)
		} else {
			cfg.RoleIDs = addUnique(cfg.RoleIDs, role.ID)
		}
		saveAutorole(c, ctx, cfg)
	case "remove":
		role := c.RoleOpt("role")
		if role == nil {
			_ = c.Errorf("You must specify a role.", nil)
			return
		}
		cfg.RoleIDs = remove(cfg.RoleIDs, role.ID)
		cfg.BotRoleIDs = remove(cfg.BotRoleIDs, role.ID)
		saveAutorole(c, ctx, cfg)
	default:
		_ = c.Errorf("Unknown autorole subcommand.", nil)
	}
}

func saveAutorole(c *core.Context, ctx context.Context, cfg *queries.AutoroleConfig) {
	if err := c.Store.UpsertAutoroleConfig(ctx, cfg); err != nil {
		_ = c.Errorf("Failed to save the autorole configuration.", err)
		return
	}
	_ = c.Reply(autoroleEmbed(c, cfg))
}

func autoroleEmbed(c *core.Context, cfg *queries.AutoroleConfig) *discordgo.MessageEmbed {
	return c.Embed().Title("Autorole Configuration").
		Field("Status", onOff(cfg.Enabled), true).
		Field("Member Roles", roleMentions(cfg.RoleIDs), false).
		Field("Bot Roles", roleMentions(cfg.BotRoleIDs), false).
		Timestamp().Build()
}

func roleMentions(ids []string) string {
	if len(ids) == 0 {
		return "*(none)*"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = "<@&" + id + ">"
	}
	return strings.Join(parts, " ")
}

func addUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func remove(list []string, v string) []string {
	out := list[:0]
	for _, x := range list {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}
