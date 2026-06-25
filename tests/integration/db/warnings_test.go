package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/salik/specter/tests/integration/testdb"
)

func TestWarningsAddAndList(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := store.AddWarning(ctx, "g1", "u1", "mod", "reason")
		require.NoError(t, err)
	}
	warns, err := store.ListWarnings(ctx, "g1", "u1")
	require.NoError(t, err)
	assert.Len(t, warns, 3)
}

func TestWarningSoftDelete(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	id, err := store.AddWarning(ctx, "g1", "u1", "mod", "r1")
	require.NoError(t, err)
	_, _ = store.AddWarning(ctx, "g1", "u1", "mod", "r2")

	ok, err := store.RemoveWarning(ctx, "g1", id)
	require.NoError(t, err)
	assert.True(t, ok)

	warns, err := store.ListWarnings(ctx, "g1", "u1")
	require.NoError(t, err)
	assert.Len(t, warns, 1, "removed warning should not appear in active list")
}

func TestWarningGuildScoped(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = store.AddWarning(ctx, "g1", "u1", "mod", "r")
	warns, err := store.ListWarnings(ctx, "g2", "u1")
	require.NoError(t, err)
	assert.Empty(t, warns)
}
