package moderation

import (
	"fmt"
	"strings"
	"time"

	"github.com/salik/specter/internal/core"
	"github.com/salik/specter/internal/modlog"
)

func handleMassban(c *core.Context) {
	raw := c.StringOpt("user_ids", "")
	reason := c.StringOpt("reason", "Mass ban")
	ids := parseIDs(raw)
	if len(ids) == 0 {
		_ = c.Errorf("No valid user IDs were provided.", nil)
		return
	}
	if len(ids) > 50 {
		_ = c.Errorf("You can mass ban at most 50 users at once.", nil)
		return
	}

	_ = c.Defer(false)

	var success, failed []string
	for _, id := range ids {
		if err := c.Session.GuildBanCreateWithReason(c.GuildID, id, reason, 0); err != nil {
			failed = append(failed, id)
		} else {
			success = append(success, id)
			recordAndLog(c, modlog.EventBan, "ban", id, id, reason, nil)
		}
		// Small delay to respect rate limits.
		time.Sleep(300 * time.Millisecond)
	}

	b := c.Embed().Title("Mass Ban Complete").
		Field("Banned", fmt.Sprintf("%d", len(success)), true).
		Field("Failed", fmt.Sprintf("%d", len(failed)), true).Timestamp()
	if len(failed) > 0 {
		b.Field("Failed IDs", truncate(strings.Join(failed, ", "), 1000), false)
		b.AsError()
	} else {
		b.AsSuccess()
	}
	_ = c.Reply(b.Build())
}

func parseIDs(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ' ' || r == '\n' || r == ',' || r == '\t' || r == '\r'
	})
	var out []string
	seen := make(map[string]struct{})
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if !isSnowflake(f) {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}

func isSnowflake(s string) bool {
	if len(s) < 15 || len(s) > 20 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
