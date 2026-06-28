package music_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/disgoorg/disgolink/v4/lavalink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/internal/music"
)

func strPtr(s string) *string { return &s }

func TestTrackHelpers(t *testing.T) {
	tr := music.Track{
		Track: lavalink.Track{Info: lavalink.TrackInfo{
			Title:  "Never Gonna Give You Up",
			Author: "Rick Astley",
			Length: lavalink.Duration((3*60 + 33) * 1000), // 3:33 in ms
			URI:    strPtr("https://youtu.be/dQw4w9WgXcQ"),
		}},
		Requester: "42",
	}
	assert.Equal(t, "Never Gonna Give You Up", tr.Title())
	assert.Equal(t, "Rick Astley", tr.Author())
	assert.Equal(t, "https://youtu.be/dQw4w9WgXcQ", tr.URL())
	assert.Equal(t, (3*60+33)*time.Second, tr.Duration())
	assert.False(t, tr.IsStream())
}

func TestTrackStreamHasNoDuration(t *testing.T) {
	tr := music.Track{Track: lavalink.Track{Info: lavalink.TrackInfo{
		Title:    "Lofi Radio",
		IsStream: true,
		Length:   lavalink.Duration(9999999),
	}}}
	assert.True(t, tr.IsStream())
	assert.Equal(t, time.Duration(0), tr.Duration())
}

// TestManagerNotReady verifies that the manager fails gracefully (rather than
// panicking) when the Lavalink node has not been connected.
func TestManagerNotReady(t *testing.T) {
	session, err := discordgo.New("Bot test-token")
	require.NoError(t, err)

	m := music.NewManager(session, music.NodeConfig{})
	assert.False(t, m.Ready())

	_, ok := m.Get("123")
	assert.False(t, ok)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, loadErr := m.Load(ctx, "test query")
	assert.ErrorIs(t, loadErr, music.ErrNotReady)

	_, _, playErr := m.Play(ctx, "123", "456", "789", nil)
	assert.ErrorIs(t, playErr, music.ErrNotReady)
}

// TestLavalinkLoadTracks talks to a real Lavalink node when LAVALINK_TEST_ADDRESS
// is set (e.g. "localhost:2333"), confirming source plugins resolve queries.
func TestLavalinkLoadTracks(t *testing.T) {
	addr := os.Getenv("LAVALINK_TEST_ADDRESS")
	if addr == "" {
		t.Skip("set LAVALINK_TEST_ADDRESS (and optionally LAVALINK_TEST_PASSWORD) to run the live Lavalink test")
	}
	password := os.Getenv("LAVALINK_TEST_PASSWORD")
	if password == "" {
		password = "youshallnotpass"
	}

	session, err := discordgo.New("Bot test-token")
	require.NoError(t, err)

	m := music.NewManager(session, music.NodeConfig{Address: addr, Password: password})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// A numeric user ID is enough for the node handshake; no Discord login needed.
	require.NoError(t, m.Start(ctx, "100000000000000000"))
	require.True(t, m.Ready())

	res, err := m.Load(ctx, "Rick Astley Never Gonna Give You Up")
	require.NoError(t, err)
	require.NotEmpty(t, res.Tracks, "expected at least one track from a YouTube search")
	assert.NotEmpty(t, res.Tracks[0].Info.Title)
}
