package music

import (
	"fmt"
	"strings"

	"github.com/0xSalik/specter/internal/core"
	musicsvc "github.com/0xSalik/specter/internal/music"
)

func handleQueue(c *core.Context) {
	p, ok := c.Music.Get(c.GuildID)
	if !ok {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}
	current, _ := p.Current()
	tracks := p.Queue().List()

	b := c.Embed().Title("Queue").Timestamp()
	if current.Title != "" {
		b.Field("Now Playing", fmt.Sprintf("**%s** • <@%s>", current.Title, current.Requester), false)
	}
	if len(tracks) == 0 {
		b.Description("The queue is empty.")
	} else {
		var sb strings.Builder
		limit := len(tracks)
		if limit > 10 {
			limit = 10
		}
		for i := 0; i < limit; i++ {
			fmt.Fprintf(&sb, "**%d.** %s\n", i+1, tracks[i].Title)
		}
		if len(tracks) > 10 {
			fmt.Fprintf(&sb, "…and %d more", len(tracks)-10)
		}
		b.Description(sb.String())
	}
	_ = c.Reply(b.Build())
}

func handleNowPlaying(c *core.Context) {
	p, ok := c.Music.Get(c.GuildID)
	if !ok {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}
	current, elapsed := p.Current()
	if current.Title == "" {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}

	b := c.Embed().Title("Now Playing").Description(fmt.Sprintf("**%s**", current.Title)).
		Field("Requested by", fmt.Sprintf("<@%s>", current.Requester), true).
		Field("State", p.State().String(), true).Timestamp()

	if current.Duration > 0 {
		b.Field("Progress", progressBar(elapsed, current.Duration), false)
	}
	_ = c.Reply(b.Build())
}

func progressBar(elapsed, total int) string {
	if total <= 0 {
		return ""
	}
	if elapsed > total {
		elapsed = total
	}
	const slots = 20
	filled := elapsed * slots / total
	bar := strings.Repeat("█", filled) + strings.Repeat("─", slots-filled)
	return fmt.Sprintf("`%s`\n%s / %s", bar, formatSeconds(elapsed), formatSeconds(total))
}

var _ = musicsvc.StateIdle
