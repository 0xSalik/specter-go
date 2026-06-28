// Package access implements the permission gate that every command handler
// passes through before executing. It enforces built-in Discord permissions
// first and then layers configurable per-guild allow/deny rules on top.
package access

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/db/queries"
)

// Gate evaluates whether an interaction may proceed.
type Gate struct {
	store *queries.Store
}

// NewGate constructs a Gate backed by the query store.
func NewGate(store *queries.Store) *Gate {
	return &Gate{store: store}
}

// Check returns whether the interaction should proceed and, on denial, a clear
// human-readable reason.
//
// Evaluation order:
//  1. Administrators always pass.
//  2. The built-in Discord permission requirement is always enforced.
//  3. Custom ACL rules (deny takes precedence over allow) refine access.
func (g *Gate) Check(i *discordgo.InteractionCreate, commandGroup string, requiredPerm int64) (bool, string) {
	member := i.Member
	if member == nil {
		// DM context: no guild permissions apply; allow non-privileged groups only.
		if requiredPerm == 0 {
			return true, ""
		}
		return false, "This command can only be used inside a server."
	}

	userID := ""
	if member.User != nil {
		userID = member.User.ID
	}

	rules, err := g.fetchRules(i.GuildID, commandGroup)
	if err != nil {
		rules = nil // fall back to the Discord permission check inside Decide
	}
	return Decide(member.Permissions, userID, member.Roles, requiredPerm, rules)
}

// Decide is the pure access-control decision used by Check. It is exported so
// the logic can be unit-tested without a database or Discord session.
//
//   - Administrators always pass.
//   - The built-in Discord permission requirement is always enforced.
//   - Custom ACL rules refine access: a matching deny always wins; if any allow
//     rule exists for the group, only matching members pass.
func Decide(perms int64, userID string, roleIDs []string, requiredPerm int64, rules []queries.AccessRule) (bool, string) {
	if perms&discordgo.PermissionAdministrator != 0 {
		return true, ""
	}
	if requiredPerm != 0 && perms&requiredPerm == 0 {
		return false, "You lack the required Discord permission to use this command."
	}
	if len(rules) == 0 {
		return true, ""
	}

	roleSet := make(map[string]struct{}, len(roleIDs))
	for _, r := range roleIDs {
		roleSet[r] = struct{}{}
	}

	var explicitAllow, anyAllowRuleExists bool
	for _, r := range rules {
		matches := (r.EntityType == "user" && r.EntityID == userID) ||
			(r.EntityType == "role" && contains(roleSet, r.EntityID))
		if r.Allowed {
			anyAllowRuleExists = true
		}
		if !matches {
			continue
		}
		if !r.Allowed {
			return false, "Access to this command group has been restricted by a server administrator."
		}
		explicitAllow = true
	}

	if anyAllowRuleExists && !explicitAllow {
		return false, "This command group is restricted to specific roles or users."
	}
	return true, ""
}

func (g *Gate) fetchRules(guildID, group string) ([]queries.AccessRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return g.store.ListAccessRules(ctx, guildID, group)
}

func contains(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}
