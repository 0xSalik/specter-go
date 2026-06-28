package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/internal/db"
	"github.com/0xSalik/specter/tests/integration/testdb"
)

func TestGuildInsertAndFetch(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	isNew, err := store.EnsureGuild(ctx, "g1")
	require.NoError(t, err)
	assert.True(t, isNew)

	g, err := store.GetGuild(ctx, "g1")
	require.NoError(t, err)
	assert.Equal(t, "#5865F2", g.EmbedColor)
}

func TestGuildUpsertIdempotent(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.EnsureGuild(ctx, "g1")
	require.NoError(t, err)
	isNew, err := store.EnsureGuild(ctx, "g1")
	require.NoError(t, err)
	assert.False(t, isNew, "second insert must not create a new row")
}

func TestGuildColorUpdate(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = store.EnsureGuild(ctx, "g1")
	require.NoError(t, store.SetEmbedColor(ctx, "g1", "#FF0000"))
	g, err := store.GetGuild(ctx, "g1")
	require.NoError(t, err)
	assert.Equal(t, "#FF0000", g.EmbedColor)
}

func TestGuildNotFound(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	_, err := store.GetGuild(context.Background(), "missing")
	assert.True(t, db.IsNotFound(err))
}

func TestGuildDeleteCascades(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = store.EnsureGuild(ctx, "g1")
	_, err := store.AddWarning(ctx, "g1", "u1", "m1", "test")
	require.NoError(t, err)

	require.NoError(t, store.DeleteGuild(ctx, "g1"))

	_, err = store.GetGuild(ctx, "g1")
	assert.True(t, db.IsNotFound(err))
	warns, err := store.ListWarnings(ctx, "g1", "u1")
	require.NoError(t, err)
	assert.Empty(t, warns)
}
