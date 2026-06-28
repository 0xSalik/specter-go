// Package music drives audio playback through a Lavalink node via the disgolink
// client. Specter does not decode or stream audio itself: the Lavalink server
// owns the Discord voice connection (including the DAVE E2EE handshake) and the
// source plugins (youtube-source, LavaSrc for Spotify, built-in SoundCloud).
// This package only forwards Discord voice gateway events to Lavalink, resolves
// queries, and maintains the per-guild queue and now-playing metadata.
package music

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/disgoorg/disgolink/v4/lavalink"
)

// Track is a single queued item: a resolved Lavalink track plus the ID of the
// user who requested it. QID is a stable per-queue identifier used by the
// dashboard to target a specific entry for removal or reordering.
type Track struct {
	lavalink.Track
	Requester string
	QID       string
}

// newTrack wraps a resolved Lavalink track with requester metadata and a fresh
// stable queue identifier.
func newTrack(t lavalink.Track, requester string) Track {
	return Track{Track: t, Requester: requester, QID: newQID()}
}

func newQID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Title returns the track title, falling back to the identifier.
func (t Track) Title() string {
	if t.Info.Title != "" {
		return t.Info.Title
	}
	return t.Info.Identifier
}

// Author returns the track author/uploader.
func (t Track) Author() string { return t.Info.Author }

// URL returns the track's source URL if present.
func (t Track) URL() string {
	if t.Info.URI != nil {
		return *t.Info.URI
	}
	return ""
}

// Source returns the source plugin name (e.g. "youtube", "spotify").
func (t Track) Source() string { return t.Info.SourceName }

// Artwork returns the cover/thumbnail URL if the source provides one.
func (t Track) Artwork() string {
	if t.Info.ArtworkURL != nil {
		return *t.Info.ArtworkURL
	}
	return ""
}

// Duration returns the track length. Streams report zero.
func (t Track) Duration() time.Duration {
	if t.Info.IsStream {
		return 0
	}
	return time.Duration(t.Info.Length) * time.Millisecond
}

// IsStream reports whether the track is a live stream of unknown length.
func (t Track) IsStream() bool { return t.Info.IsStream }
