package system

import (
	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
)

type category struct {
	Key         string
	Title       string
	Description string
	Commands    map[string]string
}

var categories = []category{
	{Key: "moderation", Title: "Moderation", Description: "Tools to keep your server safe.", Commands: map[string]string{
		"/ban": "Ban a member", "/unban": "Unban a user", "/kick": "Kick a member",
		"/timeout": "Timeout a member", "/warning": "Manage warnings", "/rapsheet": "View moderation history",
		"/clear": "Bulk delete messages", "/lock": "Lock a channel", "/unlock": "Unlock a channel",
		"/massban": "Ban many users", "/automod": "Configure automod",
	}},
	{Key: "levels", Title: "Levels", Description: "XP and ranking system.", Commands: map[string]string{
		"/rank": "Show a rank card", "/leaderboard": "Show the leaderboard", "/setlevel": "Override a level",
	}},
	{Key: "music", Title: "Music", Description: "Voice playback.", Commands: map[string]string{
		"/play": "Play a track", "/pause": "Pause", "/resume": "Resume", "/skip": "Skip",
		"/stop": "Stop", "/leave": "Disconnect", "/queue": "Show queue", "/nowplaying": "Now playing", "/volume": "Set volume",
	}},
	{Key: "fun", Title: "Fun", Description: "Entertainment commands.", Commands: map[string]string{
		"/advice": "Random advice", "/fact": "Random fact", "/flip": "Coin flip", "/cat": "Cat image",
		"/dog": "Dog image", "/capybara": "Capybara image", "/meme": "Random meme", "/wiki": "Wikipedia",
		"/uwuify": "uwuify text", "/tweet": "Fake tweet", "/threats": "Stern warnings",
		"/urmom": "Yo-mama joke", "/tiktokdownload": "Download TikTok", "/ytdownload": "Download YouTube",
	}},
	{Key: "user", Title: "User", Description: "User utilities.", Commands: map[string]string{
		"/avatar": "Show avatar", "/userinfo": "User information", "/together": "Watch Together",
	}},
	{Key: "voice", Title: "Voice", Description: "Join-to-create channel management.", Commands: map[string]string{
		"/voice": "Manage your voice channel",
	}},
	{Key: "reactionroles", Title: "Reaction Roles", Description: "Self-assignable roles.", Commands: map[string]string{
		"/reactionroles": "Manage reaction role menus",
	}},
	{Key: "system", Title: "System", Description: "Bot configuration and utilities.", Commands: map[string]string{
		"/afk": "Set AFK", "/help": "This menu", "/translate": "Translate text",
		"/setup": "Configure Specter", "/modlogs": "Configure log routing",
	}},
}

func helpCategoryChoices() []*discordgo.ApplicationCommandOptionChoice {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(categories))
	for _, cat := range categories {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: cat.Title, Value: cat.Key})
	}
	return choices
}

func handleHelp(c *core.Context) {
	selected := c.StringOpt("category", "")

	if selected == "" {
		b := c.Embed().Title("Specter — Help").
			Description("A professional administration and utility bot. Use `/help <category>` for command details.")
		for _, cat := range categories {
			if c.GuildID != "" && c.Gate != nil {
				if ok, _ := c.Gate.Check(c.Interaction, cat.Key, 0); !ok {
					continue
				}
			}
			b.Field(cat.Title, cat.Description, false)
		}
		_ = c.Reply(b.Build())
		return
	}

	for _, cat := range categories {
		if cat.Key != selected {
			continue
		}
		b := c.Embed().Title("Help — " + cat.Title).Description(cat.Description)
		for name, desc := range cat.Commands {
			b.Field(name, desc, true)
		}
		_ = c.Reply(b.Build())
		return
	}
	_ = c.Errorf("Unknown help category.", nil)
}
