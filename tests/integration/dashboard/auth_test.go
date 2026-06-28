package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/internal/config"
	"github.com/0xSalik/specter/internal/dashboard"
	"github.com/0xSalik/specter/internal/db/queries"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	session, err := discordgo.New("Bot test-token")
	require.NoError(t, err)
	cfg := &config.Config{DashboardPort: 0, DiscordClientID: "id", DiscordRedirectURI: "http://localhost/auth/callback"}
	srv, err := dashboard.New(cfg, queries.New(nil), session)
	require.NoError(t, err)
	return srv.Routes()
}

func TestUnauthenticatedDashboardRedirects(t *testing.T) {
	handler := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(ts.URL + "/dashboard")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))
}

func TestHealthEndpoint(t *testing.T) {
	handler := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLoginRedirectsToDiscord(t *testing.T) {
	handler := newTestServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(ts.URL + "/login")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Location"), "discord.com/api/oauth2/authorize")
}
