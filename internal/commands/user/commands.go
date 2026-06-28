// Package user implements user-info utility commands: /avatar, /userinfo and
// /together (Watch Together activity).
package user

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
)

const group = "user"

// youTubeTogetherAppID is Discord's YouTube "Watch Together" activity app ID.
const youTubeTogetherAppID = "880218394199220334"

// Register wires the user commands into the router.
func Register(r *core.Router) {
	r.Register(core.Command{Group: group, Handler: handleAvatar, Def: &discordgo.ApplicationCommand{
		Name: "avatar", Description: "Show a user's avatar",
		Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User", Required: false}},
	}})
	r.Register(core.Command{Group: group, Handler: handleUserinfo, Def: &discordgo.ApplicationCommand{
		Name: "userinfo", Description: "Show information about a user",
		Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User", Required: false}},
	}})
	r.Register(core.Command{Group: group, Handler: handleTogether, Def: &discordgo.ApplicationCommand{
		Name: "together", Description: "Start a YouTube Watch Together activity",
	}})
}

func targetUser(c *core.Context) *discordgo.User {
	if u := c.UserOpt("user"); u != nil {
		return u
	}
	if c.Interaction.Member != nil {
		return c.Interaction.Member.User
	}
	return c.Interaction.User
}

func handleAvatar(c *core.Context) {
	u := targetUser(c)
	if u == nil {
		_ = c.Errorf("Could not resolve the user.", nil)
		return
	}
	b := c.Embed().Title(fmt.Sprintf("%s's Avatar", u.Username)).Image(u.AvatarURL("512"))

	if member, err := c.Session.GuildMember(c.GuildID, u.ID); err == nil && member.Avatar != "" {
		serverURL := member.AvatarURL("512")
		if serverURL != "" && serverURL != u.AvatarURL("512") {
			b.Field("Server Avatar", "Shown below differs from the global avatar.", false)
			b.Thumbnail(serverURL)
		}
	}
	_ = c.Reply(b.Build())
}

func handleUserinfo(c *core.Context) {
	u := targetUser(c)
	if u == nil {
		_ = c.Errorf("Could not resolve the user.", nil)
		return
	}
	created, _ := discordgo.SnowflakeTimestamp(u.ID)

	b := c.Embed().Title("User Information").Thumbnail(u.AvatarURL("256")).
		Field("Username", u.Username, true).
		Field("ID", u.ID, true).
		Field("Bot", boolStr(u.Bot), true).
		Field("Account Created", created.Format("2006-01-02"), true)

	if member, err := c.Session.GuildMember(c.GuildID, u.ID); err == nil {
		if !member.JoinedAt.IsZero() {
			b.Field("Joined Server", member.JoinedAt.Format("2006-01-02"), true)
		}
		b.Field("Roles", formatRoles(member.Roles), false)
		if hr := highestRole(c.Session, c.GuildID, member.Roles); hr != nil {
			b.Field("Highest Role", hr.Name, true)
			if hr.Color != 0 {
				b.Color(hr.Color)
			}
		}
	}
	_ = c.Reply(b.Build())
}

func handleTogether(c *core.Context) {
	_ = c.Defer(false)
	vcID, err := userVoice(c)
	if err != nil {
		_ = c.Errorf(err.Error(), nil)
		return
	}
	invite, err := c.Session.ChannelInviteCreate(vcID, discordgo.Invite{
		MaxAge:            3600,
		TargetType:        2, // embedded application
		TargetApplication: &discordgo.Application{ID: youTubeTogetherAppID},
	})
	if err != nil {
		_ = c.Errorf("Failed to create the Watch Together activity. The bot needs Create Invite permission.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Watch Together").
		Description(fmt.Sprintf("[Click here to start watching together](https://discord.gg/%s)", invite.Code)).AsSuccess().Build())
}

func userVoice(c *core.Context) (string, error) {
	if vs, err := c.Session.State.VoiceState(c.GuildID, c.UserID); err == nil && vs != nil && vs.ChannelID != "" {
		return vs.ChannelID, nil
	}
	if g, err := c.Session.State.Guild(c.GuildID); err == nil {
		for _, vs := range g.VoiceStates {
			if vs.UserID == c.UserID && vs.ChannelID != "" {
				return vs.ChannelID, nil
			}
		}
	}
	return "", errors.New("you must be in a voice channel to start Watch Together")
}

func formatRoles(roles []string) string {
	if len(roles) == 0 {
		return "None"
	}
	limit := len(roles)
	extra := 0
	if limit > 15 {
		extra = limit - 15
		limit = 15
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, "<@&"+roles[i]+">")
	}
	out := strings.Join(parts, " ")
	if extra > 0 {
		out += fmt.Sprintf(" and %d more", extra)
	}
	return out
}

func highestRole(s *discordgo.Session, guildID string, roleIDs []string) *discordgo.Role {
	g, err := s.State.Guild(guildID)
	if err != nil || g == nil {
		return nil
	}
	byID := make(map[string]*discordgo.Role, len(g.Roles))
	for _, r := range g.Roles {
		byID[r.ID] = r
	}
	var highest *discordgo.Role
	for _, id := range roleIDs {
		if r, ok := byID[id]; ok {
			if highest == nil || r.Position > highest.Position {
				highest = r
			}
		}
	}
	return highest
}

func boolStr(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
