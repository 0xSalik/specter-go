package moderation

import (
	"context"
	"fmt"
	"time"

	"github.com/0xSalik/specter/internal/core"
	"github.com/0xSalik/specter/internal/modlog"
)

func handleWarning(c *core.Context) {
	switch c.SubCommand {
	case "add":
		warnAdd(c)
	case "remove":
		warnRemove(c)
	case "list":
		warnList(c)
	default:
		_ = c.Errorf("Unknown warning subcommand.", nil)
	}
}

func warnAdd(c *core.Context) {
	user := c.UserOpt("user")
	reason := c.StringOpt("reason", "")
	if user == nil || reason == "" {
		_ = c.Errorf("A user and reason are required.", nil)
		return
	}
	_ = c.Defer(false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, err := c.Store.AddWarning(ctx, c.GuildID, user.ID, c.UserID, reason)
	if err != nil {
		_ = c.Errorf("Failed to record the warning.", err)
		return
	}

	dmNotify(c, user.ID, "warn", "You have received a warning",
		fmt.Sprintf("You were warned in **%s**.", guildName(c)), reason)

	recordAndLog(c, modlog.EventWarn, "warn", user.ID, user.Username, reason, nil)
	_ = c.Reply(c.Embed().Title("Warning Issued").
		Description(fmt.Sprintf("**%s** has been warned.", user.Username)).
		Field("Warning ID", fmt.Sprintf("#%d", id), true).
		Field("Reason", reason, false).AsSuccess().Timestamp().Build())
}

func warnRemove(c *core.Context) {
	user := c.UserOpt("user")
	warnID := c.IntOpt("warning_id", 0)
	if user == nil || warnID == 0 {
		_ = c.Errorf("A user and warning ID are required.", nil)
		return
	}
	_ = c.Defer(false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ok, err := c.Store.RemoveWarning(ctx, c.GuildID, warnID)
	if err != nil {
		_ = c.Errorf("Failed to remove the warning.", err)
		return
	}
	if !ok {
		_ = c.Errorf(fmt.Sprintf("No active warning with ID #%d was found.", warnID), nil)
		return
	}
	_ = c.Success("Warning Removed", fmt.Sprintf("Warning #%d has been deactivated.", warnID))
}

func warnList(c *core.Context) {
	user := c.UserOpt("user")
	if user == nil {
		_ = c.Errorf("You must specify a user.", nil)
		return
	}
	_ = c.Defer(false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	warns, err := c.Store.ListWarnings(ctx, c.GuildID, user.ID)
	if err != nil {
		_ = c.Errorf("Failed to load warnings.", err)
		return
	}

	b := c.Embed().Title(fmt.Sprintf("Active Warnings for %s", user.Username)).Timestamp()
	if len(warns) == 0 {
		b.Description("This user has no active warnings.")
	} else {
		for _, w := range warns {
			b.Field(fmt.Sprintf("#%d • <@%s>", w.ID, w.ModID),
				fmt.Sprintf("%s\n%s", w.Reason, w.CreatedAt.Format("2006-01-02 15:04")), false)
		}
	}
	_ = c.Reply(b.Build())
}
