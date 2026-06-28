package moderation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
)

const rapsheetPageSize = 10

func handleRapsheet(c *core.Context) {
	user := c.UserOpt("user")
	if user == nil {
		_ = c.Errorf("You must specify a user.", nil)
		return
	}
	action := c.StringOpt("action", "view")

	if action == "clear" {
		if c.Interaction.Member == nil || c.Interaction.Member.Permissions&discordgo.PermissionAdministrator == 0 {
			_ = c.Errorf("Clearing a rapsheet requires Administrator permission.", nil)
			return
		}
		e := c.Embed().Title("Confirm Rapsheet Clear").AsError().
			Description(fmt.Sprintf("This will permanently delete all moderation actions and warnings for **%s**. This cannot be undone.", user.Username)).Build()
		components := []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{Label: "Confirm", Style: discordgo.DangerButton, CustomID: fmt.Sprintf("rapsheet:clearconfirm:%s:%s", c.GuildID, user.ID)},
				discordgo.Button{Label: "Cancel", Style: discordgo.SecondaryButton, CustomID: "rapsheet:clearcancel"},
			}},
		}
		_ = c.ReplyComponents(e, components)
		return
	}

	_ = c.Defer(false)
	e, comps, err := buildRapsheetPage(c, c.GuildID, user.ID, 0)
	if err != nil {
		_ = c.Errorf("Failed to load the rapsheet.", err)
		return
	}
	_ = c.ReplyComponents(e, comps)
}

func buildRapsheetPage(c *core.Context, guildID, userID string, page int) (*discordgo.MessageEmbed, []discordgo.MessageComponent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	total, err := c.Store.CountActions(ctx, guildID, userID)
	if err != nil {
		return nil, nil, err
	}
	if page < 0 {
		page = 0
	}
	maxPage := 0
	if total > 0 {
		maxPage = (total - 1) / rapsheetPageSize
	}
	if page > maxPage {
		page = maxPage
	}

	actions, err := c.Store.ListActions(ctx, guildID, userID, rapsheetPageSize, page*rapsheetPageSize)
	if err != nil {
		return nil, nil, err
	}

	b := c.Embed().Title("Rapsheet").
		Description(fmt.Sprintf("<@%s> — %d total action(s)", userID, total)).
		Footer(fmt.Sprintf("Page %d/%d", page+1, maxPage+1)).Timestamp()
	if len(actions) == 0 {
		b.Description(fmt.Sprintf("<@%s> has a clean record.", userID))
	}
	for _, a := range actions {
		reason := "No reason provided"
		if a.Reason != nil && *a.Reason != "" {
			reason = *a.Reason
		}
		b.Field(fmt.Sprintf("#%d • %s", a.ID, strings.ToUpper(a.Action)),
			fmt.Sprintf("By <@%s> on %s\n%s", a.ModID, a.CreatedAt.Format("2006-01-02 15:04"), reason), false)
	}

	var comps []discordgo.MessageComponent
	if maxPage > 0 {
		row := discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: "Previous", Style: discordgo.SecondaryButton, Disabled: page == 0,
				CustomID: fmt.Sprintf("rapsheet:page:%s:%s:%d", guildID, userID, page-1)},
			discordgo.Button{Label: "Next", Style: discordgo.SecondaryButton, Disabled: page >= maxPage,
				CustomID: fmt.Sprintf("rapsheet:page:%s:%s:%d", guildID, userID, page+1)},
		}}
		comps = append(comps, row)
	}
	return b.Build(), comps, nil
}

func handleRapsheetComponent(c *core.Context, customID string) {
	parts := strings.Split(customID, ":")
	if len(parts) < 2 {
		return
	}
	switch parts[1] {
	case "page":
		if len(parts) != 5 {
			return
		}
		page, _ := strconv.Atoi(parts[4])
		e, comps, err := buildRapsheetPage(c, parts[2], parts[3], page)
		if err != nil {
			return
		}
		_ = c.Session.InteractionRespond(c.Interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{e}, Components: comps},
		})
	case "clearcancel":
		_ = c.Session.InteractionRespond(c.Interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{
				c.Embed().Title("Cancelled").Description("Rapsheet clear cancelled.").Build()}, Components: []discordgo.MessageComponent{}},
		})
	case "clearconfirm":
		if len(parts) != 4 {
			return
		}
		if c.Interaction.Member == nil || c.Interaction.Member.Permissions&discordgo.PermissionAdministrator == 0 {
			return
		}
		guildID, userID := parts[2], parts[3]
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Store.ClearActions(ctx, guildID, userID)
		_ = c.Store.ClearWarnings(ctx, guildID, userID)
		_ = c.Session.InteractionRespond(c.Interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{
				c.Embed().Title("Rapsheet Cleared").AsSuccess().
					Description(fmt.Sprintf("All actions and warnings for <@%s> have been deleted.", userID)).Build()},
				Components: []discordgo.MessageComponent{}},
		})
	}
}
