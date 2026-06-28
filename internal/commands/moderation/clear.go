package moderation

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
)

// bulkDeleteCutoff is Discord's 14-day limit for bulk deletion.
const bulkDeleteCutoff = 14 * 24 * time.Hour

func handleClear(c *core.Context) {
	amount := c.IntOpt("amount", 0)
	if amount < 1 || amount > 100 {
		_ = c.Errorf("Amount must be between 1 and 100.", nil)
		return
	}
	filterUser := c.UserOpt("user")

	_ = c.Defer(true)

	channelID := c.Interaction.ChannelID
	msgs, err := c.Session.ChannelMessages(channelID, amount, "", "", "")
	if err != nil {
		_ = c.Errorf("Failed to fetch messages to delete.", err)
		return
	}

	var bulkIDs []string
	var oldIDs []string
	now := time.Now()
	for _, m := range msgs {
		if filterUser != nil && (m.Author == nil || m.Author.ID != filterUser.ID) {
			continue
		}
		if now.Sub(m.Timestamp) < bulkDeleteCutoff {
			bulkIDs = append(bulkIDs, m.ID)
		} else {
			oldIDs = append(oldIDs, m.ID)
		}
	}

	deleted := 0
	if len(bulkIDs) == 1 {
		if err := c.Session.ChannelMessageDelete(channelID, bulkIDs[0]); err == nil {
			deleted++
		}
	} else if len(bulkIDs) > 1 {
		if err := c.Session.ChannelMessagesBulkDelete(channelID, bulkIDs); err != nil {
			_ = c.Errorf("Failed to bulk delete messages.", err)
			return
		}
		deleted += len(bulkIDs)
	}

	// Older messages must be deleted individually.
	for _, id := range oldIDs {
		if err := c.Session.ChannelMessageDelete(channelID, id); err == nil {
			deleted++
		}
	}

	desc := fmt.Sprintf("Deleted %d message(s).", deleted)
	if len(oldIDs) > 0 {
		desc += fmt.Sprintf(" %d were older than 14 days and removed individually.", len(oldIDs))
	}
	_ = c.Reply(c.Embed().Title("Messages Cleared").Description(desc).AsSuccess().Build())
}

var _ = discordgo.PermissionManageMessages
