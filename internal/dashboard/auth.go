package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/0xSalik/specter/internal/db/queries"
)

const sessionCookie = "specter_session"

// discordUser is the subset of /users/@me we consume.
type discordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// partialGuild is the subset of /users/@me/guilds we consume.
type partialGuild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Owner       bool   `json:"owner"`
	Permissions string `json:"permissions"`
}

// permManageGuild is the MANAGE_GUILD permission bit.
const permManageGuild = 0x20

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	state := randomToken()
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: state, Path: "/", HttpOnly: true, MaxAge: 600, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, s.oauth.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	token, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		log.Error().Err(err).Msg("dashboard: oauth exchange")
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	user, err := s.fetchUser(ctx, token.AccessToken)
	if err != nil {
		http.Error(w, "failed to load user profile", http.StatusBadGateway)
		return
	}

	sessionToken := randomToken()
	var avatar *string
	if user.Avatar != "" {
		avatar = &user.Avatar
	}
	sess := queries.Session{
		Token:       sessionToken,
		UserID:      user.ID,
		Username:    user.Username,
		Avatar:      avatar,
		AccessToken: token.AccessToken,
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}
	if err := s.store.CreateSession(ctx, sess); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: sessionToken, Path: "/", HttpOnly: true,
		MaxAge: 7 * 24 * 3600, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_ = s.store.DeleteSession(ctx, c.Value)
		s.guildCacheMu.Lock()
		delete(s.guildCache, c.Value)
		s.guildCacheMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) fetchUser(ctx context.Context, accessToken string) (*discordUser, error) {
	body, err := s.discordGet(ctx, "https://discord.com/api/v10/users/@me", accessToken)
	if err != nil {
		return nil, err
	}
	var u discordUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Server) fetchManageableGuilds(ctx context.Context, accessToken string) (map[string]string, error) {
	body, err := s.discordGet(ctx, "https://discord.com/api/v10/users/@me/guilds", accessToken)
	if err != nil {
		return nil, err
	}
	var guilds []partialGuild
	if err := json.Unmarshal(body, &guilds); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, g := range guilds {
		perms := parsePerms(g.Permissions)
		if g.Owner || perms&permManageGuild != 0 {
			out[g.ID] = g.Name
		}
	}
	return out, nil
}

func (s *Server) discordGet(ctx context.Context, url, accessToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errStatus(resp.StatusCode)
	}
	return readAll(resp.Body)
}
