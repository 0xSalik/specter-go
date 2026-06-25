package automod_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/salik/specter/internal/automod"
	"github.com/salik/specter/internal/db/queries"
)

func TestAntiInvite(t *testing.T) {
	assert.True(t, automod.HasInvite("join discord.gg/abc now"))
	assert.True(t, automod.HasInvite("https://discord.com/invite/xyz"))
	assert.False(t, automod.HasInvite("just a normal message"))
}

func TestAntiLink(t *testing.T) {
	assert.True(t, automod.HasDisallowedLink("see https://evil.example.com", []string{"trusted.com"}))
	assert.False(t, automod.HasDisallowedLink("see https://trusted.com/page", []string{"trusted.com"}))
	assert.False(t, automod.HasDisallowedLink("no links here", nil))
	assert.True(t, automod.HasDisallowedLink("https://anything.com", nil))
}

func TestAntiCaps(t *testing.T) {
	assert.True(t, automod.ExceedsCaps("HELLO WORLD THIS IS CAPS", 70))
	assert.False(t, automod.ExceedsCaps("Hello World this is normal", 70))
	assert.False(t, automod.ExceedsCaps("SHORT", 70)) // under 10 chars
}

func TestBadWords(t *testing.T) {
	words := []string{"badword", "spam"}
	assert.True(t, automod.ContainsBadWord("this is a BadWord here", words))
	assert.True(t, automod.ContainsBadWord("contains badwordsuffix", words)) // substring
	assert.False(t, automod.ContainsBadWord("totally clean text", words))
}

func TestExemptBypass(t *testing.T) {
	assert.True(t, automod.IsExempt([]string{"modrole"}, "c", []string{"modrole"}, nil))
	assert.True(t, automod.IsExempt(nil, "general", nil, []string{"general"}))
	assert.False(t, automod.IsExempt([]string{"member"}, "c", []string{"modrole"}, []string{"other"}))
}

func TestSpamThreshold(t *testing.T) {
	c := automod.NewChecker()
	defer c.Close()
	key := "g:u:c"
	// Threshold 3 within a wide window: first two pass, third trips.
	assert.False(t, c.RecordAndCheckSpam(key, 3, 60))
	assert.False(t, c.RecordAndCheckSpam(key, 3, 60))
	assert.True(t, c.RecordAndCheckSpam(key, 3, 60))
}

func TestEvaluateRuleDisabled(t *testing.T) {
	c := automod.NewChecker()
	defer c.Close()
	cfg := queries.DefaultAutomodConfig("g")
	cfg.Enabled = true
	cfg.AntiInviteEnabled = false
	v := c.Evaluate(cfg, "g:u:c", "discord.gg/abc", nil, "c")
	assert.Nil(t, v, "invite should pass when the rule is disabled")
}

func TestEvaluateInviteEnabled(t *testing.T) {
	c := automod.NewChecker()
	defer c.Close()
	cfg := queries.DefaultAutomodConfig("g")
	cfg.Enabled = true
	cfg.AntiInviteEnabled = true
	v := c.Evaluate(cfg, "g:u:c", "discord.gg/abc", nil, "c")
	if assert.NotNil(t, v) {
		assert.Equal(t, "invite", v.Rule)
	}
}

func TestEvaluateExemptRole(t *testing.T) {
	c := automod.NewChecker()
	defer c.Close()
	cfg := queries.DefaultAutomodConfig("g")
	cfg.Enabled = true
	cfg.AntiInviteEnabled = true
	cfg.ExemptRoles = []string{"vip"}
	v := c.Evaluate(cfg, "g:u:c", "discord.gg/abc", []string{"vip"}, "c")
	assert.Nil(t, v)
}
