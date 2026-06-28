package moderation

import (
	"fmt"
	"time"

	"github.com/0xSalik/specter/internal/core"
	"github.com/0xSalik/specter/internal/discordutil"
	"github.com/0xSalik/specter/internal/modlog"
)

const maxTimeout = 28 * 24 * time.Hour

func handleTimeout(c *core.Context) {
	user := c.UserOpt("user")
	if user == nil {
		_ = c.Errorf("You must specify a user to timeout.", nil)
		return
	}
	durStr := c.StringOpt("duration", "")
	reason := c.StringOpt("reason", "No reason provided")

	dur, err := discordutil.ParseDuration(durStr)
	if err != nil {
		_ = c.Errorf(err.Error(), nil)
		return
	}
	if dur > maxTimeout {
		_ = c.Errorf("Timeout duration cannot exceed 28 days (Discord's limit).", nil)
		return
	}

	_ = c.Defer(false)

	if ok, why := discordutil.CanActOn(c.Session, c.GuildID, c.UserID, user.ID); !ok {
		_ = c.Errorf(why, nil)
		return
	}

	until := time.Now().Add(dur)
	if err := c.Session.GuildMemberTimeout(c.GuildID, user.ID, &until); err != nil {
		_ = c.Errorf("Failed to timeout the user. Check the bot's permissions and role position.", err)
		return
	}

	tryDM(c, user.ID, c.Embed().Title("You have been timed out").
		Description(fmt.Sprintf("You were timed out in **%s** for %s.", guildName(c), discordutil.FormatDuration(dur))).
		Field("Reason", reason, false).AsError().Build())

	recordAndLog(c, modlog.EventTimeout, "timeout", user.ID, user.Username, reason, &dur)
	_ = c.Reply(c.Embed().Title("Member Timed Out").
		Description(fmt.Sprintf("**%s** has been timed out for %s.", user.Username, discordutil.FormatDuration(dur))).
		Field("Reason", reason, false).AsSuccess().Timestamp().Build())
}
