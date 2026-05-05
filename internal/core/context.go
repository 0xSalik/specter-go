package core

import (
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/salik/specter/internal/embed"
)

// Context carries everything a handler needs for a single interaction.
type Context struct {
	*Deps
	Interaction *discordgo.InteractionCreate
	GuildID     string
	UserID      string

	// options is the flattened option slice for the invoked (sub)command.
	options    []*discordgo.ApplicationCommandInteractionDataOption
	SubCommand string
	deferred   bool
}

// newContext builds a Context, resolving subcommand options into a flat slice.
func newContext(d *Deps, i *discordgo.InteractionCreate) *Context {
	c := &Context{Deps: d, Interaction: i, GuildID: i.GuildID}
	if i.Member != nil && i.Member.User != nil {
		c.UserID = i.Member.User.ID
	} else if i.User != nil {
		c.UserID = i.User.ID
	}

	data := i.ApplicationCommandData()
	c.options = data.Options
	if len(data.Options) > 0 {
		switch data.Options[0].Type {
		case discordgo.ApplicationCommandOptionSubCommand:
			c.SubCommand = data.Options[0].Name
			c.options = data.Options[0].Options
		case discordgo.ApplicationCommandOptionSubCommandGroup:
			grp := data.Options[0]
			if len(grp.Options) > 0 {
				c.SubCommand = grp.Name + " " + grp.Options[0].Name
				c.options = grp.Options[0].Options
			}
		}
	}
	return c
}

// Embed returns a fresh builder pre-set to the guild's accent color.
func (c *Context) Embed() *embed.EmbedBuilder {
	return embed.New(c.Session, c.GuildID)
}

// option returns a raw option by name from the flattened slice.
func (c *Context) option(name string) *discordgo.ApplicationCommandInteractionDataOption {
	for _, o := range c.options {
		if o.Name == name {
			return o
		}
	}
	return nil
}

// StringOpt returns a string option or the provided default.
func (c *Context) StringOpt(name, def string) string {
	if o := c.option(name); o != nil {
		return o.StringValue()
	}
	return def
}

// IntOpt returns an integer option or the provided default.
func (c *Context) IntOpt(name string, def int) int {
	if o := c.option(name); o != nil {
		return int(o.IntValue())
	}
	return def
}

// BoolOpt returns a bool option or the provided default.
func (c *Context) BoolOpt(name string, def bool) bool {
	if o := c.option(name); o != nil {
		return o.BoolValue()
	}
	return def
}

// UserOpt returns a *discordgo.User option, or nil if absent.
func (c *Context) UserOpt(name string) *discordgo.User {
	o := c.option(name)
	if o == nil {
		return nil
	}
	return o.UserValue(c.Session)
}

// ChannelOpt returns a *discordgo.Channel option, or nil if absent.
func (c *Context) ChannelOpt(name string) *discordgo.Channel {
	o := c.option(name)
	if o == nil {
		return nil
	}
	return o.ChannelValue(c.Session)
}

// RoleOpt returns a *discordgo.Role option, or nil if absent.
func (c *Context) RoleOpt(name string) *discordgo.Role {
	o := c.option(name)
	if o == nil {
		return nil
	}
	return o.RoleValue(c.Session, c.GuildID)
}

// HasOpt reports whether an option was supplied.
func (c *Context) HasOpt(name string) bool { return c.option(name) != nil }

// Defer acknowledges the interaction so the handler can take longer than 3s.
// Pass ephemeral=true to make the eventual response visible only to the caller.
func (c *Context) Defer(ephemeral bool) error {
	resp := &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredChannelMessageWithSource}
	if ephemeral {
		resp.Data = &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral}
	}
	err := c.Session.InteractionRespond(c.Interaction.Interaction, resp)
	if err == nil {
		c.deferred = true
	}
	return err
}

// Reply sends an embed, choosing the correct mechanism based on whether the
// interaction was deferred.
func (c *Context) Reply(e *discordgo.MessageEmbed) error {
	return c.reply([]*discordgo.MessageEmbed{e}, nil, false)
}

// ReplyEphemeral sends an ephemeral embed (only when not deferred-ephemeral).
func (c *Context) ReplyEphemeral(e *discordgo.MessageEmbed) error {
	return c.reply([]*discordgo.MessageEmbed{e}, nil, true)
}

// ReplyComponents sends an embed alongside message components (e.g. buttons).
func (c *Context) ReplyComponents(e *discordgo.MessageEmbed, components []discordgo.MessageComponent) error {
	return c.reply([]*discordgo.MessageEmbed{e}, components, false)
}

func (c *Context) reply(embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent, ephemeral bool) error {
	if c.deferred {
		edit := &discordgo.WebhookEdit{Embeds: &embeds}
		if components != nil {
			edit.Components = &components
		}
		_, err := c.Session.InteractionResponseEdit(c.Interaction.Interaction, edit)
		return err
	}
	data := &discordgo.InteractionResponseData{Embeds: embeds, Components: components}
	if ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}
	return c.Session.InteractionRespond(c.Interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
}

// ReplyFile sends an embed with a file attachment (e.g. rank cards, downloads).
func (c *Context) ReplyFile(e *discordgo.MessageEmbed, name string, data []byte) error {
	files := []*discordgo.File{{Name: name, Reader: bytesReader(data)}}
	embeds := []*discordgo.MessageEmbed{e}
	if c.deferred {
		_, err := c.Session.InteractionResponseEdit(c.Interaction.Interaction, &discordgo.WebhookEdit{
			Embeds: &embeds,
			Files:  files,
		})
		return err
	}
	return c.Session.InteractionRespond(c.Interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Embeds: embeds, Files: files},
	})
}

// Success replies with a green success embed.
func (c *Context) Success(title, desc string) error {
	return c.Reply(c.Embed().Title(title).Description(desc).AsSuccess().Timestamp().Build())
}

// Errorf replies with a red error embed and logs the underlying cause.
func (c *Context) Errorf(userMsg string, cause error) error {
	if cause != nil {
		log.Error().Err(cause).Str("guild", c.GuildID).Str("user", c.UserID).
			Str("command", c.Interaction.ApplicationCommandData().Name).Msg(userMsg)
	}
	e := c.Embed().Title("Error").Description(userMsg).AsError().Build()
	if c.deferred {
		return c.Reply(e)
	}
	return c.ReplyEphemeral(e)
}
