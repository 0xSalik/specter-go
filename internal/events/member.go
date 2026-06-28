package events

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/modlog"
)

func (h *Handlers) onGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.User == nil {
		return
	}

	created, _ := discordgo.SnowflakeTimestamp(m.User.ID)
	extra := map[string]string{
		"User":            fmt.Sprintf("%s (`%s`)", userTag(m.User), m.User.ID),
		"Account Created": timestampFull(created),
		"Account Age":     humanizeDuration(time.Since(created)),
	}
	if m.User.Bot {
		extra["Type"] = "Bot"
	}
	if age := time.Since(created); age < 7*24*time.Hour {
		extra["Flag"] = "New account (created less than 7 days ago)"
	}
	if count := guildMemberCount(s, m.GuildID); count > 0 {
		extra["Member Count"] = fmt.Sprintf("%d", count)
	}

	// Attribute the join to an invite where possible.
	switch used, err := h.deps.Invites.ResolveJoin(s, m.GuildID); {
	case err != nil:
		extra["Invited By"] = "Unknown (Specter lacks the Manage Server permission)"
	case used != nil:
		if used.Inviter != nil {
			extra["Invited By"] = fmt.Sprintf("%s (`%s`)", userTag(used.Inviter), used.Inviter.ID)
		} else {
			extra["Invited By"] = "Unknown inviter"
		}
		detail := fmt.Sprintf("`%s` — %d use", used.Code, used.Uses)
		if used.Uses != 1 {
			detail += "s"
		}
		if used.MaxUses > 0 {
			detail += fmt.Sprintf(" / %d", used.MaxUses)
		}
		if used.Temporary {
			detail += " (temporary membership)"
		}
		extra["Invite"] = detail
		if used.ChannelID != "" {
			extra["Invite Channel"] = "<#" + used.ChannelID + ">"
		}
	default:
		extra["Invited By"] = "Could not determine (vanity URL, server discovery, or bot-added)"
	}

	// Remember the member so a future leave can report tenure and roles.
	h.deps.Invites.SnapshotMember(m.GuildID, m.Member)

	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:    m.GuildID,
		EventType:  modlog.EventMemberJoin,
		TargetID:   m.User.ID,
		TargetName: userTag(m.User),
		Thumbnail:  m.User.AvatarURL("256"),
		Extra:      extra,
		Timestamp:  time.Now(),
	})
}

func (h *Handlers) onGuildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m.User == nil {
		return
	}

	created, _ := discordgo.SnowflakeTimestamp(m.User.ID)
	extra := map[string]string{
		"User":            fmt.Sprintf("%s (`%s`)", userTag(m.User), m.User.ID),
		"Account Created": timestampFull(created),
	}
	if m.User.Bot {
		extra["Type"] = "Bot"
	}

	// Prefer our snapshot; fall back to whatever the event carried.
	snap, ok := h.deps.Invites.PopMember(m.GuildID, m.User.ID)
	joinedAt := snap.JoinedAt
	roles := snap.Roles
	if (!ok || joinedAt.IsZero()) && m.Member != nil {
		joinedAt = m.Member.JoinedAt
	}
	if len(roles) == 0 && m.Member != nil {
		roles = m.Member.Roles
	}

	if !joinedAt.IsZero() {
		extra["Joined Server"] = timestampFull(joinedAt)
		extra["Time in Server"] = humanizeDuration(time.Since(joinedAt))
	}
	if rendered := renderRoles(roles, m.GuildID, 20); rendered != "" {
		extra["Roles"] = rendered
	}
	if count := guildMemberCount(s, m.GuildID); count > 0 {
		extra["Member Count"] = fmt.Sprintf("%d", count)
	}

	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:    m.GuildID,
		EventType:  modlog.EventMemberLeave,
		TargetID:   m.User.ID,
		TargetName: userTag(m.User),
		Thumbnail:  m.User.AvatarURL("256"),
		Extra:      extra,
		Timestamp:  time.Now(),
	})
}

