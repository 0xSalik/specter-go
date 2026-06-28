package music

import (
	"context"
	"sync"
	"time"

	"github.com/disgoorg/disgolink/v4/disgolink"
)

// State enumerates the player lifecycle states.
type State int

const (
	StateIdle State = iota
	StatePlaying
	StatePaused
)

func (s State) String() string {
	switch s {
	case StatePlaying:
		return "Playing"
	case StatePaused:
		return "Paused"
	default:
		return "Idle"
	}
}

// GuildPlayer holds the queue and now-playing metadata for one guild. Actual
// playback is performed by the Lavalink node; this type tracks what we asked it
// to play and translates commands into Lavalink player updates.
type GuildPlayer struct {
	guildID string
	mgr     *Manager

	mu      sync.Mutex
	queue   []Track
	current *Track
	vcID    string
	volume  int
}

func newGuildPlayer(guildID string, mgr *Manager) *GuildPlayer {
	return &GuildPlayer{guildID: guildID, mgr: mgr, volume: 100}
}

// Current returns the currently playing track and the live playback position.
// ok is false when nothing is playing.
func (gp *GuildPlayer) Current() (track Track, position time.Duration, ok bool) {
	gp.mu.Lock()
	cur := gp.current
	gp.mu.Unlock()
	if cur == nil {
		return Track{}, 0, false
	}
	var pos time.Duration
	if p, ready := gp.mgr.lavaPlayer(gp.guildID); ready {
		pos = time.Duration(p.Position()) * time.Millisecond
	}
	return *cur, pos, true
}

// State reports whether the player is idle, playing, or paused.
func (gp *GuildPlayer) State() State {
	gp.mu.Lock()
	cur := gp.current
	gp.mu.Unlock()
	if cur == nil {
		return StateIdle
	}
	if p, ok := gp.mgr.lavaPlayer(gp.guildID); ok && p.Paused {
		return StatePaused
	}
	return StatePlaying
}

// QueueList returns a copy of the pending tracks (excluding the current track).
func (gp *GuildPlayer) QueueList() []Track {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	out := make([]Track, len(gp.queue))
	copy(out, gp.queue)
	return out
}

// QueueLen returns the number of pending tracks.
func (gp *GuildPlayer) QueueLen() int {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	return len(gp.queue)
}

// Volume returns the configured volume (1..100, Lavalink scale).
func (gp *GuildPlayer) Volume() int {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	return gp.volume
}

// Pause pauses playback. Returns false if nothing is playing.
func (gp *GuildPlayer) Pause(ctx context.Context) (bool, error) {
	gp.mu.Lock()
	cur := gp.current
	gp.mu.Unlock()
	if cur == nil {
		return false, nil
	}
	p, ok := gp.mgr.lavaPlayer(gp.guildID)
	if !ok {
		return false, ErrNotReady
	}
	if p.Paused {
		return false, nil
	}
	return true, p.Update(ctx, disgolink.WithPaused(true))
}

// Resume resumes playback. Returns false if it was not paused.
func (gp *GuildPlayer) Resume(ctx context.Context) (bool, error) {
	gp.mu.Lock()
	cur := gp.current
	gp.mu.Unlock()
	if cur == nil {
		return false, nil
	}
	p, ok := gp.mgr.lavaPlayer(gp.guildID)
	if !ok {
		return false, ErrNotReady
	}
	if !p.Paused {
		return false, nil
	}
	return true, p.Update(ctx, disgolink.WithPaused(false))
}

// Skip stops the current track and starts the next queued track, if any. It
// returns the track that started playing, or nil when the queue is now empty.
func (gp *GuildPlayer) Skip(ctx context.Context) (*Track, error) {
	p, ok := gp.mgr.lavaPlayer(gp.guildID)
	if !ok {
		return nil, ErrNotReady
	}
	gp.mu.Lock()
	if gp.current == nil {
		gp.mu.Unlock()
		return nil, ErrNoVoice
	}
	if len(gp.queue) == 0 {
		gp.current = nil
		gp.mu.Unlock()
		return nil, p.Update(ctx, disgolink.WithNullTrack())
	}
	next := gp.queue[0]
	gp.queue = gp.queue[1:]
	gp.current = &next
	gp.mu.Unlock()
	// Replacing the track ends the previous one with reason "replaced", which
	// does not trigger auto-advance, so we drive the transition explicitly here.
	return &next, p.Update(ctx, disgolink.WithTrack(next.Track))
}

// Stop clears the queue and stops playback without leaving the channel.
func (gp *GuildPlayer) Stop(ctx context.Context) error {
	p, ok := gp.mgr.lavaPlayer(gp.guildID)
	if !ok {
		return ErrNotReady
	}
	gp.mu.Lock()
	gp.queue = nil
	gp.current = nil
	gp.mu.Unlock()
	return p.Update(ctx, disgolink.WithNullTrack())
}

// SetVolume sets playback volume (1..100), applied immediately.
func (gp *GuildPlayer) SetVolume(ctx context.Context, percent int) error {
	if percent < 1 {
		percent = 1
	}
	if percent > 100 {
		percent = 100
	}
	gp.mu.Lock()
	gp.volume = percent
	gp.mu.Unlock()
	p, ok := gp.mgr.lavaPlayer(gp.guildID)
	if !ok {
		return ErrNotReady
	}
	return p.Update(ctx, disgolink.WithVolume(percent))
}
