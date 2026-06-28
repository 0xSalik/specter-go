package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/tests/integration/testdb"
)

func TestReactionRoleMenuLifecycle(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	id, err := store.CreateMenu(ctx, "g1", "chan", "msg", "Roles", nil, "normal")
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := store.AddEntry(ctx, id, "emoji", "role")
		require.NoError(t, err)
	}
	entries, err := store.ListEntries(ctx, id)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestReactionRoleCascadeDelete(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	id, _ := store.CreateMenu(ctx, "g1", "chan", "msg", "Roles", nil, "normal")
	_, _ = store.AddEntry(ctx, id, "e", "r")

	ok, err := store.DeleteMenu(ctx, "g1", id)
	require.NoError(t, err)
	assert.True(t, ok)

	count, err := store.CountEntries(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "entries must cascade delete with the menu")
}

func TestGetMenuByMessage(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	id, _ := store.CreateMenu(ctx, "g1", "chan", "msg123", "Roles", nil, "unique")
	menu, err := store.GetMenuByMessage(ctx, "msg123")
	require.NoError(t, err)
	assert.Equal(t, id, menu.ID)
	assert.Equal(t, "unique", menu.Type)
}
