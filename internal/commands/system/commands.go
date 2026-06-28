// Package system implements general system commands: /afk, /help and /translate.
package system

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
	"github.com/0xSalik/specter/internal/discordutil"
	"github.com/0xSalik/specter/internal/httpx"
)

const group = "system"

// Register wires the system commands into the router.
func Register(r *core.Router) {
	r.Register(core.Command{Group: group, Handler: handleAFK, Def: &discordgo.ApplicationCommand{
		Name: "afk", Description: "Set your AFK status",
		Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason", Required: false}},
	}})
	r.Register(core.Command{Group: group, Handler: handleHelp, Def: &discordgo.ApplicationCommand{
		Name: "help", Description: "Show command help",
		Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "category", Description: "Category", Required: false,
			Choices: helpCategoryChoices()}},
	}})
	r.Register(core.Command{Group: group, Handler: handleTranslate, Def: &discordgo.ApplicationCommand{
		Name: "translate", Description: "Translate text to another language",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "text", Description: "Text to translate", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "target_language", Description: "Target language code, e.g. es, fr, de", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "source_language", Description: "Source language code (default: en)", Required: false},
		},
	}})
}

func handleAFK(c *core.Context) {
	reason := c.StringOpt("reason", "AFK")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Store.SetAFK(ctx, c.GuildID, c.UserID, reason); err != nil {
		_ = c.Errorf("Failed to set your AFK status.", err)
		return
	}
	_ = c.ReplyEphemeral(c.Embed().Title("AFK Set").Description("You are now AFK: " + reason).AsSuccess().Build())
}

func handleTranslate(c *core.Context) {
	text := c.StringOpt("text", "")
	target := c.StringOpt("target_language", "")
	source := c.StringOpt("source_language", "en")
	if text == "" || target == "" {
		_ = c.Errorf("Text and a target language are required.", nil)
		return
	}
	_ = c.Defer(false)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("https://api.mymemory.translated.net/get?q=%s&langpair=%s",
		url.QueryEscape(text), url.QueryEscape(source+"|"+target))

	var resp struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
		ResponseStatus int `json:"responseStatus"`
	}
	if err := httpx.GetJSON(ctx, endpoint, &resp); err != nil || resp.ResponseData.TranslatedText == "" {
		_ = c.Errorf("Translation failed. Please check the language codes and try again.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Translation").
		Field("Source ("+source+")", truncate(text, 1000), false).
		Field("Target ("+target+")", truncate(resp.ResponseData.TranslatedText, 1000), false).Build())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

var _ = discordutil.FormatDuration
