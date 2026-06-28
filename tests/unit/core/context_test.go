package core_test

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/internal/core"
)

// TestNewContextComponentDoesNotPanic guards the regression where newContext
// called ApplicationCommandData() on a component interaction and panicked,
// crashing the bot when a button/select menu was clicked.
func TestNewContextComponentDoesNotPanic(t *testing.T) {
	i := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data:    discordgo.MessageComponentInteractionData{CustomID: "automod:toggle:spam"},
	}}

	require.NotPanics(t, func() {
		c := core.NewContextForTest(&core.Deps{}, i)
		assert.Equal(t, "g1", c.GuildID)
		assert.Equal(t, "u1", c.UserID)
		assert.Empty(t, c.SubCommand)
		assert.Empty(t, c.Options())
	})
}

func TestNewContextParsesSubcommand(t *testing.T) {
	i := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "automod",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "config"},
			},
		},
	}}

	c := core.NewContextForTest(&core.Deps{}, i)
	assert.Equal(t, "config", c.SubCommand)
	assert.Equal(t, "u1", c.UserID)
}

func TestNewContextParsesSubcommandGroup(t *testing.T) {
	i := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		User:    &discordgo.User{ID: "u2"},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "reactionroles",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{
					Type: discordgo.ApplicationCommandOptionSubCommandGroup,
					Name: "menu",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "create"},
					},
				},
			},
		},
	}}

	c := core.NewContextForTest(&core.Deps{}, i)
	assert.Equal(t, "menu create", c.SubCommand)
	assert.Equal(t, "u2", c.UserID)
}