func (h *Handlers) onGuildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.Member == nil || m.User == nil {
		return
	}
	// Keep the member snapshot fresh so leave logs reflect current roles.
	h.deps.Invites.SnapshotMember(m.GuildID, m.Member)

	extra := map[string]string{}
	if m.BeforeUpdate != nil {
		if m.BeforeUpdate.Nick != m.Nick {
			extra["Nickname"] = display(m.BeforeUpdate.Nick) + " → " + display(m.Nick)
		}
		if added, removed := diffRoles(m.BeforeUpdate.Roles, m.Roles); added != "" || removed != "" {
			if added != "" {
				extra["Roles Added"] = added
			}
			if removed != "" {
				extra["Roles Removed"] = removed
			}
		}
	}
	if len(extra) == 0 {
		return
	}
	h.deps.Modlog.Log(s, modlog.ModLogEvent{
		GuildID:    m.GuildID,
		EventType:  modlog.EventMemberUpdate,
		TargetID:   m.User.ID,
		TargetName: userTag(m.User),
		Extra:      extra,
		Timestamp:  time.Now(),
	})
}

func display(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// userTag renders a user as username#discriminator, or just the username for
// migrated accounts whose discriminator is "0".
func userTag(u *discordgo.User) string {
	if u == nil {
		return "Unknown"
	}
	if u.Discriminator == "" || u.Discriminator == "0" {
		return u.Username
	}
	return u.Username + "#" + u.Discriminator
}

// timestampFull renders an absolute + relative Discord timestamp.
func timestampFull(t time.Time) string {
	if t.IsZero() {
		return "Unknown"
	}
	return fmt.Sprintf("<t:%d:F> (<t:%d:R>)", t.Unix(), t.Unix())
}

// humanizeDuration renders a duration using its two largest non-zero units
// (e.g. "1 year, 2 months", "3 days, 4 hours", "5 minutes").
func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d < time.Minute {
		return "less than a minute"
	}

	type unit struct {
		name string
		secs int64
	}
	units := []unit{
		{"year", 365 * 24 * 3600},
		{"month", 30 * 24 * 3600},
		{"day", 24 * 3600},
		{"hour", 3600},
		{"minute", 60},
	}

	remaining := int64(d.Seconds())
	parts := make([]string, 0, 2)
	for _, u := range units {
		if remaining < u.secs {
			continue
		}
		n := remaining / u.secs
		remaining -= n * u.secs
		plural := ""
		if n != 1 {
			plural = "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s%s", n, u.name, plural))
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) == 0 {
		return "less than a minute"
	}
	return strings.Join(parts, ", ")
}

// guildMemberCount reads the live member count from state, 0 if unavailable.
func guildMemberCount(s *discordgo.Session, guildID string) int {
	if s.State == nil {
		return 0
	}
	if g, err := s.State.Guild(guildID); err == nil {
		return g.MemberCount
	}
	return 0
}

// renderRoles renders role IDs as mentions, excluding @everyone (== guildID),
// capping at max with an overflow note.
func renderRoles(roleIDs []string, guildID string, max int) string {
	filtered := make([]string, 0, len(roleIDs))
	for _, id := range roleIDs {
		if id != "" && id != guildID {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	sort.Strings(filtered)
	shown := filtered
	overflow := 0
	if len(shown) > max {
		overflow = len(shown) - max
		shown = shown[:max]
	}
	mentions := make([]string, len(shown))
	for i, id := range shown {
		mentions[i] = "<@&" + id + ">"
	}
	out := strings.Join(mentions, " ")
	if overflow > 0 {
		out += fmt.Sprintf(" and %d more", overflow)
	}
	return out
}

// diffRoles returns mention lists of roles added and removed between two sets.
func diffRoles(before, after []string) (added, removed string) {
	beforeSet := make(map[string]struct{}, len(before))
	for _, id := range before {
		beforeSet[id] = struct{}{}
	}
	afterSet := make(map[string]struct{}, len(after))
	for _, id := range after {
		afterSet[id] = struct{}{}
	}
	var addedIDs, removedIDs []string
	for _, id := range after {
		if _, ok := beforeSet[id]; !ok {
			addedIDs = append(addedIDs, id)
		}
	}
	for _, id := range before {
		if _, ok := afterSet[id]; !ok {
			removedIDs = append(removedIDs, id)
		}
	}
	return joinRoleMentions(addedIDs), joinRoleMentions(removedIDs)
}

func joinRoleMentions(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = "<@&" + id + ">"
	}
	return strings.Join(parts, " ")
}
