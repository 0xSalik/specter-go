// Package embed provides the single, fluent embed builder used for every bot
// response. It applies the per-guild accent color (cached) and offers
// error/success variants with fixed colors.
package embed

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/db/queries"
)

const (
	// DefaultColor is Discord blurple, used when a guild has no configured color.
	DefaultColor = 0x5865F2
	// ErrorColor is the fixed red used by AsError.
	ErrorColor = 0xED4245
	// SuccessColor is the fixed green used by AsSuccess.
	SuccessColor = 0x57F287
)

// colorProvider resolves and caches per-guild accent colors. It is populated at
// startup via Init so the builder never needs a store handle passed explicitly.
type colorProvider struct {
	store *queries.Store
	mu    sync.RWMutex
	cache map[string]int
}

var provider = &colorProvider{cache: make(map[string]int)}

// Init wires the embed package to the database. Call once at startup before
// building any embeds that depend on a guild color.
func Init(store *queries.Store) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.store = store
}

// Invalidate clears a guild's cached color so the next build re-reads the DB.
// Call after updating the embed color via /setup color or the dashboard.
func Invalidate(guildID string) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	delete(provider.cache, guildID)
}

// guildColor returns the cached/looked-up accent color for a guild as an int.
func guildColor(guildID string) int {
	if guildID == "" {
		return DefaultColor
	}
	provider.mu.RLock()
	if c, ok := provider.cache[guildID]; ok {
		provider.mu.RUnlock()
		return c
	}
	store := provider.store
	provider.mu.RUnlock()

	if store == nil {
		return DefaultColor
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	g, err := store.GetGuild(ctx, guildID)
	color := DefaultColor
	if err == nil {
		color = ParseHexColor(g.EmbedColor)
	}

	provider.mu.Lock()
	provider.cache[guildID] = color
	provider.mu.Unlock()
	return color
}

// ParseHexColor converts a "#RRGGBB" string to an int, falling back to the
// default color on any parse error.
func ParseHexColor(hex string) int {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return DefaultColor
	}
	v, err := strconv.ParseInt(hex, 16, 32)
	if err != nil {
		return DefaultColor
	}
	return int(v)
}

// ValidHexColor reports whether s is a well-formed "#RRGGBB" string.
func ValidHexColor(s string) bool {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return false
	}
	_, err := strconv.ParseInt(s, 16, 32)
	return err == nil
}

// EmbedBuilder is a fluent builder around discordgo.MessageEmbed.
type EmbedBuilder struct {
	embed *discordgo.MessageEmbed
}

// New creates a builder pre-set to the guild's accent color. The session
// argument is accepted for API symmetry and future use (e.g. footer icons).
func New(_ *discordgo.Session, guildID string) *EmbedBuilder {
	return &EmbedBuilder{embed: &discordgo.MessageEmbed{Color: guildColor(guildID)}}
}

// Title sets the embed title.
func (e *EmbedBuilder) Title(t string) *EmbedBuilder { e.embed.Title = t; return e }

// Description sets the embed description.
func (e *EmbedBuilder) Description(d string) *EmbedBuilder { e.embed.Description = d; return e }

// Field appends a field.
func (e *EmbedBuilder) Field(name, value string, inline bool) *EmbedBuilder {
	if value == "" {
		value = "\u200b"
	}
	e.embed.Fields = append(e.embed.Fields, &discordgo.MessageEmbedField{
		Name: name, Value: value, Inline: inline,
	})
	return e
}

// Footer sets the footer text.
func (e *EmbedBuilder) Footer(text string) *EmbedBuilder {
	e.embed.Footer = &discordgo.MessageEmbedFooter{Text: text}
	return e
}

// Thumbnail sets the thumbnail image URL.
func (e *EmbedBuilder) Thumbnail(url string) *EmbedBuilder {
	if url != "" {
		e.embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: url}
	}
	return e
}

// Image sets the main image URL.
func (e *EmbedBuilder) Image(url string) *EmbedBuilder {
	if url != "" {
		e.embed.Image = &discordgo.MessageEmbedImage{URL: url}
	}
	return e
}

// Color overrides the accent color with an explicit int value.
func (e *EmbedBuilder) Color(c int) *EmbedBuilder { e.embed.Color = c; return e }

// Timestamp sets the embed timestamp to now (RFC3339).
func (e *EmbedBuilder) Timestamp() *EmbedBuilder {
	e.embed.Timestamp = time.Now().Format(time.RFC3339)
	return e
}

// AsError sets the fixed error (red) color.
func (e *EmbedBuilder) AsError() *EmbedBuilder { e.embed.Color = ErrorColor; return e }

// AsSuccess sets the fixed success (green) color.
func (e *EmbedBuilder) AsSuccess() *EmbedBuilder { e.embed.Color = SuccessColor; return e }

// Build returns the finished embed.
func (e *EmbedBuilder) Build() *discordgo.MessageEmbed { return e.embed }
