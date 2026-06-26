//go:build e2e

// Package harness provides end-to-end test scaffolding that connects a real
// test bot to a dedicated Discord guild. It is compiled only under the e2e
// build tag and requires the E2E_* environment variables.
package harness

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Harness wraps a live Discord session against the test guild.
type Harness struct {
	Session   *discordgo.Session
	GuildID   string
	ChannelID string
	VCID      string
	UserID    string

	mu       sync.Mutex
	messages []*discordgo.Message
}

// New connects the test bot and blocks until the gateway is READY.
func New(t *testing.T) *Harness {
	t.Helper()
	token := os.Getenv("E2E_BOT_TOKEN")
	guild := os.Getenv("E2E_TEST_GUILD_ID")
	channel := os.Getenv("E2E_TEST_CHANNEL_ID")
	if token == "" || guild == "" || channel == "" {
		t.Skip("E2E environment variables not set; skipping e2e test")
	}

	s, err := discordgo.New("Bot " + token)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	s.Identify.Intents = discordgo.IntentsAll

	h := &Harness{
		Session:   s,
		GuildID:   guild,
		ChannelID: channel,
		VCID:      os.Getenv("E2E_TEST_VC_ID"),
		UserID:    os.Getenv("E2E_TEST_USER_ID"),
	}

	ready := make(chan struct{})
	var once sync.Once
	s.AddHandler(func(_ *discordgo.Session, _ *discordgo.Ready) { once.Do(func() { close(ready) }) })
	s.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageCreate) {
		h.mu.Lock()
		h.messages = append(h.messages, m.Message)
		h.mu.Unlock()
	})

	if err := s.Open(); err != nil {
		t.Fatalf("open session: %v", err)
	}
	select {
	case <-ready:
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for READY")
	}
	return h
}

// WaitForMessage waits for a captured message matching the predicate.
func (h *Harness) WaitForMessage(predicate func(*discordgo.Message) bool, timeout time.Duration) (*discordgo.Message, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		for _, m := range h.messages {
			if predicate(m) {
				h.mu.Unlock()
				return m, nil
			}
		}
		h.mu.Unlock()
		time.Sleep(200 * time.Millisecond)
	}
	return nil, errors.New("timed out waiting for matching message")
}

// Close disconnects the session.
func (h *Harness) Close() {
	_ = h.Session.Close()
}

// Cleanup resets transient test state. Extend as needed per test suite.
func (h *Harness) Cleanup() {
	h.mu.Lock()
	h.messages = nil
	h.mu.Unlock()
}
