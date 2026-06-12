// Package httpx provides small, typed HTTP helpers with mandatory timeouts so
// no external call can hang a command handler.
package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// UserAgent identifies the bot to external APIs.
const UserAgent = "Specter/1.0 (+https://github.com/salik/specter)"

// GetJSON fetches url and decodes the JSON body into out.
func GetJSON(ctx context.Context, url string, out any) error {
	body, err := Get(ctx, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// Get fetches url and returns the raw body, enforcing a non-2xx error.
func Get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request to %s returned status %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}
