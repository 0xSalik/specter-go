// Package fun implements the entertainment commands. Each external call uses a
// typed response struct (never interface{}) and a bounded context.
package fun

import (
	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
)

const group = "fun"

func simple(name, desc string, h core.HandlerFunc) core.Command {
	return core.Command{Group: group, Handler: h, Def: &discordgo.ApplicationCommand{Name: name, Description: desc}}
}

// Register wires all fun commands into the router.
func Register(r *core.Router) {
	r.Register(simple("advice", "Get a random piece of advice", handleAdvice))
	r.Register(simple("fact", "Get a random useless fact", handleFact))
	r.Register(simple("flip", "Flip a coin", handleFlip))
	r.Register(simple("urmom", "Get a yo-mama joke", handleUrmom))
	r.Register(simple("cat", "Get a random cat image", handleCat))
	r.Register(simple("dog", "Get a random dog image", handleDog))
	r.Register(simple("capybara", "Get a random capybara image", handleCapybara))
	r.Register(simple("meme", "Get a random meme", handleMeme))
	r.Register(simple("threats", "Receive a lighthearted threat", handleThreats))

	r.Register(core.Command{Group: group, Handler: handleWiki, Def: &discordgo.ApplicationCommand{
		Name: "wiki", Description: "Search Wikipedia",
		Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "query", Description: "Search term", Required: true}},
	}})
	r.Register(core.Command{Group: group, Handler: handleUwuify, Def: &discordgo.ApplicationCommand{
		Name: "uwuify", Description: "uwuify some text",
		Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "text", Description: "Text", Required: true}},
	}})
	r.Register(core.Command{Group: group, Handler: handleTweet, Def: &discordgo.ApplicationCommand{
		Name: "tweet", Description: "Generate a fake tweet image",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "username", Description: "Display name", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "text", Description: "Tweet text", Required: true},
		},
	}})
	r.Register(core.Command{Group: group, Handler: handleTikTok, Def: &discordgo.ApplicationCommand{
		Name: "tiktokdownload", Description: "Download a TikTok video",
		Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "url", Description: "TikTok URL", Required: true}},
	}})
	r.Register(core.Command{Group: group, Handler: handleYTDownload, Def: &discordgo.ApplicationCommand{
		Name: "ytdownload", Description: "Download a YouTube video (<=720p)",
		Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "url", Description: "YouTube URL", Required: true}},
	}})
}
