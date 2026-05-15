package moderation

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/core"
	"github.com/salik/specter/internal/discordutil"
	"github.com/salik/specter/internal/modlog"
)

func handleBan(c *core.Context) {
	user := c.UserOpt("user")
	if user == nil {
		_ = c.Errorf("You must specify a user to ban.", nil)
		return
	}
	reason := c.StringOpt("reason", "No reason provided")
	delDays := c.IntOpt("delete_messages", 0)
	if delDays < 0 {
		delDays = 0
	}
	if delDays > 7 {
		delDays = 7
	}

	_ = c.Defer(false)

	if ok, why := discordutil.CanActOn(c.Session, c.GuildID, c.UserID, user.ID); !ok {
		_ = c.Errorf(why, nil)
		return
	}

	tryDM(c, user.ID, c.Embed().Title("You have been banned").
		Description(fmt.Sprintf("You were banned from **%s**.", guildName(c))).
		Field("Reason", reason, false).AsError().Build())

	if err := c.Session.GuildBanCreateWithReason(c.GuildID, user.ID, reason, delDays); err != nil {
		_ = c.Errorf("Failed to ban the user. Check the bot's permissions and role position.", err)
		return
	}

	recordAndLog(c, modlog.EventBan, "ban", user.ID, user.Username, reason, nil)
	_ = c.Reply(c.Embed().Title("Member Banned").
		Description(fmt.Sprintf("**%s** has been banned.", user.Username)).
		Field("Reason", reason, false).AsSuccess().Timestamp().Build())
}

func handleUnban(c *core.Context) {
	userID := c.StringOpt("user_id", "")
	reason := c.StringOpt("reason", "No reason provided")
	if userID == "" {
		_ = c.Errorf("You must provide a user ID to unban.", nil)
		return
	}

	_ = c.Defer(false)

	if err := c.Session.GuildBanDelete(c.GuildID, userID); err != nil {
		_ = c.Errorf("Failed to unban that user. They may not be banned, or the ID is invalid.", err)
		return
	}

	name := userID
	if u, err := c.Session.User(userID); err == nil {
		name = u.Username
	}
	recordAndLog(c, modlog.EventUnban, "unban", userID, name, reason, nil)
	_ = c.Reply(c.Embed().Title("Member Unbanned").
		Description(fmt.Sprintf("`%s` has been unbanned.", userID)).
		Field("Reason", reason, false).AsSuccess().Timestamp().Build())
}

func handleKick(c *core.Context) {
	user := c.UserOpt("user")
	if user == nil {
		_ = c.Errorf("You must specify a user to kick.", nil)
		return
	}
	reason := c.StringOpt("reason", "No reason provided")

	_ = c.Defer(false)

	if ok, why := discordutil.CanActOn(c.Session, c.GuildID, c.UserID, user.ID); !ok {
		_ = c.Errorf(why, nil)
		return
	}

	tryDM(c, user.ID, c.Embed().Title("You have been kicked").
		Description(fmt.Sprintf("You were kicked from **%s**.", guildName(c))).
		Field("Reason", reason, false).AsError().Build())

	if err := c.Session.GuildMemberDeleteWithReason(c.GuildID, user.ID, reason); err != nil {
		_ = c.Errorf("Failed to kick the user. Check the bot's permissions and role position.", err)
		return
	}

	recordAndLog(c, modlog.EventKick, "kick", user.ID, user.Username, reason, nil)
	_ = c.Reply(c.Embed().Title("Member Kicked").
		Description(fmt.Sprintf("**%s** has been kicked.", user.Username)).
		Field("Reason", reason, false).AsSuccess().Timestamp().Build())
}

var _ = discordgo.PermissionBanMembers
