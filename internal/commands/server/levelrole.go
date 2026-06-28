package server

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
)

func registerLevelRole(r *core.Router) {
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageServer, Handler: handleLevelRole,
		Def: &discordgo.ApplicationCommand{
			Name: "levelrole", Description: "Configure role rewards granted at levels",
			Options: []*discordgo.ApplicationCommandOption{
				sub("list", "List configured level role rewards"),
				sub("set", "Grant a role when members reach a level",
					optInt("level", "Level threshold (1+)", true),
					optRole("role", "Role to grant", true)),
				sub("remove", "Remove the reward for a level", optInt("level", "Level threshold", true)),
				sub("stack", "Keep lower reward roles when leveling up (on) or replace them (off)",
					optBool("enabled", "On = stack, Off = replace", true)),
			},
		},
	})
}

func handleLevelRole(c *core.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch c.SubCommand {
	case "list":
		levelRoleList(c, ctx)
	case "set":
		level := c.IntOpt("level", 0)
		role := c.RoleOpt("role")
		if level < 1 || role == nil {
			_ = c.Errorf("Provide a level of at least 1 and a role.", nil)
			return
		}
		if err := c.Store.SetLevelReward(ctx, c.GuildID, level, role.ID); err != nil {
			_ = c.Errorf("Failed to save the level reward.", err)
			return
		}
		_ = c.Success("Level Reward Set", fmt.Sprintf("Members reaching level %d will receive <@&%s>.", level, role.ID))
	case "remove":
		level := c.IntOpt("level", 0)
		ok, err := c.Store.DeleteLevelReward(ctx, c.GuildID, level)
		if err != nil {
			_ = c.Errorf("Failed to remove the level reward.", err)
			return
		}
		if !ok {
			_ = c.Errorf(fmt.Sprintf("No reward is configured for level %d.", level), nil)
			return
		}
		_ = c.Success("Level Reward Removed", fmt.Sprintf("The reward for level %d has been removed.", level))
	case "stack":
		cfg, err := c.Store.GetLevelConfig(ctx, c.GuildID)
		if err != nil {
			_ = c.Errorf("Failed to load the level configuration.", err)
			return
		}
		cfg.StackRewards = c.BoolOpt("enabled", cfg.StackRewards)
		if err := c.Store.UpsertLevelConfig(ctx, cfg); err != nil {
			_ = c.Errorf("Failed to save the level configuration.", err)
			return
		}
		mode := "stacked (lower roles kept)"
		if !cfg.StackRewards {
			mode = "replaced (only the highest role kept)"
		}
		_ = c.Success("Reward Stacking Updated", "Reward roles will now be "+mode+".")
	default:
		_ = c.Errorf("Unknown levelrole subcommand.", nil)
	}
}

func levelRoleList(c *core.Context, ctx context.Context) {
	rewards, err := c.Store.ListLevelRewards(ctx, c.GuildID)
	if err != nil {
		_ = c.Errorf("Failed to load level rewards.", err)
		return
	}
	cfg, _ := c.Store.GetLevelConfig(ctx, c.GuildID)
	stack := "Replace"
	if cfg != nil && cfg.StackRewards {
		stack = "Stack"
	}
	b := c.Embed().Title("Level Role Rewards").Field("Mode", stack, true).Timestamp()
	if len(rewards) == 0 {
		b.Description("No level rewards configured. Use `/levelrole set` to add one.")
	} else {
		for _, rw := range rewards {
			b.Field(fmt.Sprintf("Level %d", rw.Level), "<@&"+rw.RoleID+">", true)
		}
	}
	_ = c.Reply(b.Build())
}
