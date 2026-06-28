package levels_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/0xSalik/specter/internal/levels"
)

func TestLevelForXP(t *testing.T) {
	assert.Equal(t, 0, levels.LevelForXP(0))
	assert.Equal(t, 0, levels.LevelForXP(50))
	assert.Equal(t, 1, levels.LevelForXP(100)) // 0.1*sqrt(100)=1
	assert.Equal(t, 2, levels.LevelForXP(400)) // 0.1*sqrt(400)=2
	assert.Equal(t, 3, levels.LevelForXP(900)) // 0.1*sqrt(900)=3
	assert.Equal(t, 10, levels.LevelForXP(10000))
}

func TestCalculateXPForLevel(t *testing.T) {
	for n := 0; n <= 100; n++ {
		xp := levels.CalculateXPForLevel(n)
		assert.Equal(t, int64(100*n*n), xp)
		// The threshold XP for level n must map back to at least level n.
		assert.GreaterOrEqual(t, levels.LevelForXP(xp), n)
	}
}

func TestLevelUpDetection(t *testing.T) {
	oldXP := int64(90)
	newXP := int64(110)
	assert.Equal(t, 0, levels.LevelForXP(oldXP))
	assert.Equal(t, 1, levels.LevelForXP(newXP))
	assert.Greater(t, levels.LevelForXP(newXP), levels.LevelForXP(oldXP))
}

func TestOnCooldown(t *testing.T) {
	now := time.Now()
	recent := now.Add(-10 * time.Second)
	old := now.Add(-120 * time.Second)
	assert.True(t, levels.OnCooldown(&recent, now, 60))
	assert.False(t, levels.OnCooldown(&old, now, 60))
	assert.False(t, levels.OnCooldown(nil, now, 60))
}

func TestAwardXPRange(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 1000; i++ {
		xp := levels.AwardXP(r, 15, 40)
		assert.GreaterOrEqual(t, xp, int64(15))
		assert.LessOrEqual(t, xp, int64(40))
	}
}

func TestIsExempt(t *testing.T) {
	assert.True(t, levels.IsExempt([]string{"role1"}, "chan", []string{"role1"}, nil))
	assert.True(t, levels.IsExempt(nil, "chan", nil, []string{"chan"}))
	assert.False(t, levels.IsExempt([]string{"role2"}, "chan", []string{"role1"}, []string{"other"}))
}
