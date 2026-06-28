package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/internal/db/queries"
	"github.com/0xSalik/specter/tests/integration/testdb"
)

func TestAccessRuleUpsertAndList(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	rule := queries.AccessRule{GuildID: "g1", CommandGroup: "moderation", EntityType: "role", EntityID: "r1", Allowed: true}
	require.NoError(t, store.SetAccessRule(ctx, rule))

	// Upsert flips allowed.
	rule.Allowed = false
	require.NoError(t, store.SetAccessRule(ctx, rule))

	rules, err := store.ListAccessRules(ctx, "g1", "moderation")
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.False(t, rules[0].Allowed)
}

func TestAccessRuleDelete(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	rule := queries.AccessRule{GuildID: "g1", CommandGroup: "fun", EntityType: "user", EntityID: "u1", Allowed: true}
	require.NoError(t, store.SetAccessRule(ctx, rule))
	require.NoError(t, store.DeleteAccessRule(ctx, "g1", "fun", "user", "u1"))

	rules, err := store.ListAccessRules(ctx, "g1", "fun")
	require.NoError(t, err)
	assert.Empty(t, rules)
}
