// Package music implements the playback slash commands backed by the
// per-guild player manager.
package music

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/core"
	musicsvc "github.com/salik/specter/internal/music"
)

const group = "music"

// Register wires all music commands into the router.
func Register(r *core.Router) {
	r.Register(core.Command{Group: group, Handler: handlePlay, Def: &discordgo.ApplicationCommand{
		Name: "play", Description: "Play a track or add it to the queue",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "query", Description: "Search term or URL", Required: true},
		},
	}})
	r.Register(core.Command{Group: group, Handler: handlePause, Def: &discordgo.ApplicationCommand{Name: "pause", Description: "Pause playback"}})
	r.Register(core.Command{Group: group, Handler: handleResume, Def: &discordgo.ApplicationCommand{Name: "resume", Description: "Resume playback"}})
	r.Register(core.Command{Group: group, Handler: handleSkip, Def: &discordgo.ApplicationCommand{Name: "skip", Description: "Skip the current track"}})
	r.Register(core.Command{Group: group, Handler: handleStop, Def: &discordgo.ApplicationCommand{Name: "stop", Description: "Stop playback and clear the queue"}})
	r.Register(core.Command{Group: group, Handler: handleLeave, Def: &discordgo.ApplicationCommand{Name: "leave", Description: "Disconnect from voice"}})
	r.Register(core.Command{Group: group, Handler: handleQueue, Def: &discordgo.ApplicationCommand{Name: "queue", Description: "Show the queue"}})
	r.Register(core.Command{Group: group, Handler: handleNowPlaying, Def: &discordgo.ApplicationCommand{Name: "nowplaying", Description: "Show the current track"}})
	r.Register(core.Command{Group: group, Handler: handleVolume, Def: &discordgo.ApplicationCommand{
		Name: "volume", Description: "Set playback volume (1-100)",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "level", Description: "1-100", Required: true, MinValue: ptrFloat(1), MaxValue: 100},
		},
	}})
}

// userVoiceChannel returns the voice channel ID the invoking user is in.
func userVoiceChannel(c *core.Context) (string, error) {
	if vs, err := c.Session.State.VoiceState(c.GuildID, c.UserID); err == nil && vs != nil && vs.ChannelID != "" {
		return vs.ChannelID, nil
	}
	g, err := c.Session.State.Guild(c.GuildID)
	if err != nil || g == nil {
		return "", errors.New("you must be in a voice channel to use this command")
	}
	for _, vs := range g.VoiceStates {
		if vs.UserID == c.UserID && vs.ChannelID != "" {
			return vs.ChannelID, nil
		}
	}
	return "", errors.New("you must be in a voice channel to use this command")
}

func handlePlay(c *core.Context) {
	query := c.StringOpt("query", "")
	if strings.TrimSpace(query) == "" {
		_ = c.Errorf("You must provide a search term or URL.", nil)
		return
	}

	_ = c.Defer(false)

	if !c.Music.Resolver().Available() {
		_ = c.Errorf("Music is unavailable: yt-dlp is not installed on the host. Install it from https://github.com/yt-dlp/yt-dlp.", nil)
		return
	}

	vcID, err := userVoiceChannel(c)
	if err != nil {
		_ = c.Errorf(err.Error(), nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	track, err := c.Music.Resolver().Resolve(ctx, query)
	if err != nil {
		_ = c.Errorf("Could not resolve that track: "+err.Error(), err)
		return
	}
	track.Requester = c.UserID

	player, err := c.Music.Play(c.GuildID, vcID, *track)
	if err != nil {
		_ = c.Errorf("Failed to join the voice channel or start playback.", err)
		return
	}

	position := player.Queue().Len()
	b := c.Embed().Timestamp()
	if position == 0 && player.State() == musicsvc.StatePlaying {
		b.Title("Now Playing").Description(fmt.Sprintf("**%s**", track.Title))
	} else {
		b.Title("Added to Queue").Description(fmt.Sprintf("**%s**", track.Title)).
			Field("Position", fmt.Sprintf("#%d", position), true)
	}
	if track.Duration > 0 {
		b.Field("Duration", formatSeconds(track.Duration), true)
	}
	_ = c.Reply(b.Build())
}

func handlePause(c *core.Context) {
	p, ok := c.Music.Get(c.GuildID)
	if !ok || !p.Pause() {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}
	_ = c.Success("Paused", "Playback has been paused.")
}

func handleResume(c *core.Context) {
	p, ok := c.Music.Get(c.GuildID)
	if !ok || !p.Resume() {
		_ = c.Errorf("Playback is not paused.", nil)
		return
	}
	_ = c.Success("Resumed", "Playback has resumed.")
}

func handleSkip(c *core.Context) {
	p, ok := c.Music.Get(c.GuildID)
	if !ok {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}
	p.Skip()
	if p.Queue().Len() == 0 {
		_ = c.Success("Skipped", "Skipped the current track. The queue is now empty.")
		return
	}
	_ = c.Success("Skipped", "Skipped to the next track.")
}

func handleStop(c *core.Context) {
	p, ok := c.Music.Get(c.GuildID)
	if !ok {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}
	p.Stop()
	_ = c.Success("Stopped", "Playback stopped and the queue cleared.")
}

func handleLeave(c *core.Context) {
	if err := c.Music.Leave(c.GuildID); err != nil {
		_ = c.Errorf("I'm not connected to a voice channel.", nil)
		return
	}
	_ = c.Success("Disconnected", "Left the voice channel.")
}

func handleVolume(c *core.Context) {
	level := c.IntOpt("level", 0)
	p, ok := c.Music.Get(c.GuildID)
	if !ok {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}
	p.SetVolume(level)
	_ = c.Success("Volume Set", fmt.Sprintf("Volume set to **%d%%**. It applies to the next track.", level))
}

func ptrFloat(f float64) *float64 { return &f }

func formatSeconds(s int) string {
	d := time.Duration(s) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%d:%02d", m, sec)
}
