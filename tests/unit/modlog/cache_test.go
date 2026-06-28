package modlog_test

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/internal/modlog"
)

func TestCacheCapturesAttachmentsAndEmbeds(t *testing.T) {
	c := modlog.NewMessageCache()
	c.Put(&discordgo.Message{
		ID:        "msg1",
		ChannelID: "chan1",
		GuildID:   "g1",
		Author:    &discordgo.User{ID: "u1", Username: "alice"},
		Content:   "", // attachment/embed-only message
		Attachments: []*discordgo.MessageAttachment{
			{Filename: "cat.png", URL: "https://cdn/cat.png", ProxyURL: "https://proxy/cat.png", Size: 2048},
		},
		Embeds: []*discordgo.MessageEmbed{{Title: "preview"}},
	})

	got, ok := c.Get("msg1")
	require.True(t, ok, "message must be cached even with empty content")
	assert.Equal(t, "u1", got.AuthorID)
	assert.Empty(t, got.Content)
	require.Len(t, got.Attachments, 1)
	assert.Equal(t, "cat.png", got.Attachments[0].Filename)
	assert.Equal(t, "https://cdn/cat.png", got.Attachments[0].URL)
	assert.Equal(t, 2048, got.Attachments[0].Size)
	assert.Equal(t, 1, got.EmbedCount)
}

func TestCacheTextOnlyMessage(t *testing.T) {
	c := modlog.NewMessageCache()
	c.Put(&discordgo.Message{
		ID:      "msg2",
		Author:  &discordgo.User{ID: "u2"},
		Content: "hello",
	})
	got, ok := c.Get("msg2")
	require.True(t, ok)
	assert.Equal(t, "hello", got.Content)
	assert.Empty(t, got.Attachments)
	assert.Equal(t, 0, got.EmbedCount)
}

func TestCacheMissReturnsFalse(t *testing.T) {
	c := modlog.NewMessageCache()
	_, ok := c.Get("nope")
	assert.False(t, ok)
}
