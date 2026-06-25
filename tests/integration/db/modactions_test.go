package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/salik/specter/tests/integration/testdb"
)

func TestRecordAndListActions(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	reason := "spam"
	_, err := store.RecordAction(ctx, "g1", "u1", "mod", "ban", &reason, nil)
	require.NoError(t, err)
	_, err = store.RecordAction(ctx, "g1", "u1", "mod", "warn", nil, nil)
	require.NoError(t, err)

	actions, err := store.ListActions(ctx, "g1", "u1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, actions, 2)

	count, err := store.CountActions(ctx, "g1", "u1")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestClearActionsScoped(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = store.RecordAction(ctx, "g1", "u1", "mod", "ban", nil, nil)
	_, _ = store.RecordAction(ctx, "g1", "u2", "mod", "ban", nil, nil)

	require.NoError(t, store.ClearActions(ctx, "g1", "u1"))

	c1, _ := store.CountActions(ctx, "g1", "u1")
	c2, _ := store.CountActions(ctx, "g1", "u2")
	assert.Equal(t, 0, c1)
	assert.Equal(t, 1, c2, "other users must be unaffected")
}

func TestCountActionsByType(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = store.RecordAction(ctx, "g1", "u1", "mod", "ban", nil, nil)
	_, _ = store.RecordAction(ctx, "g1", "u2", "mod", "ban", nil, nil)
	_, _ = store.RecordAction(ctx, "g1", "u3", "mod", "kick", nil, nil)

	bans, err := store.CountActionsByType(ctx, "g1", "ban")
	require.NoError(t, err)
	assert.Equal(t, 2, bans)
}
