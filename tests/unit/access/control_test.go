package access_test

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"

	"github.com/salik/specter/internal/access"
	"github.com/salik/specter/internal/db/queries"
)

func TestAdministratorAlwaysPasses(t *testing.T) {
	ok, _ := access.Decide(discordgo.PermissionAdministrator, "u", nil, discordgo.PermissionBanMembers, nil)
	assert.True(t, ok)
}

func TestMissingDiscordPermissionDenied(t *testing.T) {
	// Custom allow does not override a missing Discord permission.
	rules := []queries.AccessRule{{CommandGroup: "moderation", EntityType: "user", EntityID: "u", Allowed: true}}
	ok, reason := access.Decide(0, "u", nil, discordgo.PermissionBanMembers, rules)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestNoCustomRulesFallsBackToPermission(t *testing.T) {
	ok, _ := access.Decide(discordgo.PermissionBanMembers, "u", nil, discordgo.PermissionBanMembers, nil)
	assert.True(t, ok)
}

func TestCustomDenyOverridesDefaultAllow(t *testing.T) {
	rules := []queries.AccessRule{{CommandGroup: "fun", EntityType: "role", EntityID: "r1", Allowed: false}}
	ok, _ := access.Decide(0, "u", []string{"r1"}, 0, rules)
	assert.False(t, ok)
}

func TestCustomAllowForRole(t *testing.T) {
	rules := []queries.AccessRule{{CommandGroup: "fun", EntityType: "role", EntityID: "r1", Allowed: true}}
	// Member with the role passes.
	ok, _ := access.Decide(0, "u", []string{"r1"}, 0, rules)
	assert.True(t, ok)
	// Member without the role is restricted by the allow-list.
	ok, _ = access.Decide(0, "u", []string{"other"}, 0, rules)
	assert.False(t, ok)
}
