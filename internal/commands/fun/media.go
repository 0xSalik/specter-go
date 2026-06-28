package fun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xSalik/specter/internal/core"
)

// discordFileLimit is the default non-boosted upload limit (25 MB).
const discordFileLimit = 25 * 1024 * 1024

func handleTikTok(c *core.Context) { download(c, nil) }

func handleYTDownload(c *core.Context) {
	download(c, []string{"--format", "bestvideo[height<=720]+bestaudio/best[height<=720]"})
}

func download(c *core.Context, extraArgs []string) {
	rawURL := c.StringOpt("url", "")
	if !strings.HasPrefix(rawURL, "http") {
		_ = c.Errorf("You must provide a valid URL.", nil)
		return
	}
	_ = c.Defer(false)

	bin := c.Config.YTDLPPath
	if bin == "" {
		bin = "yt-dlp"
	}
	if _, err := exec.LookPath(bin); err != nil {
		_ = c.Errorf("Downloads are unavailable: yt-dlp is not installed on the host.", nil)
		return
	}

	tmpDir, err := os.MkdirTemp("", "specter-dl-*")
	if err != nil {
		_ = c.Errorf("Failed to prepare a temporary download directory.", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	args := []string{"--no-playlist", "--no-warnings", "-o", filepath.Join(tmpDir, "media.%(ext)s")}
	args = append(args, extraArgs...)
	args = append(args, rawURL)

	cmd := exec.CommandContext(ctx, bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = c.Errorf("Failed to download that media: "+lastLine(string(out)), err)
		return
	}

	path, err := firstFile(tmpDir)
	if err != nil {
		_ = c.Errorf("The download produced no file.", err)
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		_ = c.Errorf("Could not read the downloaded file.", err)
		return
	}
	if info.Size() > discordFileLimit {
		_ = c.Errorf(fmt.Sprintf("The downloaded file is %.1f MB, which exceeds Discord's 25 MB upload limit.", float64(info.Size())/1024/1024), nil)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		_ = c.Errorf("Could not read the downloaded file.", err)
		return
	}
	e := c.Embed().Title("Download Complete").Description("Here is your media.").AsSuccess().Build()
	_ = c.ReplyFile(e, filepath.Base(path), data)
}

func firstFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", errors.New("no file produced")
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown error"
	}
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}
