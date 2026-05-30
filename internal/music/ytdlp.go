package music

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ErrYTDLPMissing indicates the yt-dlp binary could not be executed.
var ErrYTDLPMissing = errors.New("yt-dlp is not installed or not on PATH; install it from https://github.com/yt-dlp/yt-dlp")

// Resolver wraps the yt-dlp binary to resolve a query or URL into a streamable
// audio URL plus metadata.
type Resolver struct {
	Binary string
}

// NewResolver constructs a Resolver. An empty binary defaults to "yt-dlp".
func NewResolver(binary string) *Resolver {
	if binary == "" {
		binary = "yt-dlp"
	}
	return &Resolver{Binary: binary}
}

// Resolve returns a direct audio stream URL, title and duration for a query.
// Plain search terms are resolved via ytsearch1:. The supplied context bounds
// the subprocess lifetime.
func (r *Resolver) Resolve(ctx context.Context, query string) (*Track, error) {
	input := query
	if !strings.HasPrefix(query, "http://") && !strings.HasPrefix(query, "https://") {
		input = "ytsearch1:" + query
	}

	// Print URL, title and duration on separate lines via the print template.
	cmd := exec.CommandContext(ctx, r.Binary,
		"--no-playlist",
		"--quiet",
		"--no-warnings",
		"--format", "bestaudio/best",
		"--print", "%(urls)s\n%(title)s\n%(duration)s",
		input,
	)
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, ErrYTDLPMissing
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("yt-dlp failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("yt-dlp: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 1 || strings.TrimSpace(lines[0]) == "" {
		return nil, errors.New("yt-dlp returned no stream URL for the query")
	}

	t := &Track{URL: strings.Fields(lines[0])[0]}
	if len(lines) >= 2 {
		t.Title = strings.TrimSpace(lines[1])
	}
	if t.Title == "" {
		t.Title = query
	}
	if len(lines) >= 3 {
		if d, err := strconv.ParseFloat(strings.TrimSpace(lines[2]), 64); err == nil {
			t.Duration = int(d)
		}
	}
	return t, nil
}

// Available reports whether the yt-dlp binary can be located.
func (r *Resolver) Available() bool {
	_, err := exec.LookPath(r.Binary)
	return err == nil
}
