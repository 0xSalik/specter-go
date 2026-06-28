package voice_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/0xSalik/specter/internal/voice"
)

func TestRenderNameUsername(t *testing.T) {
	assert.Equal(t, "Alice's Channel", voice.RenderName("{username}'s Channel", "Alice"))
}

func TestRenderNameFallback(t *testing.T) {
	// An empty template falls back to "<user>'s Channel".
	assert.Equal(t, "Bob's Channel", voice.RenderName("", "Bob"))
}

func TestRenderNameTruncates(t *testing.T) {
	long := strings.Repeat("x", 200)
	out := voice.RenderName("{username}", long)
	assert.LessOrEqual(t, len(out), 100)
}
