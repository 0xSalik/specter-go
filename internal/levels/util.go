package levels

import (
	"strconv"
	"strings"
)

func itoa(n int) string { return strconv.Itoa(n) }

// renderAnnounce substitutes the supported template variables into a custom
// level-up message: {user} and {level}.
func renderAnnounce(tmpl, userID string, level int) string {
	r := strings.NewReplacer(
		"{user}", "<@"+userID+">",
		"{level}", strconv.Itoa(level),
	)
	return r.Replace(tmpl)
}
