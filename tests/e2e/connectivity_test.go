//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/tests/e2e/harness"
)

// TestConnectivity verifies the test bot can connect and observe its own
// messages in the test channel — the foundation every other e2e test builds on.
func TestConnectivity(t *testing.T) {
	h := harness.New(t)
	defer h.Close()
	defer h.Cleanup()

	marker := "specter-e2e-" + time.Now().Format("150405.000")
	_, err := h.Session.ChannelMessageSend(h.ChannelID, marker)
	require.NoError(t, err)

	msg, err := h.WaitForMessage(func(m *discordgo.Message) bool {
		return m.Content == marker
	}, 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, marker, msg.Content)
}
