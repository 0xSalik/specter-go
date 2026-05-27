package levels

import (
	"os"
	"sync"

	// Register decoders so avatars in these formats can be loaded.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

var (
	fontOnce sync.Once
	fontPath string
)

// candidateFonts lists common TrueType font locations across Linux/macOS so the
// rank card can render text wherever it runs. If none is found, text drawing is
// skipped gracefully by the caller.
var candidateFonts = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/TTF/DejaVuSans.ttf",
	"/usr/share/fonts/dejavu/DejaVuSans.ttf",
	"/System/Library/Fonts/Supplemental/Arial.ttf",
	"/System/Library/Fonts/Helvetica.ttc",
	"/Library/Fonts/Arial.ttf",
}

func findFont() string {
	fontOnce.Do(func() {
		if env := os.Getenv("SPECTER_FONT"); env != "" {
			if _, err := os.Stat(env); err == nil {
				fontPath = env
				return
			}
		}
		for _, p := range candidateFonts {
			if _, err := os.Stat(p); err == nil {
				fontPath = p
				return
			}
		}
	})
	return fontPath
}
