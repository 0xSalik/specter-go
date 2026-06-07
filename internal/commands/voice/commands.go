// Package voice implements the /voice owner-management subcommands for
// join-to-create channels.
package voice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/core"
	"github.com/salik/specter/internal/db"
)

const group = "voice"

// Register wires the /voice command into the router.
func Register(r *core.Router) {
	r.Register(core.Command{
		Group: group, Handler: handle,
		Def: &discordgo.ApplicationCommand{
			Name: "voice", Description: "Manage your join-to-create voice channel",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "name", Description: "Rename your channel",
					Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "New name", Required: true}}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "limit", Description: "Set the user limit",
					Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionInteger, Name: "limit", Description: "0-99", Required: true, MinValue: ptrFloat(0), MaxValue: 99}}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "lock", Description: "Lock the channel"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "unlock", Description: "Unlock the channel"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "permit", Description: "Permit a user to connect",
					Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User", Required: true}}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "reject", Description: "Reject a user",
					Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User", Required: true}}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "transfer", Description: "Transfer ownership",
					Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "New owner", Required: true}}},
			},
		},
	})
}

func handle(c *core.Context) {
	channelID, err := ownedChannel(c)
	if err != nil {
		_ = c.Errorf(err.Error(), nil)
		return
	}

	switch c.SubCommand {
	case "name":
		setName(c, channelID)
	case "limit":
		setLimit(c, channelID)
	case "lock":
		setLock(c, channelID, true)
	case "unlock":
		setLock(c, channelID, false)
	case "permit":
		permit(c, channelID, true)
	case "reject":
		permit(c, channelID, false)
	case "transfer":
		transfer(c, channelID)
	default:
		_ = c.Errorf("Unknown subcommand.", nil)
	}
}

// ownedChannel returns the JTC channel the caller currently occupies and owns.
func ownedChannel(c *core.Context) (string, error) {
	vcID := ""
	if vs, err := c.Session.State.VoiceState(c.GuildID, c.UserID); err == nil && vs != nil {
		vcID = vs.ChannelID
	}
	if vcID == "" {
		if g, err := c.Session.State.Guild(c.GuildID); err == nil {
			for _, vs := range g.VoiceStates {
				if vs.UserID == c.UserID {
					vcID = vs.ChannelID
					break
				}
			}
		}
	}
	if vcID == "" {
		return "", errors.New("you must be in your join-to-create voice channel to use this command")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rec, err := c.Store.GetJTCChannel(ctx, vcID)
	if err != nil {
		if db.IsNotFound(err) {
			return "", errors.New("this is not a join-to-create channel")
		}
		return "", errors.New("could not verify channel ownership")
	}
	isAdmin := c.Interaction.Member != nil && c.Interaction.Member.Permissions&discordgo.PermissionAdministrator != 0
	if rec.OwnerID != c.UserID && !isAdmin {
		return "", errors.New("only the channel owner can manage this channel")
	}
	return vcID, nil
}

func setName(c *core.Context, channelID string) {
	name := c.StringOpt("name", "")
	_ = c.Defer(true)
	if _, err := c.Session.ChannelEdit(channelID, &discordgo.ChannelEdit{Name: name}); err != nil {
		_ = c.Errorf("Failed to rename the channel.", err)
		return
	}
	_ = c.Success("Channel Renamed", fmt.Sprintf("Renamed to **%s**.", name))
}

func setLimit(c *core.Context, channelID string) {
	limit := c.IntOpt("limit", 0)
	_ = c.Defer(true)
	if _, err := c.Session.ChannelEdit(channelID, &discordgo.ChannelEdit{UserLimit: limit}); err != nil {
		_ = c.Errorf("Failed to set the user limit.", err)
		return
	}
	_ = c.Success("Limit Updated", fmt.Sprintf("User limit set to **%d**.", limit))
}

func setLock(c *core.Context, channelID string, lock bool) {
	_ = c.Defer(true)
	ch, err := c.Session.Channel(channelID)
	if err != nil {
		_ = c.Errorf("Could not load the channel.", err)
		return
	}
	var allow, deny int64
	for _, ow := range ch.PermissionOverwrites {
		if ow.ID == c.GuildID && ow.Type == discordgo.PermissionOverwriteTypeRole {
			allow, deny = ow.Allow, ow.Deny
			break
		}
	}
	if lock {
		deny |= discordgo.PermissionVoiceConnect
		allow &^= discordgo.PermissionVoiceConnect
	} else {
		deny &^= discordgo.PermissionVoiceConnect
	}
	if err := c.Session.ChannelPermissionSet(channelID, c.GuildID, discordgo.PermissionOverwriteTypeRole, allow, deny); err != nil {
		_ = c.Errorf("Failed to update channel permissions.", err)
		return
	}
	if lock {
		_ = c.Success("Channel Locked", "Members can no longer connect.")
	} else {
		_ = c.Success("Channel Unlocked", "Members can connect again.")
	}
}

func permit(c *core.Context, channelID string, allow bool) {
	user := c.UserOpt("user")
	if user == nil {
		_ = c.Errorf("You must specify a user.", nil)
		return
	}
	_ = c.Defer(true)
	var a, d int64
	if allow {
		a = discordgo.PermissionVoiceConnect
	} else {
		d = discordgo.PermissionVoiceConnect
	}
	if err := c.Session.ChannelPermissionSet(channelID, user.ID, discordgo.PermissionOverwriteTypeMember, a, d); err != nil {
		_ = c.Errorf("Failed to update permissions for that user.", err)
		return
	}
	if !allow {
		// Kick them from the channel if currently connected.
		_ = c.Session.GuildMemberMove(c.GuildID, user.ID, nil)
		_ = c.Success("User Rejected", fmt.Sprintf("<@%s> can no longer connect.", user.ID))
		return
	}
	_ = c.Success("User Permitted", fmt.Sprintf("<@%s> can now connect.", user.ID))
}

func transfer(c *core.Context, channelID string) {
	user := c.UserOpt("user")
	if user == nil {
		_ = c.Errorf("You must specify a user.", nil)
		return
	}
	_ = c.Defer(true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Store.SetJTCOwner(ctx, channelID, user.ID); err != nil {
		_ = c.Errorf("Failed to transfer ownership.", err)
		return
	}
	_ = c.Success("Ownership Transferred", fmt.Sprintf("<@%s> now owns this channel.", user.ID))
}

func ptrFloat(f float64) *float64 { return &f }

var _ = time.Second
