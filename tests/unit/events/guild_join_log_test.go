package events_test

import (
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/internal/events"
)

func fieldsByName(em *discordgo.MessageEmbed) map[string]string {
	out := make(map[string]string, len(em.Fields))
	for _, f := range em.Fields {
		out[f.Name] = f.Value
	}
	return out
}

func TestBuildGuildJoinEmbed_FullDetail(t *testing.T) {
	gc := &discordgo.GuildCreate{Guild: &discordgo.Guild{
		ID:                       "1151004995332866128",
		Name:                     "Cool Server",
		OwnerID:                  "801803471619620916",
		MemberCount:              42,
		Icon:                     "abc123",
		Description:              "A friendly place",
		VerificationLevel:        discordgo.VerificationLevelHigh,
		PremiumTier:              2,
		PremiumSubscriptionCount: 7,
		JoinedAt:                 time.Now(),
		Channels:                 []*discordgo.Channel{{ID: "1"}, {ID: "2"}, {ID: "3"}},
		Roles:                    []*discordgo.Role{{ID: "r1"}, {ID: "r2"}},
	}}

	em := events.BuildGuildJoinEmbedForTest(gc, "owner#0001 (`801803471619620916`)", 5)
	require.NotNil(t, em)

	assert.Equal(t, "Joined a new server", em.Title)
	assert.Equal(t, 0x57F287, em.Color, "join log uses the success color")
	require.NotNil(t, em.Thumbnail)
	assert.Contains(t, em.Thumbnail.URL, "abc123")

	f := fieldsByName(em)
	assert.Contains(t, f["Server"], "Cool Server")
	assert.Contains(t, f["Server"], "1151004995332866128")
	assert.Contains(t, f["Owner"], "801803471619620916")
	assert.Equal(t, "42", f["Members"])
	assert.Equal(t, "3", f["Channels"])
	assert.Equal(t, "2", f["Roles"])
	assert.Equal(t, "5 servers", f["Now Serving"])
	assert.Equal(t, "A friendly place", f["Description"])
	assert.Contains(t, f["Details"], "High")
	assert.Contains(t, f["Details"], "Boost tier: 2")
	assert.Contains(t, f["Details"], "Boosts: 7")
	// Server creation derived from the snowflake must be present.
	assert.NotEmpty(t, f["Server Created"])
}

func TestBuildGuildJoinEmbed_MinimalGuild(t *testing.T) {
	gc := &discordgo.GuildCreate{Guild: &discordgo.Guild{
		ID:          "1151004995332866128",
		Name:        "Bare",
		OwnerID:     "1",
		MemberCount: 1,
	}}

	em := events.BuildGuildJoinEmbedForTest(gc, "<@1>", 1)
	require.NotNil(t, em)
	assert.Nil(t, em.Thumbnail, "no icon means no thumbnail")
	f := fieldsByName(em)
	_, hasDesc := f["Description"]
	assert.False(t, hasDesc, "no description field when guild has none")
	assert.Equal(t, "1 servers", f["Now Serving"])
}
