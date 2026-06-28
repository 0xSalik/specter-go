package music

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgolink/v4/disgolink"
	"github.com/disgoorg/disgolink/v4/lavalink"
	"github.com/disgoorg/snowflake/v2"
	"github.com/rs/zerolog/log"
)

var (
	// ErrNotReady indicates the Lavalink node is not connected yet.
	ErrNotReady = errors.New("music backend (Lavalink) is not connected")
	// ErrNoMatches indicates a query resolved to nothing playable.
	ErrNoMatches = errors.New("no matches found for that query")
	// ErrNoVoice indicates there is no active player for the guild.
	ErrNoVoice = errors.New("not connected to a voice channel")
)

// searchPrefix matches Lavalink search identifiers like "ytsearch:", "scsearch:",
// "spsearch:" so user-supplied prefixes are passed through unchanged.
var searchPrefix = regexp.MustCompile(`^[a-z]{2,}search:`)

// NodeConfig describes how to reach the Lavalink node.
type NodeConfig struct {
	Address  string // host:port, e.g. "localhost:2333"
	Password string
	Secure   bool // true for wss/https
}

// Manager owns the disgolink client, the Lavalink node connection, and the
// per-guild queue state. It is safe for concurrent use.
type Manager struct {
	session *discordgo.Session
	cfg     NodeConfig

	mu   sync.RWMutex
	link *disgolink.Client
	node *disgolink.Node

	players sync.Map // guildID string -> *GuildPlayer
}

// NewManager constructs a Manager. The Lavalink connection is established later
// via Start once the bot's user ID is known.
func NewManager(session *discordgo.Session, cfg NodeConfig) *Manager {
	if cfg.Address == "" {
		cfg.Address = "localhost:2333"
	}
	if cfg.Password == "" {
		cfg.Password = "youshallnotpass"
	}
	return &Manager{session: session, cfg: cfg}
}

// Start creates the disgolink client, connects to the Lavalink node, and wires
// the Discord voice gateway events into disgolink. It blocks until the node
// handshake completes or the context expires.
func (m *Manager) Start(ctx context.Context, botUserID string) error {
	uid, err := snowflake.Parse(botUserID)
	if err != nil {
		return err
	}

	// disgolink logs via slog; route everything to a discard handler so the
	// bot's zerolog output stays clean. We log node lifecycle ourselves.
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	link := disgolink.New(uid,
		disgolink.WithLogger(quiet),
		disgolink.WithListenerFunc(m.onTrackEnd),
		disgolink.WithListenerFunc(m.onTrackException),
		disgolink.WithListenerFunc(m.onTrackStuck),
		disgolink.WithListenerFunc(m.onWebSocketClosed),
	)

	node, err := link.AddNode(ctx, disgolink.NodeConfig{
		Name:     "main",
		Address:  m.cfg.Address,
		Password: m.cfg.Password,
		Secure:   m.cfg.Secure,
	})
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.link = link
	m.node = node
	m.mu.Unlock()

	// Forward only the bot's own voice updates to Lavalink so it can establish
	// (and DAVE-encrypt) the voice connection on our behalf.
	m.session.AddHandler(m.onVoiceStateUpdate)
	m.session.AddHandler(m.onVoiceServerUpdate)

	log.Info().Str("address", m.cfg.Address).Msg("connected to Lavalink node")
	return nil
}

// Ready reports whether the Lavalink node is connected and usable.
func (m *Manager) Ready() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.node != nil && m.node.Status() == disgolink.StatusConnected
}

// LoadResult is the outcome of resolving a query.
type LoadResult struct {
	Tracks   []lavalink.Track
	Playlist string // playlist name when the query loaded a playlist, else ""
}

