package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/internal/db/queries"
	"github.com/0xSalik/specter/tests/integration/testdb"
)

func TestAutomodDefaults(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	cfg, err := store.GetAutomodConfig(ctx, "g1")
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
	assert.Equal(t, 5, cfg.AntiSpamThreshold)
	assert.Equal(t, "delete", cfg.Action)
}

func TestAutomodUpsert(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	cfg := queries.DefaultAutomodConfig("g1")
	cfg.Enabled = true
	cfg.BadWordsEnabled = true
	cfg.BadWords = []string{"foo", "bar"}
	require.NoError(t, store.UpsertAutomodConfig(ctx, cfg))

	got, err := store.GetAutomodConfig(ctx, "g1")
	require.NoError(t, err)
	assert.True(t, got.Enabled)
	assert.Equal(t, []string{"foo", "bar"}, got.BadWords)
}
