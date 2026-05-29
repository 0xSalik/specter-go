// Package music implements the per-guild audio player: a thread-safe queue, a
// yt-dlp resolver, an ffmpeg+dca encoding pipeline, and a player state machine.
package music

import "sync"

// Track is a single queued item.
type Track struct {
	Title     string
	URL       string // resolvable input (page URL or search term result)
	Requester string // user ID
	Duration  int    // seconds, 0 if unknown
}

// Queue is a FIFO queue safe for concurrent use.
type Queue struct {
	mu    sync.Mutex
	items []Track
}

// NewQueue constructs an empty queue.
func NewQueue() *Queue { return &Queue{} }

// Enqueue appends a track.
func (q *Queue) Enqueue(t Track) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, t)
}

// Dequeue removes and returns the front track. ok is false when empty.
func (q *Queue) Dequeue() (Track, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return Track{}, false
	}
	t := q.items[0]
	q.items = q.items[1:]
	return t, true
}

// Peek returns the front track without removing it.
func (q *Queue) Peek() (Track, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return Track{}, false
	}
	return q.items[0], true
}

// Len returns the number of queued tracks.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// List returns a copy of the queued tracks.
func (q *Queue) List() []Track {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Track, len(q.items))
	copy(out, q.items)
	return out
}

// Clear empties the queue atomically.
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = nil
}
