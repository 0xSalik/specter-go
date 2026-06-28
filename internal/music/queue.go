// Package music drives audio playback through a Lavalink node via the disgolink
// client. Specter does not decode or stream audio itself: the Lavalink server
// owns the Discord voice connection (including the DAVE E2EE handshake) and the
// source plugins (youtube-source, LavaSrc for Spotify, built-in SoundCloud).
// This package only forwards Discord voice gateway events to Lavalink, resolves
// queries, and maintains the per-guild queue and now-playing metadata.
package music

import (
	"time"

	"github.com/disgoorg/disgolink/v4/lavalink"
)

// Track is a single queued item: a resolved Lavalink track plus the ID of the
// user who requested it.
type Track struct {
	lavalink.Track
	Requester string
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

// Duration returns the track length. Streams report zero.
func (t Track) Duration() time.Duration {
	if t.Info.IsStream {
		return 0
	}
	return time.Duration(t.Info.Length) * time.Millisecond
}

// IsStream reports whether the track is a live stream of unknown length.
func (t Track) IsStream() bool { return t.Info.IsStream }
