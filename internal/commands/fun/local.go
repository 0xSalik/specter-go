package fun

import (
	"math/rand"
	"strings"

	"github.com/0xSalik/specter/internal/core"
)

func handleFlip(c *core.Context) {
	result := "Heads"
	if rand.Intn(2) == 1 {
		result = "Tails"
	}
	_ = c.Reply(c.Embed().Title("Coin Flip").Description("The coin landed on **" + result + "**.").Build())
}

var threatPool = []string{
	"I will alphabetize your bookshelf incorrectly.",
	"I will replace your coffee with decaf.",
	"I will set all your clocks five minutes fast.",
	"I will leave one sock unmatched, forever.",
	"I will reply-all to that email.",
	"I will move your mouse pointer one pixel at a time.",
	"I will breathe near your sourdough starter.",
	"I will rename all your files to 'final_final_v2'.",
	"I will adjust your thermostat by a single degree.",
	"I will untuck your fitted sheet.",
	"I will close all your browser tabs.",
	"I will turn your autocorrect to a different dialect.",
	"I will eat the last slice and leave the empty box.",
	"I will park slightly over the line next to you.",
	"I will whistle a song you can't identify.",
	"I will swap your salt and sugar.",
	"I will let the screen door slam.",
	"I will read the ending of your book aloud.",
	"I will set your ringtone to a marimba.",
	"I will leave the gas tank on empty.",
	"I will fold your laundry with the seams out.",
	"I will use your good scissors on tape.",
	"I will leave voicemails instead of texting.",
	"I will reorganize your spice rack by vibe.",
	"I will leave 2% battery on every device.",
	"I will hum the wrong tune at the right moment.",
	"I will tell you the score before you watch it.",
	"I will leave the printer out of paper.",
	"I will respond only with 'k'.",
	"I will put the milk back with two drops left.",
}

func handleThreats(c *core.Context) {
	idx := rand.Perm(len(threatPool))[:3]
	var sb strings.Builder
	for _, i := range idx {
		sb.WriteString("• " + threatPool[i] + "\n")
	}
	_ = c.Reply(c.Embed().Title("A Stern Warning").Description(sb.String()).Build())
}

func handleUwuify(c *core.Context) {
	text := c.StringOpt("text", "")
	_ = c.Reply(c.Embed().Title("uwuified").Description(Uwuify(text)).Build())
}

// Uwuify performs the text transformation. Exported for unit testing.
func Uwuify(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	prevSpace := true
	for _, r := range s {
		switch r {
		case 'r', 'l':
			b.WriteRune('w')
		case 'R', 'L':
			b.WriteRune('W')
		default:
			b.WriteRune(r)
		}
		_ = prevSpace
	}
	out := b.String()
	out = strings.ReplaceAll(out, "th", "d")
	if rand.Intn(2) == 0 {
		out += " nya~"
	}
	return out
}
