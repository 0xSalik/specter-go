package fun

import (
	"bytes"
	"os"
	"strings"

	"github.com/fogleman/gg"

	"github.com/salik/specter/internal/core"
)

func handleTweet(c *core.Context) {
	username := c.StringOpt("username", "")
	text := c.StringOpt("text", "")
	if username == "" || text == "" {
		_ = c.Errorf("A username and text are required.", nil)
		return
	}
	_ = c.Defer(false)

	img, err := renderTweet(username, text)
	if err != nil {
		_ = c.Errorf("Failed to render the tweet image.", err)
		return
	}
	e := c.Embed().Title("Tweet").Image("attachment://tweet.png").Build()
	_ = c.ReplyFile(e, "tweet.png", img)
}

func renderTweet(name, text string) ([]byte, error) {
	const w = 800
	lines := wrapText(text, 60)
	h := 200 + len(lines)*40

	dc := gg.NewContext(w, h)
	dc.SetRGB(0.08, 0.09, 0.10) // X dark mode
	dc.Clear()

	// Avatar circle.
	dc.SetRGB(0.35, 0.38, 0.45)
	dc.DrawCircle(60, 70, 32)
	dc.Fill()

	font := findTweetFont()
	if font != "" {
		_ = dc.LoadFontFace(font, 28)
		dc.SetRGB(1, 1, 1)
		dc.DrawString(name, 110, 62)
		_ = dc.LoadFontFace(font, 22)
		dc.SetRGB(0.45, 0.5, 0.56)
		dc.DrawString("@"+strings.ToLower(strings.ReplaceAll(name, " ", "")), 110, 92)

		_ = dc.LoadFontFace(font, 30)
		dc.SetRGB(0.95, 0.96, 0.98)
		y := 150.0
		for _, ln := range lines {
			dc.DrawString(ln, 40, y)
			y += 40
		}
	}

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func wrapText(s string, max int) []string {
	words := strings.Fields(s)
	var lines []string
	var cur string
	for _, w := range words {
		if len(cur)+len(w)+1 > max {
			lines = append(lines, cur)
			cur = w
		} else if cur == "" {
			cur = w
		} else {
			cur += " " + w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

var tweetFontCandidates = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/TTF/DejaVuSans.ttf",
	"/System/Library/Fonts/Supplemental/Arial.ttf",
	"/Library/Fonts/Arial.ttf",
}

func findTweetFont() string {
	if env := os.Getenv("SPECTER_FONT"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	for _, p := range tweetFontCandidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
