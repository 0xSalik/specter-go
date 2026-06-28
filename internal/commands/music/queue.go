package music

import (
	"fmt"
	"strings"
	"time"

	"github.com/0xSalik/specter/internal/core"
)

func handleQueue(c *core.Context) {
	p, ok := c.Music.Get(c.GuildID)
	if !ok {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}
	current, _, playing := p.Current()
	tracks := p.QueueList()

	b := c.Embed().Title("Queue").Timestamp()
	if playing {
		b.Field("Now Playing", fmt.Sprintf("%s • <@%s>", trackLink(current), current.Requester), false)
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
			fmt.Fprintf(&sb, "**%d.** %s • <@%s>\n", i+1, trackLink(tracks[i]), tracks[i].Requester)
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
	current, position, playing := p.Current()
	if !playing {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}

	b := c.Embed().Title("Now Playing").Description(trackLink(current)).
		Field("Requested by", fmt.Sprintf("<@%s>", current.Requester), true).
		Field("State", p.State().String(), true).Timestamp()

	if current.Author() != "" {
		b.Field("Author", current.Author(), true)
	}
	if current.IsStream() {
		b.Field("Progress", "🔴 Live stream", false)
	} else if d := current.Duration(); d > 0 {
		b.Field("Progress", progressBar(position, d), false)
	}
	_ = c.Reply(b.Build())
}

func progressBar(elapsed, total time.Duration) string {
	if total <= 0 {
		return ""
	}
	if elapsed > total {
		elapsed = total
	}
	const slots = 20
	filled := int(elapsed * slots / total)
	if filled > slots {
		filled = slots
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("─", slots-filled)
	return fmt.Sprintf("`%s`\n%s / %s", bar, formatDuration(elapsed), formatDuration(total))
}
