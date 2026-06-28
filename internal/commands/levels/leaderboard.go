package levels

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
)

const lbPageSize = 10

func handleLeaderboard(c *core.Context) {
	_ = c.Defer(false)
	e, comps, err := buildLeaderboard(c, c.GuildID, 0)
	if err != nil {
		_ = c.Errorf("Failed to load the leaderboard.", err)
		return
	}
	_ = c.ReplyComponents(e, comps)
}

func buildLeaderboard(c *core.Context, guildID string, page int) (*discordgo.MessageEmbed, []discordgo.MessageComponent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	total, err := c.Store.CountLevelEntries(ctx, guildID)
	if err != nil {
		return nil, nil, err
	}
	if page < 0 {
		page = 0
	}
	maxPage := 0
	if total > 0 {
		maxPage = (total - 1) / lbPageSize
	}
	if page > maxPage {
		page = maxPage
	}

	entries, err := c.Store.GetTopN(ctx, guildID, lbPageSize, page*lbPageSize)
	if err != nil {
		return nil, nil, err
	}

	var sb strings.Builder
	for i, e := range entries {
		rank := page*lbPageSize + i + 1
		fmt.Fprintf(&sb, "**%d.** <@%s> — Level %d (%d XP)\n", rank, e.UserID, e.Level, e.XP)
	}
	if sb.Len() == 0 {
		sb.WriteString("No one has earned XP yet.")
	}

	b := c.Embed().Title("XP Leaderboard").Description(sb.String()).
		Footer(fmt.Sprintf("Page %d/%d", page+1, maxPage+1)).Timestamp().Build()

	var comps []discordgo.MessageComponent
	if maxPage > 0 {
		comps = append(comps, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: "Previous", Style: discordgo.SecondaryButton, Disabled: page == 0,
				CustomID: fmt.Sprintf("leaderboard:%s:%d", guildID, page-1)},
			discordgo.Button{Label: "Next", Style: discordgo.SecondaryButton, Disabled: page >= maxPage,
				CustomID: fmt.Sprintf("leaderboard:%s:%d", guildID, page+1)},
		}})
	}
	return b, comps, nil
}

func handleLeaderboardComponent(c *core.Context, customID string) {
	parts := strings.Split(customID, ":")
	if len(parts) != 3 {
		return
	}
	page, _ := strconv.Atoi(parts[2])
	e, comps, err := buildLeaderboard(c, parts[1], page)
	if err != nil {
		return
	}
	_ = c.Session.InteractionRespond(c.Interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{e}, Components: comps},
	})
}
