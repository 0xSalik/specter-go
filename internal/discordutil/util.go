// Package discordutil holds stateless helpers shared across command handlers:
// role-hierarchy checks, duration parsing, and small formatting utilities.
package discordutil

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// HighestRolePosition returns the position of a member's highest role within a
// guild. Roles not found in the guild are ignored.
func HighestRolePosition(guild *discordgo.Guild, roleIDs []string) int {
	pos := 0
	roleByID := make(map[string]*discordgo.Role, len(guild.Roles))
	for _, r := range guild.Roles {
		roleByID[r.ID] = r
	}
	for _, id := range roleIDs {
		if r, ok := roleByID[id]; ok && r.Position > pos {
			pos = r.Position
		}
	}
	return pos
}

// CanActOn verifies the bot can moderate the target and that the invoking
// moderator outranks them. It fetches guild and member data from state/REST.
// Returns a clear reason when the action must be refused.
func CanActOn(s *discordgo.Session, guildID, modID, targetID string) (bool, string) {
	guild, err := s.State.Guild(guildID)
	if err != nil || guild == nil {
		guild, err = s.Guild(guildID)
		if err != nil {
			return false, "Could not load server information to verify role hierarchy."
		}
	}
	if targetID == guild.OwnerID {
		return false, "You cannot moderate the server owner."
	}

	botMember, err := s.State.Member(guildID, s.State.User.ID)
	if err != nil || botMember == nil {
		botMember, err = s.GuildMember(guildID, s.State.User.ID)
		if err != nil {
			return false, "Could not determine the bot's roles to verify hierarchy."
		}
	}
	targetMember, err := s.State.Member(guildID, targetID)
	if err != nil || targetMember == nil {
		targetMember, err = s.GuildMember(guildID, targetID)
		if err != nil {
			// Target not in guild (e.g. ban by ID) — nothing to compare.
			return true, ""
		}
	}

	botPos := HighestRolePosition(guild, botMember.Roles)
	targetPos := HighestRolePosition(guild, targetMember.Roles)
	if targetPos >= botPos {
		return false, "The bot's highest role must be above the target's highest role to perform this action."
	}

	if modID != guild.OwnerID {
		modMember, err := s.State.Member(guildID, modID)
		if err != nil || modMember == nil {
			modMember, _ = s.GuildMember(guildID, modID)
		}
		if modMember != nil {
			modPos := HighestRolePosition(guild, modMember.Roles)
			if targetPos >= modPos {
				return false, "You cannot moderate a member whose highest role is equal to or above your own."
			}
		}
	}
	return true, ""
}

var durationRe = regexp.MustCompile(`^(\d+)\s*([smhdw])$`)

// ParseDuration parses human durations like "30s", "10m", "1h", "1d", "1w".
func ParseDuration(s string) (time.Duration, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	m := durationRe.FindStringSubmatch(s)
	if m == nil {
		return 0, errors.New("invalid duration; use formats like 30s, 10m, 1h, 1d, 1w")
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, err
	}
	unit := map[string]time.Duration{
		"s": time.Second, "m": time.Minute, "h": time.Hour,
		"d": 24 * time.Hour, "w": 7 * 24 * time.Hour,
	}[m[2]]
	return time.Duration(n) * unit, nil
}

// FormatDuration renders a duration in a compact human form.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	var parts []string
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	if secs > 0 && days == 0 {
		parts = append(parts, fmt.Sprintf("%ds", secs))
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, " ")
}

// AvatarURL returns a PNG avatar URL for a user, preferring the animated form
// only when needed. PNG is forced so it can be decoded for rank cards.
func AvatarURL(u *discordgo.User) string {
	if u == nil {
		return ""
	}
	return u.AvatarURL("256")
}
