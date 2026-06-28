package music_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/internal/music"
)

func TestQueueFIFO(t *testing.T) {
	q := music.NewQueue()
	for i := 0; i < 5; i++ {
		q.Enqueue(music.Track{Title: string(rune('a' + i))})
	}
	require.Equal(t, 5, q.Len())
	for i := 0; i < 5; i++ {
		tr, ok := q.Dequeue()
		require.True(t, ok)
		assert.Equal(t, string(rune('a'+i)), tr.Title)
	}
	_, ok := q.Dequeue()
	assert.False(t, ok)
}

func TestQueuePeek(t *testing.T) {
	q := music.NewQueue()
	q.Enqueue(music.Track{Title: "first"})
	q.Enqueue(music.Track{Title: "second"})
	tr, ok := q.Peek()
	require.True(t, ok)
	assert.Equal(t, "first", tr.Title)
	assert.Equal(t, 2, q.Len(), "peek must not remove")
}

func TestQueueClear(t *testing.T) {
	q := music.NewQueue()
	q.Enqueue(music.Track{Title: "x"})
	q.Clear()
	assert.Equal(t, 0, q.Len())
}

func TestQueueConcurrentEnqueue(t *testing.T) {
	q := music.NewQueue()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				q.Enqueue(music.Track{Title: "t"})
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1000, q.Len())
}
