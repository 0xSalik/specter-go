package utils

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/core"
	"github.com/salik/specter/internal/db/queries"
)

var modlogEventTypes = []string{
	"message_delete", "message_edit", "member_join", "member_leave", "member_update",
	"ban", "unban", "kick", "warn", "channel_update", "guild_update",
}

func registerModlogs(r *core.Router) {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(modlogEventTypes))
	for _, t := range modlogEventTypes {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: t, Value: t})
	}
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageServer, Handler: handleModlogs,
		Def: &discordgo.ApplicationCommand{
			Name: "modlogs", Description: "Override the channel or enable/disable a log event type",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "event_type", Description: "Event type", Required: true, Choices: choices},
				{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Target channel (omit to clear override)", Required: false},
				{Type: discordgo.ApplicationCommandOptionString, Name: "state", Description: "enable or disable", Required: false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{{Name: "enable", Value: "enable"}, {Name: "disable", Value: "disable"}}},
			},
		},
	})
}

func handleModlogs(c *core.Context) {
	eventType := c.StringOpt("event_type", "")
	state := c.StringOpt("state", "enable")
	ch := c.ChannelOpt("channel")

	_ = c.Defer(false)

	override := queries.ModlogOverride{
		GuildID:   c.GuildID,
		EventType: eventType,
		Enabled:   state != "disable",
	}
	if ch != nil {
		override.ChannelID = &ch.ID
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Store.SetOverride(ctx, override); err != nil {
		_ = c.Errorf("Failed to save the override.", err)
		return
	}

	b := c.Embed().Title("Mod Log Override Updated").AsSuccess().
		Field("Event", eventType, true).
		Field("State", state, true)
	if ch != nil {
		b.Field("Channel", "<#"+ch.ID+">", true)
	} else {
		b.Field("Channel", "Default", true)
	}
	_ = c.Reply(b.Build())
}
