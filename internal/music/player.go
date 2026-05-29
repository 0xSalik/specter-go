package music

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

// State enumerates the player lifecycle states.
type State int

const (
	StateIdle State = iota
	StatePlaying
	StatePaused
	StateStopped
)

func (s State) String() string {
	switch s {
	case StatePlaying:
		return "Playing"
	case StatePaused:
		return "Paused"
	case StateStopped:
		return "Stopped"
	default:
		return "Idle"
	}
}

// Player owns the playback loop and voice connection for a single guild.
type Player struct {
	guildID string
	mgr     *Manager

	mu        sync.Mutex
	state     State
	queue     *Queue
	vc        *discordgo.VoiceConnection
	current   Track
	volume    int
	startedAt time.Time
	running   bool

	skipCh  chan struct{}
	stopCh  chan struct{}
	pauseCh chan bool
}

func newPlayer(guildID string, mgr *Manager) *Player {
	return &Player{
		guildID: guildID,
		mgr:     mgr,
		queue:   NewQueue(),
		volume:  256,
		state:   StateIdle,
		skipCh:  make(chan struct{}, 1),
		stopCh:  make(chan struct{}, 1),
		pauseCh: make(chan bool, 1),
	}
}

// State returns the current player state.
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Queue exposes the underlying queue.
func (p *Player) Queue() *Queue { return p.queue }

// Current returns the currently playing track and elapsed seconds.
func (p *Player) Current() (Track, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	elapsed := 0
	if !p.startedAt.IsZero() && p.state == StatePlaying {
		elapsed = int(time.Since(p.startedAt).Seconds())
	}
	return p.current, elapsed
}

// Volume returns the configured volume (0..256 scale, displayed as 1..100).
func (p *Player) Volume() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

// SetVolume sets the playback volume. percent is 1..100.
func (p *Player) SetVolume(percent int) {
	if percent < 1 {
		percent = 1
	}
	if percent > 100 {
		percent = 100
	}
	p.mu.Lock()
	p.volume = percent * 256 / 100
	p.mu.Unlock()
}

// Pause requests playback to pause.
func (p *Player) Pause() bool {
	p.mu.Lock()
	if p.state != StatePlaying {
		p.mu.Unlock()
		return false
	}
	p.state = StatePaused
	p.mu.Unlock()
	select {
	case p.pauseCh <- true:
	default:
	}
	return true
}

// Resume requests playback to resume.
func (p *Player) Resume() bool {
	p.mu.Lock()
	if p.state != StatePaused {
		p.mu.Unlock()
		return false
	}
	p.state = StatePlaying
	p.mu.Unlock()
	select {
	case p.pauseCh <- false:
	default:
	}
	return true
}

// Skip skips the current track.
func (p *Player) Skip() {
	select {
	case p.skipCh <- struct{}{}:
	default:
	}
}

// Stop stops playback and clears the queue.
func (p *Player) Stop() {
	p.queue.Clear()
	select {
	case p.stopCh <- struct{}{}:
	default:
	}
}

// runLoop drains the queue, encoding and streaming each track until exhausted,
// stopped, or the voice connection is lost. It owns transitions back to Idle.
func (p *Player) runLoop() {
	defer func() {
		p.mu.Lock()
		p.running = false
		p.state = StateIdle
		p.current = Track{}
		p.mu.Unlock()
	}()

	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		track, ok := p.queue.Dequeue()
		if !ok {
			return // queue exhausted
		}

		p.mu.Lock()
		p.current = track
		p.state = StatePlaying
		p.startedAt = time.Now()
		vc := p.vc
		vol := p.volume
		p.mu.Unlock()

		if vc == nil {
			return
		}

		if stopped := p.playTrack(vc, track, vol); stopped {
			return
		}
	}
}

// playTrack streams a single track. It returns true if the player was stopped
// (queue cleared / leave) and the loop should terminate.
func (p *Player) playTrack(vc *discordgo.VoiceConnection, track Track, vol int) (stopped bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := EncodeStream(ctx, p.mgr.ffmpeg, p.mgr.dca, track.URL, vol)
	if err != nil {
		log.Error().Err(err).Str("guild", p.guildID).Str("track", track.Title).Msg("music: encode failed")
		return false // skip to next track
	}
	defer stream.Stop()

	_ = vc.Speaking(true)
	defer vc.Speaking(false) //nolint:errcheck

	for {
		select {
		case <-p.stopCh:
			return true
		case <-p.skipCh:
			return false
		case paused := <-p.pauseCh:
			if paused {
				if stop := p.waitResume(); stop {
					return true
				}
			}
		case frame, ok := <-stream.Frames:
			if !ok {
				return false // track finished
			}
			select {
			case vc.OpusSend <- frame:
			case <-time.After(2 * time.Second):
				return false // voice connection stalled; move on
			case <-p.stopCh:
				return true
			}
		}
	}
}

// waitResume blocks while paused. Returns true if stopped while paused.
func (p *Player) waitResume() bool {
	for {
		select {
		case <-p.stopCh:
			return true
		case <-p.skipCh:
			return false
		case paused := <-p.pauseCh:
			if !paused {
				return false
			}
		}
	}
}

var errNoVoice = errors.New("not connected to a voice channel")
