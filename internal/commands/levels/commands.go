// Package levels implements the /rank, /leaderboard and /setlevel commands.
package levels

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/core"
	"github.com/salik/specter/internal/db"
	"github.com/salik/specter/internal/discordutil"
	levelsvc "github.com/salik/specter/internal/levels"
)

const group = "levels"

// Register wires the level commands into the router.
func Register(r *core.Router) {
	r.Register(core.Command{
		Group: group, Handler: handleRank,
		Def: &discordgo.ApplicationCommand{
			Name: "rank", Description: "Show your or another member's rank card",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User", Required: false},
			},
		},
	})
	r.Register(core.Command{
		Group: group, Handler: handleLeaderboard,
		Def: &discordgo.ApplicationCommand{Name: "leaderboard", Description: "Show the server XP leaderboard"},
	})
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionAdministrator, Handler: handleSetLevel,
		Def: &discordgo.ApplicationCommand{
			Name: "setlevel", Description: "Override a member's level",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User", Required: true},
				{Type: discordgo.ApplicationCommandOptionInteger, Name: "level", Description: "New level", Required: true, MinValue: ptrFloat(0), MaxValue: 1000},
			},
		},
	})
	r.RegisterComponent("leaderboard", handleLeaderboardComponent)
}

func handleRank(c *core.Context) {
	target := c.UserOpt("user")
	if target == nil && c.Interaction.Member != nil {
		target = c.Interaction.Member.User
	}
	if target == nil {
		_ = c.Errorf("Could not resolve the target user.", nil)
		return
	}

	_ = c.Defer(false)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	entry, err := c.Store.GetLevel(ctx, c.GuildID, target.ID)
	if err != nil {
		if db.IsNotFound(err) {
			_ = c.Errorf(fmt.Sprintf("%s has not earned any XP yet.", target.Username), nil)
			return
		}
		_ = c.Errorf("Failed to load rank data.", err)
		return
	}
	rank, err := c.Store.GetRank(ctx, c.GuildID, target.ID)
	if err != nil {
		_ = c.Errorf("Failed to compute rank.", err)
		return
	}

	card, err := levelsvc.RenderRankCard(ctx, levelsvc.RankCardData{
		Username:  target.Username,
		Discrim:   target.Discriminator,
		AvatarURL: discordutil.AvatarURL(target),
		Level:     entry.Level,
		Rank:      rank,
		XP:        entry.XP,
		TotalMsgs: entry.TotalMsgs,
	})
	if err != nil {
		// Fall back to a text embed if image rendering fails.
		_ = c.Reply(c.Embed().Title(fmt.Sprintf("%s's Rank", target.Username)).
			Field("Level", strconv.Itoa(entry.Level), true).
			Field("Rank", "#"+strconv.Itoa(rank), true).
			Field("XP", strconv.FormatInt(entry.XP, 10), true).Build())
		return
	}

	e := c.Embed().Title(fmt.Sprintf("%s's Rank", target.Username)).
		Description(fmt.Sprintf("Level **%d** • Rank **#%d** • **%d** XP", entry.Level, rank, entry.XP)).
		Image("attachment://rank.png").Build()
	_ = c.ReplyFile(e, "rank.png", card)
}

func handleSetLevel(c *core.Context) {
	target := c.UserOpt("user")
	level := c.IntOpt("level", -1)
	if target == nil || level < 0 {
		_ = c.Errorf("A user and a non-negative level are required.", nil)
		return
	}
	_ = c.Defer(false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	xp := levelsvc.CalculateXPForLevel(level)
	if err := c.Store.SetLevel(ctx, c.GuildID, target.ID, level, xp); err != nil {
		_ = c.Errorf("Failed to update the level.", err)
		return
	}
	_ = c.Success("Level Updated", fmt.Sprintf("%s is now level **%d** (%d XP).", target.Username, level, xp))
}

func ptrFloat(f float64) *float64 { return &f }

var _ = strings.TrimSpace
