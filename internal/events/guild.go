package events

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/salik/specter/internal/embed"
	"github.com/salik/specter/internal/guildsetup"
)

func (h *Handlers) onGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	isNew, err := h.deps.Store.EnsureGuild(ctx, g.ID)
	if err != nil {
		log.Error().Err(err).Str("guild", g.ID).Msg("guild_create: ensure guild")
		return
	}
	if !isNew {
		return // reconnect / already known
	}

	log.Info().Str("guild", g.ID).Str("name", g.Name).Msg("joined new guild")

	res, err := guildsetup.EnsureLogInfrastructure(ctx, s, h.deps.Store, g.ID)
	if err != nil {
		log.Error().Err(err).Str("guild", g.ID).Msg("guild_create: provision logs")
		return
	}

	if res.Config == nil || res.Config.GeneralLogID == nil || *res.Config.GeneralLogID == "" {
		return
	}

	desc := strings.Join([]string{
		"Specter is a professional administration and utility bot.",
		"",
		"Run `/setup` to configure leveling, automod, join-to-create voice, and the embed color.",
		"All moderation and event log channels have been created under the **Specter Logs** category.",
		"Use `/help` to explore available commands.",
	}, "\n")

	b := embed.New(s, g.ID).Title("Specter is online.").Description(desc).
		Field("Server", g.Name, true).
		Field("Members", fmt.Sprintf("%d", g.MemberCount), true).
		Field("Date", time.Now().Format("2006-01-02"), true).
		Timestamp()
	if len(res.Failed) > 0 {
		b.Field("Channels not created", fmt.Sprintf("%v (check bot permissions)", res.Failed), false)
	}
	if _, err := s.ChannelMessageSendEmbed(*res.Config.GeneralLogID, b.Build()); err != nil {
		log.Warn().Err(err).Msg("guild_create: welcome message")
	}
}

func (h *Handlers) onGuildDelete(s *discordgo.Session, g *discordgo.GuildDelete) {
	if g.Unavailable {
		return // outage, not a removal
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.deps.Store.DeleteGuild(ctx, g.ID); err != nil {
		log.Error().Err(err).Str("guild", g.ID).Msg("guild_delete: cleanup")
	}
}
