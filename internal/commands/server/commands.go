// Package server implements server-configuration slash commands that mirror the
// dashboard: welcome/goodbye messages, autorole, level role rewards, starboard,
// and moderation DM notifications. Every command requires Manage Server (or a
// more specific permission) and is grouped under "settings" for access control.
package server

import (
	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
)

const group = "settings"

// Register wires all server-configuration commands into the router.
func Register(r *core.Router) {
	registerWelcome(r)
	registerAutorole(r)
	registerLevelRole(r)
	registerStarboard(r)
	registerModNotify(r)
}

func optBool(name, desc string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type: discordgo.ApplicationCommandOptionBoolean, Name: name, Description: desc, Required: required,
	}
}

func optString(name, desc string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type: discordgo.ApplicationCommandOptionString, Name: name, Description: desc, Required: required,
	}
}

func optChannel(name, desc string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type: discordgo.ApplicationCommandOptionChannel, Name: name, Description: desc, Required: required,
	}
}

func optRole(name, desc string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type: discordgo.ApplicationCommandOptionRole, Name: name, Description: desc, Required: required,
	}
}

func optInt(name, desc string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type: discordgo.ApplicationCommandOptionInteger, Name: name, Description: desc, Required: required,
	}
}

func sub(name, desc string, opts ...*discordgo.ApplicationCommandOption) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type: discordgo.ApplicationCommandOptionSubCommand, Name: name, Description: desc, Options: opts,
	}
}

func onOff(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}

func channelMention(id *string) string {
	if id == nil || *id == "" {
		return "*(not set)*"
	}
	return "<#" + *id + ">"
}
