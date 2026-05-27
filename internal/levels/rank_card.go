package levels

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"time"

	"github.com/fogleman/gg"
)

// RankCardData holds everything needed to render a rank card.
type RankCardData struct {
	Username  string
	Discrim   string
	AvatarURL string
	Level     int
	Rank      int
	XP        int64
	TotalMsgs int64
}

// RenderRankCard produces a clean, minimal PNG rank card. The avatar is fetched
// over HTTP (best-effort); on failure a neutral placeholder circle is drawn.
func RenderRankCard(ctx context.Context, d RankCardData) ([]byte, error) {
	const w, h = 900, 280
	dc := gg.NewContext(w, h)

	// Background.
	dc.SetRGB(0.13, 0.14, 0.18)
	dc.Clear()

	// Accent panel.
	dc.SetRGB(0.10, 0.11, 0.15)
	dc.DrawRoundedRectangle(20, 20, w-40, h-40, 24)
	dc.Fill()

	// Avatar.
	const ax, ay, ar = 90.0, 140.0, 70.0
	if img := fetchAvatar(ctx, d.AvatarURL); img != nil {
		dc.Push()
		dc.DrawCircle(ax, ay, ar)
		dc.Clip()
		scaled := resizeToSquare(img, int(ar*2))
		dc.DrawImage(scaled, int(ax-ar), int(ay-ar))
		dc.ResetClip()
		dc.Pop()
	} else {
		dc.SetRGB(0.35, 0.38, 0.95)
		dc.DrawCircle(ax, ay, ar)
		dc.Fill()
	}
	dc.SetRGB(0.35, 0.38, 0.95)
	dc.SetLineWidth(5)
	dc.DrawCircle(ax, ay, ar)
	dc.Stroke()

	// Username.
	dc.SetRGB(1, 1, 1)
	if err := dc.LoadFontFace(findFont(), 40); err == nil {
		name := d.Username
		if d.Discrim != "" && d.Discrim != "0" {
			name = fmt.Sprintf("%s#%s", d.Username, d.Discrim)
		}
		dc.DrawString(name, 200, 90)
	}

	// Stats line.
	if err := dc.LoadFontFace(findFont(), 26); err == nil {
		dc.SetRGB(0.75, 0.78, 0.85)
		dc.DrawString(fmt.Sprintf("Rank #%d    Level %d    %d messages", d.Rank, d.Level, d.TotalMsgs), 200, 130)
	}

	// XP progress bar.
	curBase := CalculateXPForLevel(d.Level)
	nextBase := CalculateXPForLevel(d.Level + 1)
	span := nextBase - curBase
	progress := 0.0
	if span > 0 {
		progress = float64(d.XP-curBase) / float64(span)
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	const barX, barY, barW, barH = 200.0, 170.0, 640.0, 34.0
	dc.SetRGB(0.2, 0.22, 0.28)
	dc.DrawRoundedRectangle(barX, barY, barW, barH, barH/2)
	dc.Fill()
	dc.SetRGB(0.35, 0.38, 0.95)
	dc.DrawRoundedRectangle(barX, barY, barW*progress, barH, barH/2)
	dc.Fill()

	if err := dc.LoadFontFace(findFont(), 20); err == nil {
		dc.SetRGB(0.85, 0.87, 0.92)
		dc.DrawString(fmt.Sprintf("%d / %d XP", d.XP-curBase, span), barX, barY+barH+26)
	}

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("encode rank card: %w", err)
	}
	return buf.Bytes(), nil
}

func fetchAvatar(ctx context.Context, url string) image.Image {
	if url == "" {
		return nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return img
}

func resizeToSquare(src image.Image, size int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	for y := 0; y < size; y++ {
		sy := b.Min.Y + y*sh/size
		for x := 0; x < size; x++ {
			sx := b.Min.X + x*sw/size
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

var _ = color.RGBA{}
