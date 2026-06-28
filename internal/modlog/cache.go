package modlog

import (
	"sync"

	"github.com/bwmarrin/discordgo"
)

// maxPerChannel bounds the message cache size per channel to limit memory.
const maxPerChannel = 1000

// CachedAttachment records the details of a message attachment so it can still
// be reported after the original message (and its live CDN link) is gone.
type CachedAttachment struct {
	Filename string
	URL      string
	ProxyURL string
	Size     int
}

// CachedMessage stores the content needed to enrich delete/edit logs, including
// attachments and embed count so embed-only / attachment-only messages are not
// reported as empty.
type CachedMessage struct {
	ID          string
	ChannelID   string
	GuildID     string
	AuthorID    string
	Author      string
	Content     string
	Attachments []CachedAttachment
	EmbedCount  int
}

// MessageCache is a bounded, concurrency-safe per-channel ring of recent
// messages used to recover content on delete/edit events. Eviction is FIFO.
type MessageCache struct {
	mu       sync.Mutex
	byID     map[string]*CachedMessage
	order    map[string][]string // channelID -> message IDs in insertion order
}

// NewMessageCache constructs an empty cache.
func NewMessageCache() *MessageCache {
	return &MessageCache{
		byID:  make(map[string]*CachedMessage),
		order: make(map[string][]string),
	}
}

// Put records a message, evicting the oldest in the channel past the cap.
func (c *MessageCache) Put(m *discordgo.Message) {
	if m == nil || m.ID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	author, authorID := "", ""
	if m.Author != nil {
		author = m.Author.Username
		authorID = m.Author.ID
	}

	var attachments []CachedAttachment
	for _, a := range m.Attachments {
		if a == nil {
			continue
		}
		attachments = append(attachments, CachedAttachment{
			Filename: a.Filename,
			URL:      a.URL,
			ProxyURL: a.ProxyURL,
			Size:     a.Size,
		})
	}

	c.byID[m.ID] = &CachedMessage{
		ID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID,
		AuthorID: authorID, Author: author, Content: m.Content,
		Attachments: attachments, EmbedCount: len(m.Embeds),
	}
	order := append(c.order[m.ChannelID], m.ID)
	if len(order) > maxPerChannel {
		evict := order[0]
		order = order[1:]
		delete(c.byID, evict)
	}
	c.order[m.ChannelID] = order
}

// Get returns a cached message by ID, if present.
func (c *MessageCache) Get(id string) (*CachedMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.byID[id]
	return m, ok
}
