package modlog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/salik/specter/internal/db/queries"
	"github.com/salik/specter/internal/modlog"
)

func ptr(s string) *string { return &s }

func testGuild() *queries.GuildConfig {
	return &queries.GuildConfig{
		GeneralLogID: ptr("general"),
		UserLogID:    ptr("user"),
		MessageLogID: ptr("message"),
		WarnLogID:    ptr("warn"),
		KickLogID:    ptr("kick"),
		BanLogID:     ptr("ban"),
	}
}

func TestEventRouting(t *testing.T) {
	g := testGuild()
	cases := map[string]string{
		modlog.EventMemberJoin:    "user",
		modlog.EventMemberLeave:   "user",
		modlog.EventMemberUpdate:  "user",
		modlog.EventMessageDelete: "message",
		modlog.EventMessageEdit:   "message",
		modlog.EventWarn:          "warn",
		modlog.EventTimeout:       "warn",
		modlog.EventKick:          "kick",
		modlog.EventBan:           "ban",
		modlog.EventUnban:         "ban",
		modlog.EventChannelUpdate: "general",
		modlog.EventGuildUpdate:   "general",
	}
	for event, expected := range cases {
		field := modlog.ChannelField(g, event)
		require.NotNil(t, field, "event %s should route to a channel", event)
		assert.Equal(t, expected, *field, "event %s routed incorrectly", event)
	}
}

func TestRoutingNilWhenUnconfigured(t *testing.T) {
	g := &queries.GuildConfig{}
	assert.Nil(t, modlog.ChannelField(g, modlog.EventBan))
}