// Load resolves a query or URL into one or more playable tracks. Plain search
// terms default to a YouTube search; URLs and explicit "xxsearch:" identifiers
// are passed through so Spotify/SoundCloud/YouTube Music links all work.
func (m *Manager) Load(ctx context.Context, query string) (LoadResult, error) {
	m.mu.RLock()
	node := m.node
	m.mu.RUnlock()
	if node == nil {
		return LoadResult{}, ErrNotReady
	}

	identifier := strings.TrimSpace(query)
	if !isURL(identifier) && !searchPrefix.MatchString(identifier) {
		identifier = string(lavalink.SearchTypeYouTube) + ":" + identifier
	}

	result, err := node.Rest.LoadTracks(ctx, identifier)
	if err != nil {
		return LoadResult{}, err
	}

	switch data := result.Data.(type) {
	case lavalink.Track:
		return LoadResult{Tracks: []lavalink.Track{data}}, nil
	case lavalink.Playlist:
		if len(data.Tracks) == 0 {
			return LoadResult{}, ErrNoMatches
		}
		return LoadResult{Tracks: data.Tracks, Playlist: data.Info.Name}, nil
	case lavalink.Search:
		if len(data) == 0 {
			return LoadResult{}, ErrNoMatches
		}
		return LoadResult{Tracks: []lavalink.Track{data[0]}}, nil
	case lavalink.Empty:
		return LoadResult{}, ErrNoMatches
	case lavalink.Exception:
		return LoadResult{}, errors.New(data.Message)
	default:
		return LoadResult{}, ErrNoMatches
	}
}

// Play joins the voice channel (if needed), enqueues the supplied tracks, and
// starts playback when idle. It returns whether the first track started playing
// immediately and, when queued, the queue position of the first added track.
func (m *Manager) Play(ctx context.Context, guildID, voiceChannelID, requester string, tracks []lavalink.Track) (started bool, position int, err error) {
	if !m.Ready() {
		return false, 0, ErrNotReady
	}
	if len(tracks) == 0 {
		return false, 0, ErrNoMatches
	}

	gp := m.getOrCreate(guildID)

	// ChannelVoiceJoinManual only sends the gateway voice-state op; Lavalink
	// owns the actual UDP/DAVE voice connection once it receives the forwarded
	// state and server updates.
	if err := m.session.ChannelVoiceJoinManual(guildID, voiceChannelID, false, false); err != nil {
		return false, 0, err
	}

	gp.mu.Lock()
	gp.vcID = voiceChannelID
	if gp.current == nil {
		first := Track{Track: tracks[0], Requester: requester}
		gp.current = &first
		for _, t := range tracks[1:] {
			gp.queue = append(gp.queue, Track{Track: t, Requester: requester})
		}
		vol := gp.volume
		gp.mu.Unlock()

		player := m.link.Player(snowflake.MustParse(guildID))
		if err := player.Update(ctx, disgolink.WithTrack(tracks[0]), disgolink.WithVolume(vol)); err != nil {
			gp.mu.Lock()
			gp.current = nil
			gp.mu.Unlock()
			return false, 0, err
		}
		return true, 0, nil
	}

	pos := len(gp.queue) + 1
	for _, t := range tracks {
		gp.queue = append(gp.queue, Track{Track: t, Requester: requester})
	}
	gp.mu.Unlock()
	return false, pos, nil
}

// Get returns the player for a guild if one exists.
func (m *Manager) Get(guildID string) (*GuildPlayer, bool) {
	v, ok := m.players.Load(guildID)
	if !ok {
		return nil, false
	}
	return v.(*GuildPlayer), true
}

func (m *Manager) getOrCreate(guildID string) *GuildPlayer {
	v, _ := m.players.LoadOrStore(guildID, newGuildPlayer(guildID, m))
	return v.(*GuildPlayer)
}

func (m *Manager) lavaPlayer(guildID string) (*disgolink.Player, bool) {
	m.mu.RLock()
	link := m.link
	m.mu.RUnlock()
	if link == nil {
		return nil, false
	}
	return link.Player(snowflake.MustParse(guildID)), true
}

// Leave stops playback, destroys the Lavalink player, and disconnects from
// voice, discarding the guild's queue state.
func (m *Manager) Leave(guildID string) error {
	v, ok := m.players.LoadAndDelete(guildID)
	if !ok {
		return ErrNoVoice
	}
	gp := v.(*GuildPlayer)
	gp.mu.Lock()
	gp.current = nil
	gp.queue = nil
	gp.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m.mu.RLock()
	link := m.link
	m.mu.RUnlock()
	if link != nil {
		if p := link.ExistingPlayer(snowflake.MustParse(guildID)); p != nil {
			_ = p.Destroy(ctx)
		}
	}
	_ = m.session.ChannelVoiceJoinManual(guildID, "", false, false)
	return nil
}

