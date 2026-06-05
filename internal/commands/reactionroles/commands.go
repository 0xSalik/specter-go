// Package reactionroles implements the /reactionroles management commands.
package reactionroles

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/core"
	"github.com/salik/specter/internal/db"
	rrsvc "github.com/salik/specter/internal/reactionroles"
)

const group = "reactionroles"

// Register wires the reaction-role commands into the router.
func Register(r *core.Router) {
	r.Register(core.Command{
		Group: group, RequiredPerm: discordgo.PermissionManageRoles, Handler: handle,
		Def: &discordgo.ApplicationCommand{
			Name: "reactionroles", Description: "Manage reaction role menus",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "create", Description: "Create a new menu",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Target channel", Required: true},
						{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "Menu name", Required: true},
						{Type: discordgo.ApplicationCommandOptionString, Name: "description", Description: "Menu description", Required: false},
						{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Menu type", Required: false,
							Choices: []*discordgo.ApplicationCommandOptionChoice{
								{Name: "normal", Value: "normal"}, {Name: "unique", Value: "unique"},
								{Name: "verify", Value: "verify"}, {Name: "reverse", Value: "reverse"}}},
					}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "add", Description: "Add an emoji-role mapping",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "menu_id", Description: "Menu ID", Required: true},
						{Type: discordgo.ApplicationCommandOptionString, Name: "emoji", Description: "Emoji", Required: true},
						{Type: discordgo.ApplicationCommandOptionRole, Name: "role", Description: "Role to grant", Required: true},
					}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "list", Description: "List all menus"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "delete", Description: "Delete a menu",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "menu_id", Description: "Menu ID", Required: true}}},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "edit", Description: "Edit a menu's title/description",
					Options: []*discordgo.ApplicationCommandOption{
						{Type: discordgo.ApplicationCommandOptionInteger, Name: "menu_id", Description: "Menu ID", Required: true},
						{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "New name", Required: true},
						{Type: discordgo.ApplicationCommandOptionString, Name: "description", Description: "New description", Required: false}}},
			},
		},
	})
}

func handle(c *core.Context) {
	switch c.SubCommand {
	case "create":
		create(c)
	case "add":
		addEntry(c)
	case "list":
		list(c)
	case "delete":
		del(c)
	case "edit":
		edit(c)
	default:
		_ = c.Errorf("Unknown subcommand.", nil)
	}
}

func create(c *core.Context) {
	ch := c.ChannelOpt("channel")
	name := c.StringOpt("name", "")
	desc := c.StringOpt("description", "")
	typ := c.StringOpt("type", "normal")
	if ch == nil || name == "" {
		_ = c.Errorf("A channel and name are required.", nil)
		return
	}
	_ = c.Defer(true)

	b := c.Embed().Title(name)
	if desc != "" {
		b.Description(desc)
	} else {
		b.Description("React below to assign yourself a role.")
	}
	msg, err := c.Session.ChannelMessageSendEmbed(ch.ID, b.Build())
	if err != nil {
		_ = c.Errorf("Failed to post the menu message. Check the bot's permissions in that channel.", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}
	id, err := c.Store.CreateMenu(ctx, c.GuildID, ch.ID, msg.ID, name, descPtr, typ)
	if err != nil {
		_ = c.Errorf("Failed to record the menu.", err)
		return
	}
	_ = c.Success("Menu Created", fmt.Sprintf("Created menu **%s** (ID `%d`) in <#%s>. Use `/reactionroles add menu_id:%d` to add roles.", name, id, ch.ID, id))
}

func addEntry(c *core.Context) {
	menuID := c.IntOpt("menu_id", 0)
	rawEmoji := c.StringOpt("emoji", "")
	role := c.RoleOpt("role")
	if menuID == 0 || rawEmoji == "" || role == nil {
		_ = c.Errorf("Menu ID, emoji and role are required.", nil)
		return
	}
	_ = c.Defer(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	menu, err := c.Store.GetMenu(ctx, c.GuildID, menuID)
	if err != nil {
		if db.IsNotFound(err) {
			_ = c.Errorf(fmt.Sprintf("No menu with ID `%d` exists.", menuID), nil)
			return
		}
		_ = c.Errorf("Failed to load the menu.", err)
		return
	}

	emoji := rrsvc.NormalizeEmoji(rawEmoji)
	if _, err := c.Store.AddEntry(ctx, menuID, emoji, role.ID); err != nil {
		_ = c.Errorf("Failed to add the entry.", err)
		return
	}
	if err := c.Session.MessageReactionAdd(menu.ChannelID, menu.MessageID, emoji); err != nil {
		_ = c.Errorf("Entry saved, but I could not add the reaction. Check that the emoji is valid and accessible.", err)
		return
	}
	_ = c.Success("Entry Added", fmt.Sprintf("Reacting with %s now grants <@&%s>.", rawEmoji, role.ID))
}

func list(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	menus, err := c.Store.ListMenus(ctx, c.GuildID)
	if err != nil {
		_ = c.Errorf("Failed to load menus.", err)
		return
	}
	b := c.Embed().Title("Reaction Role Menus").Timestamp()
	if len(menus) == 0 {
		b.Description("No reaction role menus have been created.")
	}
	for _, m := range menus {
		count, _ := c.Store.CountEntries(ctx, m.ID)
		b.Field(fmt.Sprintf("#%d • %s", m.ID, m.Name),
			fmt.Sprintf("Channel <#%s> • Type `%s` • %d entries", m.ChannelID, m.Type, count), false)
	}
	_ = c.Reply(b.Build())
}

func del(c *core.Context) {
	menuID := c.IntOpt("menu_id", 0)
	_ = c.Defer(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	menu, err := c.Store.GetMenu(ctx, c.GuildID, menuID)
	if err == nil {
		_ = c.Session.ChannelMessageDelete(menu.ChannelID, menu.MessageID)
	}
	ok, err := c.Store.DeleteMenu(ctx, c.GuildID, menuID)
	if err != nil {
		_ = c.Errorf("Failed to delete the menu.", err)
		return
	}
	if !ok {
		_ = c.Errorf(fmt.Sprintf("No menu with ID `%d` exists.", menuID), nil)
		return
	}
	_ = c.Success("Menu Deleted", fmt.Sprintf("Menu `%d` and its entries were removed.", menuID))
}

func edit(c *core.Context) {
	menuID := c.IntOpt("menu_id", 0)
	name := c.StringOpt("name", "")
	desc := c.StringOpt("description", "")
	_ = c.Defer(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}
	ok, err := c.Store.UpdateMenu(ctx, c.GuildID, menuID, name, descPtr)
	if err != nil {
		_ = c.Errorf("Failed to update the menu.", err)
		return
	}
	if !ok {
		_ = c.Errorf(fmt.Sprintf("No menu with ID `%d` exists.", menuID), nil)
		return
	}

	// Update the live message embed too.
	if menu, err := c.Store.GetMenu(ctx, c.GuildID, menuID); err == nil {
		b := c.Embed().Title(name)
		if desc != "" {
			b.Description(desc)
		}
		_, _ = c.Session.ChannelMessageEditEmbed(menu.ChannelID, menu.MessageID, b.Build())
	}
	_ = c.Success("Menu Updated", fmt.Sprintf("Menu `%d` has been updated.", menuID))
}
