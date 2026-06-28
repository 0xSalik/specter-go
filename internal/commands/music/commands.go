// Package music implements the playback slash commands backed by the Lavalink
// player manager.
package music

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/core"
	musicsvc "github.com/0xSalik/specter/internal/music"
)

const group = "music"

// Register wires all music commands into the router.
func Register(r *core.Router) {
	r.Register(core.Command{Group: group, Handler: handlePlay, Def: &discordgo.ApplicationCommand{
		Name: "play", Description: "Play a track from YouTube, YouTube Music, Spotify, or SoundCloud",
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
	query := strings.TrimSpace(c.StringOpt("query", ""))
	if query == "" {
		_ = c.Errorf("You must provide a search term or URL.", nil)
		return
	}

	_ = c.Defer(false)

	if !c.Music.Ready() {
		_ = c.Errorf("Music is currently unavailable: the Lavalink audio server is not connected. Make sure it's running and try again.", nil)
		return
	}

	vcID, err := userVoiceChannel(c)
	if err != nil {
		_ = c.Errorf(err.Error(), nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := c.Music.Load(ctx, query)
	if err != nil {
		if errors.Is(err, musicsvc.ErrNoMatches) {
			_ = c.Errorf("No results found for that query.", nil)
			return
		}
		_ = c.Errorf("Could not load that track: "+err.Error(), err)
		return
	}

	started, position, err := c.Music.Play(ctx, c.GuildID, vcID, c.UserID, result.Tracks)
	if err != nil {
		_ = c.Errorf("Failed to join the voice channel or start playback.", err)
		return
	}

	b := c.Embed().Timestamp()
	switch {
	case result.Playlist != "":
		b.Title("Added Playlist to Queue").
			Description(fmt.Sprintf("**%s**", result.Playlist)).
			Field("Tracks", fmt.Sprintf("%d", len(result.Tracks)), true)
		if !started {
			b.Field("Starting At", fmt.Sprintf("#%d", position), true)
		}
	default:
		track := musicsvc.Track{Track: result.Tracks[0]}
		if started {
			b.Title("Now Playing").Description(trackLink(track))
		} else {
			b.Title("Added to Queue").Description(trackLink(track)).
				Field("Position", fmt.Sprintf("#%d", position), true)
		}
		if d := track.Duration(); d > 0 {
			b.Field("Duration", formatDuration(d), true)
		}
	}
	_ = c.Reply(b.Build())
}

func handlePause(c *core.Context) {
	p, ok := c.Music.Get(c.GuildID)
	if !ok {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	paused, err := p.Pause(ctx)
	if err != nil {
		_ = c.Errorf("Failed to pause playback.", err)
		return
	}
	if !paused {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}
	_ = c.Success("Paused", "Playback has been paused.")
}

func handleResume(c *core.Context) {
	p, ok := c.Music.Get(c.GuildID)
	if !ok {
		_ = c.Errorf("Playback is not paused.", nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resumed, err := p.Resume(ctx)
	if err != nil {
		_ = c.Errorf("Failed to resume playback.", err)
		return
	}
	if !resumed {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	next, err := p.Skip(ctx)
	if err != nil {
		if errors.Is(err, musicsvc.ErrNoVoice) {
			_ = c.Errorf("Nothing is currently playing.", nil)
			return
		}
		_ = c.Errorf("Failed to skip the track.", err)
		return
	}
	if next == nil {
		_ = c.Success("Skipped", "Skipped the current track. The queue is now empty.")
		return
	}
	_ = c.Success("Skipped", fmt.Sprintf("Now playing %s.", trackLink(*next)))
}

func handleStop(c *core.Context) {
	p, ok := c.Music.Get(c.GuildID)
	if !ok {
		_ = c.Errorf("Nothing is currently playing.", nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		_ = c.Errorf("Failed to stop playback.", err)
		return
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.SetVolume(ctx, level); err != nil {
		_ = c.Errorf("Failed to set the volume.", err)
		return
	}
	_ = c.Success("Volume Set", fmt.Sprintf("Volume set to **%d%%**.", level))
}

func ptrFloat(f float64) *float64 { return &f }

// trackLink renders a track as a markdown link to its source, or bold title.
func trackLink(t musicsvc.Track) string {
	if url := t.URL(); url != "" {
		return fmt.Sprintf("[%s](%s)", t.Title(), url)
	}
	return fmt.Sprintf("**%s**", t.Title())
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%d:%02d", m, sec)
}
