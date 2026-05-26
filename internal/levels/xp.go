// Package levels implements the XP and leveling system: pure progression math
// plus a message-driven engine that awards XP with cooldowns and exemptions.
package levels

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"

	"github.com/salik/specter/internal/db"
	"github.com/salik/specter/internal/db/queries"
	"github.com/salik/specter/internal/embed"
)

// LevelForXP returns the level for a given total XP using the progression
// curve level = floor(0.1 * sqrt(xp)).
func LevelForXP(xp int64) int {
	if xp <= 0 {
		return 0
	}
	return int(math.Floor(0.1 * math.Sqrt(float64(xp))))
}

// CalculateXPForLevel returns the minimum total XP required to reach level n.
// It is the inverse of LevelForXP: xp = (10 * level)^2 = 100 * level^2.
func CalculateXPForLevel(level int) int64 {
	if level <= 0 {
		return 0
	}
	return int64(100 * level * level)
}

// AwardXP picks a random XP amount in [min, max].
func AwardXP(r *rand.Rand, min, max int) int64 {
	if max < min {
		max = min
	}
	if max == min {
		return int64(min)
	}
	return int64(min + r.Intn(max-min+1))
}

// OnCooldown reports whether a user is still within their XP cooldown window.
func OnCooldown(last *time.Time, now time.Time, cooldownSecs int) bool {
	if last == nil {
		return false
	}
	return now.Sub(*last) < time.Duration(cooldownSecs)*time.Second
}

// IsExempt reports whether a user/channel combination is excluded from XP.
func IsExempt(userRoles []string, channelID string, noXPRoles, noXPChannels []string) bool {
	for _, c := range noXPChannels {
		if c == channelID {
			return true
		}
	}
	roleSet := make(map[string]struct{}, len(userRoles))
	for _, r := range userRoles {
		roleSet[r] = struct{}{}
	}
	for _, r := range noXPRoles {
		if _, ok := roleSet[r]; ok {
			return true
		}
	}
	return false
}

// Engine awards XP in response to messages.
type Engine struct {
	store *queries.Store
	rng   *rand.Rand
}

// NewEngine constructs the XP engine.
func NewEngine(store *queries.Store) *Engine {
	return &Engine{store: store, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// HandleMessage applies XP rules to a single message create event. It is safe
// to call from the message handler goroutine; all errors are logged.
func (e *Engine) HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot || m.GuildID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := e.store.GetLevelConfig(ctx, m.GuildID)
	if err != nil {
		log.Error().Err(err).Str("guild", m.GuildID).Msg("levels: load config")
		return
	}
	if !cfg.Enabled {
		return
	}

	var roles []string
	if m.Member != nil {
		roles = m.Member.Roles
	}
	if IsExempt(roles, m.ChannelID, cfg.NoXPRoles, cfg.NoXPChannels) {
		return
	}

	now := time.Now()
	if existing, err := e.store.GetLevel(ctx, m.GuildID, m.Author.ID); err == nil {
		if OnCooldown(existing.LastXPAt, now, cfg.XPCooldownSecs) {
			return
		}
	} else if !db.IsNotFound(err) {
		log.Error().Err(err).Msg("levels: get level")
		return
	}

	gained := AwardXP(e.rng, cfg.XPMin, cfg.XPMax)

	// Compute the prospective new total to derive the new level.
	prev, err := e.store.GetLevel(ctx, m.GuildID, m.Author.ID)
	var prevXP int64
	var prevLevel int
	if err == nil {
		prevXP = prev.XP
		prevLevel = prev.Level
	}
	newXP := prevXP + gained
	newLevel := LevelForXP(newXP)

	entry, err := e.store.AddXP(ctx, m.GuildID, m.Author.ID, gained, newLevel, now)
	if err != nil {
		log.Error().Err(err).Msg("levels: add xp")
		return
	}

	if entry.Level > prevLevel {
		e.announce(s, m, cfg, entry.Level)
	}
}

func (e *Engine) announce(s *discordgo.Session, m *discordgo.MessageCreate, cfg *queries.LevelConfig, level int) {
	channelID := m.ChannelID
	if cfg.AnnounceChannelID != nil && *cfg.AnnounceChannelID != "" {
		channelID = *cfg.AnnounceChannelID
	}

	msg := "Congratulations <@" + m.Author.ID + ">, you reached level " + itoa(level) + "."
	if cfg.AnnounceMsg != nil && *cfg.AnnounceMsg != "" {
		msg = renderAnnounce(*cfg.AnnounceMsg, m.Author.ID, level)
	}

	b := embed.New(s, m.GuildID).Title("Level Up").Description(msg)
	if _, err := s.ChannelMessageSendEmbed(channelID, b.Build()); err != nil {
		log.Warn().Err(err).Str("guild", m.GuildID).Msg("levels: announce")
	}
}
