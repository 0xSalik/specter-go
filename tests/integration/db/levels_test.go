package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/tests/integration/testdb"
)

func TestLevelRankSingleUser(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.AddXP(ctx, "g1", "u1", 100, 1, time.Now())
	require.NoError(t, err)
	rank, err := store.GetRank(ctx, "g1", "u1")
	require.NoError(t, err)
	assert.Equal(t, 1, rank)
}

func TestLeaderboardOrdering(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	xps := map[string]int64{"a": 500, "b": 100, "c": 300, "d": 900, "e": 50}
	for u, xp := range xps {
		_, err := store.AddXP(ctx, "g1", u, xp, 0, time.Now())
		require.NoError(t, err)
	}
	top, err := store.GetTopN(ctx, "g1", 3, 0)
	require.NoError(t, err)
	require.Len(t, top, 3)
	assert.Equal(t, "d", top[0].UserID)
	assert.Equal(t, "a", top[1].UserID)
	assert.Equal(t, "c", top[2].UserID)
}

func TestLevelUpsert(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = store.AddXP(ctx, "g1", "u1", 50, 0, time.Now())
	entry, err := store.AddXP(ctx, "g1", "u1", 25, 0, time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(75), entry.XP)
	assert.Equal(t, int64(2), entry.TotalMsgs)
}

func TestLeaderboardGuildScoped(t *testing.T) {
	store, cleanup := testdb.Setup(t)
	defer cleanup()
	ctx := context.Background()

	_, _ = store.AddXP(ctx, "g1", "u1", 100, 0, time.Now())
	_, _ = store.AddXP(ctx, "g2", "u2", 200, 0, time.Now())

	top, err := store.GetTopN(ctx, "g1", 10, 0)
	require.NoError(t, err)
	require.Len(t, top, 1)
	assert.Equal(t, "u1", top[0].UserID)
}
