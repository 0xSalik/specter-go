package music

import (
	"context"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Manager owns all per-guild players and the shared encoder configuration.
type Manager struct {
	session  *discordgo.Session
	resolver *Resolver
	ffmpeg   string
	dca      string

	players sync.Map // guildID -> *Player
}

// NewManager constructs a Manager.
func NewManager(session *discordgo.Session, ytdlpPath, ffmpegPath, dcaPath string) *Manager {
	return &Manager{
		session:  session,
		resolver: NewResolver(ytdlpPath),
		ffmpeg:   ffmpegPath,
		dca:      dcaPath,
	}
}

// Resolver exposes the yt-dlp resolver.
func (m *Manager) Resolver() *Resolver { return m.resolver }

// Get returns the player for a guild if one exists.
func (m *Manager) Get(guildID string) (*Player, bool) {
	v, ok := m.players.Load(guildID)
	if !ok {
		return nil, false
	}
	return v.(*Player), true
}

func (m *Manager) getOrCreate(guildID string) *Player {
	v, _ := m.players.LoadOrStore(guildID, newPlayer(guildID, m))
	return v.(*Player)
}

// Play ensures a voice connection to voiceChannelID, enqueues the track, and
// starts the playback loop if the player is idle. Returns the player.
func (m *Manager) Play(guildID, voiceChannelID string, track Track) (*Player, error) {
	p := m.getOrCreate(guildID)

	p.mu.Lock()
	needJoin := p.vc == nil || p.vc.Status != discordgo.VoiceConnectionStatusReady
	p.mu.Unlock()

	if needJoin {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		vc, err := m.session.ChannelVoiceJoin(ctx, guildID, voiceChannelID, false, true)
		if err != nil {
			return nil, err
		}
		// Discord enforces the DAVE (E2EE) handshake on voice; wait for it to
		// complete before streaming so frames are not dropped pre-encryption.
		if err := vc.WaitForDAVEReady(ctx); err != nil {
			_ = vc.Disconnect(context.Background())
			return nil, err
		}
		p.mu.Lock()
		p.vc = vc
		p.mu.Unlock()
	}

	p.queue.Enqueue(track)

	p.mu.Lock()
	start := !p.running
	if start {
		p.running = true
	}
	p.mu.Unlock()

	if start {
		go p.runLoop()
	}
	return p, nil
}

// Leave disconnects from voice and discards the player state for a guild.
func (m *Manager) Leave(guildID string) error {
	p, ok := m.Get(guildID)
	if !ok {
		return errNoVoice
	}
	p.Stop()
	p.mu.Lock()
	vc := p.vc
	p.vc = nil
	p.mu.Unlock()
	if vc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = vc.Disconnect(ctx)
		cancel()
	}
	m.players.Delete(guildID)
	return nil
}

// Shutdown stops all players and disconnects every voice connection.
func (m *Manager) Shutdown(ctx context.Context) {
	m.players.Range(func(key, value any) bool {
		p := value.(*Player)
		p.Stop()
		p.mu.Lock()
		vc := p.vc
		p.vc = nil
		p.mu.Unlock()
		if vc != nil {
			_ = vc.Disconnect(ctx)
		}
		return true
	})
}
