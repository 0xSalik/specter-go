package moderation

import (
	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/core"
)

func optUser(required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "Target user", Required: required,
	}
}

func optReason() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason for the action", Required: false,
	}
}

// Register wires all moderation commands into the router.
func Register(r *core.Router) {
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionBanMembers, Handler: handleBan,
		Def: &discordgo.ApplicationCommand{
			Name: "ban", Description: "Ban a member from the server",
			Options: []*discordgo.ApplicationCommandOption{
				optUser(true), optReason(),
				{Type: discordgo.ApplicationCommandOptionInteger, Name: "delete_messages", Description: "Days of messages to delete (0-7)", MinValue: ptrFloat(0), MaxValue: 7},
			},
		},
	})
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionBanMembers, Handler: handleUnban,
		Def: &discordgo.ApplicationCommand{
			Name: "unban", Description: "Unban a user by ID",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "user_id", Description: "ID of the user to unban", Required: true},
				optReason(),
			},
		},
	})
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionKickMembers, Handler: handleKick,
		Def: &discordgo.ApplicationCommand{
			Name: "kick", Description: "Kick a member from the server",
			Options: []*discordgo.ApplicationCommandOption{optUser(true), optReason()},
		},
	})
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionModerateMembers, Handler: handleTimeout,
		Def: &discordgo.ApplicationCommand{
			Name: "timeout", Description: "Timeout a member for a duration",
			Options: []*discordgo.ApplicationCommandOption{
				optUser(true),
				{Type: discordgo.ApplicationCommandOptionString, Name: "duration", Description: "e.g. 10m, 1h, 1d (max 28d)", Required: true},
				optReason(),
			},
		},
	})
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionModerateMembers, Handler: handleWarning,
		Def: &discordgo.ApplicationCommand{
			Name: "warning", Description: "Manage member warnings",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "add", Description: "Add a warning",
					Options: []*discordgo.ApplicationCommandOption{optUser(true),
						{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason", Required: true}}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "remove", Description: "Remove a warning",
					Options: []*discordgo.ApplicationCommandOption{optUser(true),
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "warning_id", Description: "Warning ID", Required: true}}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "list", Description: "List active warnings",
					Options: []*discordgo.ApplicationCommandOption{optUser(true)}},
			},
		},
	})
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionModerateMembers, Handler: handleRapsheet,
		Def: &discordgo.ApplicationCommand{
			Name: "rapsheet", Description: "View or clear a member's moderation history",
			Options: []*discordgo.ApplicationCommandOption{
				optUser(true),
				{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "view or clear", Required: false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{{Name: "view", Value: "view"}, {Name: "clear", Value: "clear"}}},
			},
		},
	})
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageMessages, Handler: handleClear,
		Def: &discordgo.ApplicationCommand{
			Name: "clear", Description: "Bulk delete messages",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionInteger, Name: "amount", Description: "1-100", Required: true, MinValue: ptrFloat(1), MaxValue: 100},
				optUser(false),
			},
		},
	})
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageChannels, Handler: handleLock,
		Def: &discordgo.ApplicationCommand{
			Name: "lock", Description: "Lock a channel (deny @everyone send)",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Channel (default: current)", Required: false},
				optReason(),
			},
		},
	})
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageChannels, Handler: handleUnlock,
		Def: &discordgo.ApplicationCommand{
			Name: "unlock", Description: "Unlock a channel",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Channel (default: current)", Required: false},
			},
		},
	})
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionAdministrator, Handler: handleMassban,
		Def: &discordgo.ApplicationCommand{
			Name: "massban", Description: "Ban multiple users by ID",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "user_ids", Description: "Space or newline separated IDs", Required: true},
				optReason(),
			},
		},
	})

	registerAutomod(r)
	r.RegisterComponent("rapsheet", handleRapsheetComponent)
}

func ptrFloat(f float64) *float64 { return &f }
