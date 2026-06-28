// Package moderation implements all moderation slash commands. Every action is
// recorded to the rapsheet (mod_actions) and dispatched to the appropriate log
// channel after success.
package moderation

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
	"github.com/0xSalik/specter/internal/modlog"
)

const group = "moderation"

// recordAndLog persists a moderation action and dispatches a mod-log event.
func recordAndLog(c *core.Context, eventType, action, targetID, targetName, reason string, duration *time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	if _, err := c.Store.RecordAction(ctx, c.GuildID, targetID, c.UserID, action, reasonPtr, duration); err != nil {
		// Logged but not surfaced to the user; the Discord action already succeeded.
		c.Errorf("Action completed but failed to write to the rapsheet.", err) //nolint:errcheck
	}

	extra := map[string]string{}
	if duration != nil {
		extra["Duration"] = duration.String()
	}
	c.Modlog.Log(c.Session, modlog.ModLogEvent{
		GuildID:    c.GuildID,
		EventType:  eventType,
		ActorID:    c.UserID,
		TargetID:   targetID,
		TargetName: targetName,
		Reason:     reason,
		Extra:      extra,
		Timestamp:  time.Now(),
	})
}

// tryDM attempts to notify a user before a punitive action. Failure (closed
// DMs) is intentionally ignored.
func tryDM(c *core.Context, userID string, e *discordgo.MessageEmbed) {
	ch, err := c.Session.UserChannelCreate(userID)
	if err != nil {
		return
	}
	_, _ = c.Session.ChannelMessageSendEmbed(ch.ID, e)
}

// dmNotify DMs a member about a moderation action, but only when the guild has
// DM notifications enabled for that action type. The configured appeal message
// (if any) is appended. action is one of "warn","timeout","kick","ban".
func dmNotify(c *core.Context, userID, action, title, body, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms, err := c.Store.GetModSettings(ctx, c.GuildID)
	if err != nil {
		return
	}
	enabled := map[string]bool{
		"warn": ms.DMOnWarn, "timeout": ms.DMOnTimeout,
		"kick": ms.DMOnKick, "ban": ms.DMOnBan,
	}[action]
	if !enabled {
		return
	}

	b := c.Embed().Title(title).Description(body).AsError()
	if reason != "" {
		b.Field("Reason", reason, false)
	}
	if ms.AppealMessage != nil && *ms.AppealMessage != "" {
		b.Field("Appeal", *ms.AppealMessage, false)
	}
	tryDM(c, userID, b.Build())
}

// guildName returns the guild's name for use in DM notifications.
func guildName(c *core.Context) string {
	if g, err := c.Session.State.Guild(c.GuildID); err == nil && g != nil {
		return g.Name
	}
	if g, err := c.Session.Guild(c.GuildID); err == nil && g != nil {
		return g.Name
	}
	return "the server"
}
