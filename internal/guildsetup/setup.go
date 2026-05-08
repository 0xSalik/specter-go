// Package guildsetup provisions the per-guild logging infrastructure (a private
// category and the six log channels) and is shared by the guild_create event
// and the /setup command.
package guildsetup

import (
	"context"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/db/queries"
)

// LogChannelNames lists the channels created under the Specter Logs category,
// in display order.
var LogChannelNames = []string{"general-log", "user-log", "message-log", "warn-log", "kick-log", "ban-log"}

// Result reports the outcome of provisioning, including any channels that could
// not be created (e.g. due to missing permissions).
type Result struct {
	Config  *queries.GuildConfig
	Failed  []string
	Created bool
}

// EnsureLogInfrastructure creates the log category and channels if they have
// not been created yet, persisting their IDs. It is safe to call repeatedly: if
// the channels already exist it returns the existing configuration.
func EnsureLogInfrastructure(ctx context.Context, s *discordgo.Session, store *queries.Store, guildID string) (*Result, error) {
	if _, err := store.EnsureGuild(ctx, guildID); err != nil {
		return nil, err
	}
	cfg, err := store.GetGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	if cfg.GeneralLogID != nil && *cfg.GeneralLogID != "" {
		return &Result{Config: cfg, Created: false}, nil
	}

	res := &Result{Created: true}

	// Private category: deny @everyone View; administrators bypass overwrites.
	category, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name: "Specter Logs",
		Type: discordgo.ChannelTypeGuildCategory,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{ID: guildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel},
		},
	})
	var categoryID *string
	if err != nil {
		res.Failed = append(res.Failed, "Specter Logs (category)")
	} else {
		categoryID = &category.ID
	}

	ids := make(map[string]*string, len(LogChannelNames))
	for _, name := range LogChannelNames {
		data := discordgo.GuildChannelCreateData{Name: name, Type: discordgo.ChannelTypeGuildText}
		if categoryID != nil {
			data.ParentID = *categoryID
		}
		ch, err := s.GuildChannelCreateComplex(guildID, data)
		if err != nil {
			res.Failed = append(res.Failed, name)
			continue
		}
		id := ch.ID
		ids[name] = &id
	}

	if err := store.SetLogChannels(ctx, guildID, categoryID,
		ids["general-log"], ids["user-log"], ids["message-log"],
		ids["warn-log"], ids["kick-log"], ids["ban-log"]); err != nil {
		return res, err
	}

	cfg, err = store.GetGuild(ctx, guildID)
	if err != nil {
		return res, err
	}
	res.Config = cfg
	return res, nil
}