// Shutdown destroys all players and disconnects every voice connection.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.RLock()
	link := m.link
	m.mu.RUnlock()
	if link == nil {
		return
	}
	m.players.Range(func(key, _ any) bool {
		guildID := key.(string)
		if p := link.ExistingPlayer(snowflake.MustParse(guildID)); p != nil {
			_ = p.Destroy(ctx)
		}
		_ = m.session.ChannelVoiceJoinManual(guildID, "", false, false)
		m.players.Delete(guildID)
		return true
	})
	link.Close()
}

// advance plays the next queued track for a guild, or clears the current track
// when the queue is empty. Called when a track ends naturally.
func (m *Manager) advance(guildID snowflake.ID) {
	v, ok := m.players.Load(guildID.String())
	if !ok {
		return
	}
	gp := v.(*GuildPlayer)

	gp.mu.Lock()
	if len(gp.queue) == 0 {
		gp.current = nil
		gp.mu.Unlock()
		return
	}
	next := gp.queue[0]
	gp.queue = gp.queue[1:]
	gp.current = &next
	gp.mu.Unlock()

	player := m.link.Player(guildID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := player.Update(ctx, disgolink.WithTrack(next.Track)); err != nil {
		log.Error().Err(err).Str("guild", guildID.String()).Msg("music: failed to advance queue")
	}
}

// --- Lavalink event listeners ---

func (m *Manager) onTrackEnd(e disgolink.PlayerTrackEndEvent) {
	if !e.Reason.MayStartNext() {
		return
	}
	m.advance(e.GuildID)
}

func (m *Manager) onTrackException(e disgolink.PlayerTrackExceptionEvent) {
	log.Error().
		Str("guild", e.GuildID.String()).
		Str("track", e.Track.Info.Title).
		Str("severity", string(e.Exception.Severity)).
		Str("cause", e.Exception.Cause).
		Msg("music: track exception: " + e.Exception.Message)
	// A failed track ends with reason loadFailed which MayStartNext, so the
	// queue advances via onTrackEnd; nothing else to do here.
}

func (m *Manager) onTrackStuck(e disgolink.PlayerTrackStuckEvent) {
	log.Warn().
		Str("guild", e.GuildID.String()).
		Str("track", e.Track.Info.Title).
		Msg("music: track stuck, skipping")
	m.advance(e.GuildID)
}

func (m *Manager) onWebSocketClosed(e disgolink.PlayerWebSocketClosedEvent) {
	// 4014: disconnected/moved by Discord. Treat as a leave so state is clean.
	if e.Code == 4014 || e.ByRemote {
		m.players.Delete(e.GuildID.String())
	}
	log.Warn().
		Str("guild", e.GuildID.String()).
		Int("code", e.Code).
		Bool("by_remote", e.ByRemote).
		Msg("music: voice websocket closed: " + e.Reason)
}

// --- Discord voice gateway forwarding ---

func (m *Manager) onVoiceStateUpdate(s *discordgo.Session, e *discordgo.VoiceStateUpdate) {
	if s.State == nil || s.State.User == nil || e.UserID != s.State.User.ID {
		return
	}
	m.mu.RLock()
	link := m.link
	m.mu.RUnlock()
	if link == nil {
		return
	}
	gid, err := snowflake.Parse(e.GuildID)
	if err != nil {
		return
	}
	var channelID *snowflake.ID
	if e.ChannelID != "" {
		if cid, err := snowflake.Parse(e.ChannelID); err == nil {
			channelID = &cid
		}
	}
	link.OnVoiceStateUpdate(context.Background(), gid, channelID, e.SessionID)
}

func (m *Manager) onVoiceServerUpdate(s *discordgo.Session, e *discordgo.VoiceServerUpdate) {
	m.mu.RLock()
	link := m.link
	m.mu.RUnlock()
	if link == nil {
		return
	}
	gid, err := snowflake.Parse(e.GuildID)
	if err != nil {
		return
	}
	link.OnVoiceServerUpdate(context.Background(), gid, e.Token, e.Endpoint)
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
