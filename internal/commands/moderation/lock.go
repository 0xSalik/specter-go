package moderation

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
)

func resolveChannel(c *core.Context) (*discordgo.Channel, error) {
	if ch := c.ChannelOpt("channel"); ch != nil {
		return ch, nil
	}
	return c.Session.Channel(c.Interaction.ChannelID)
}

func handleLock(c *core.Context) {
	_ = c.Defer(false)
	ch, err := resolveChannel(c)
	if err != nil {
		_ = c.Errorf("Could not resolve the channel.", err)
		return
	}
	reason := c.StringOpt("reason", "No reason provided")

	// Preserve existing overwrites for @everyone; only add the SendMessages deny.
	var allow, deny int64
	for _, ow := range ch.PermissionOverwrites {
		if ow.ID == c.GuildID && ow.Type == discordgo.PermissionOverwriteTypeRole {
			allow = ow.Allow
			deny = ow.Deny
			break
		}
	}
	allow &^= discordgo.PermissionSendMessages
	deny |= discordgo.PermissionSendMessages

	if err := c.Session.ChannelPermissionSet(ch.ID, c.GuildID, discordgo.PermissionOverwriteTypeRole, allow, deny); err != nil {
		_ = c.Errorf("Failed to lock the channel. Check the bot's permissions.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Channel Locked").
		Description(fmt.Sprintf("<#%s> is now locked.", ch.ID)).
		Field("Reason", reason, false).AsSuccess().Timestamp().Build())
}

func handleUnlock(c *core.Context) {
	_ = c.Defer(false)
	ch, err := resolveChannel(c)
	if err != nil {
		_ = c.Errorf("Could not resolve the channel.", err)
		return
	}

	var allow, deny int64
	for _, ow := range ch.PermissionOverwrites {
		if ow.ID == c.GuildID && ow.Type == discordgo.PermissionOverwriteTypeRole {
			allow = ow.Allow
			deny = ow.Deny
			break
		}
	}
	// Remove only the SendMessages deny; do not blindly grant allow.
	deny &^= discordgo.PermissionSendMessages

	if err := c.Session.ChannelPermissionSet(ch.ID, c.GuildID, discordgo.PermissionOverwriteTypeRole, allow, deny); err != nil {
		_ = c.Errorf("Failed to unlock the channel. Check the bot's permissions.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Channel Unlocked").
		Description(fmt.Sprintf("<#%s> is now unlocked.", ch.ID)).AsSuccess().Timestamp().Build())
}
