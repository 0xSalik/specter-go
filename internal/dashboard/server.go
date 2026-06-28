// Package dashboard serves the Specter web dashboard: a server-rendered
// HTMX + Tailwind UI authenticated via Discord OAuth2 and backed by the same
// PostgreSQL store as the bot.
package dashboard

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

	"github.com/0xSalik/specter/internal/config"
	"github.com/0xSalik/specter/internal/db/queries"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Server is the dashboard HTTP server.
type Server struct {
	cfg     *config.Config
	store   *queries.Store
	session *discordgo.Session
	tmpl    *template.Template
	oauth   *oauth2.Config
	http    *http.Server

	guildCacheMu sync.Mutex
	guildCache   map[string]cachedGuilds // session token -> manageable guilds
}

type cachedGuilds struct {
	guilds  map[string]string // guildID -> name
	expires time.Time
}

// New constructs a dashboard Server.
func New(cfg *config.Config, store *queries.Store, session *discordgo.Session) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"deref": func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		},
		"join": func(items []string) string {
			out := ""
			for i, it := range items {
				if i > 0 {
					out += ", "
				}
				out += it
			}
			return out
		},
		"isin": func(items []string, v string) bool {
			for _, it := range items {
				if it == v {
					return true
				}
			}
			return false
		},
	}).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	s := &Server{
		cfg:        cfg,
		store:      store,
		session:    session,
		tmpl:       tmpl,
		guildCache: make(map[string]cachedGuilds),
		oauth: &oauth2.Config{
			ClientID:     cfg.DiscordClientID,
			ClientSecret: cfg.DiscordClientSecret,
			RedirectURL:  cfg.DiscordRedirectURI,
			Scopes:       []string{"identify", "guilds"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://discord.com/api/oauth2/authorize",
				TokenURL: "https://discord.com/api/oauth2/token",
			},
		},
	}
	return s, nil
}

// Routes builds the chi router.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Get("/", s.handleIndex)
	r.Get("/login", s.handleLogin)
	r.Get("/auth/callback", s.handleCallback)
	r.Get("/logout", s.handleLogout)

	r.Route("/dashboard", func(r chi.Router) {
		r.Use(s.requireSession)
		r.Get("/", s.handleHome)
		r.Route("/{guildID}", func(r chi.Router) {
			r.Use(s.requireGuildAdmin)
			r.Get("/", s.handleGuildOverview)
			r.Get("/levels", s.handleLevelsPage)
			r.Post("/levels", s.handleLevelsSave)
			r.Get("/automod", s.handleAutomodPage)
			r.Post("/automod", s.handleAutomodSave)
			r.Get("/welcome", s.handleWelcomePage)
			r.Post("/welcome", s.handleWelcomeSave)
			r.Get("/autorole", s.handleAutorolePage)
			r.Post("/autorole", s.handleAutoroleSave)
			r.Get("/levelroles", s.handleLevelRolesPage)
			r.Post("/levelroles", s.handleLevelRolesSave)
			r.Get("/starboard", s.handleStarboardPage)
			r.Post("/starboard", s.handleStarboardSave)
			r.Get("/modnotify", s.handleModNotifyPage)
			r.Post("/modnotify", s.handleModNotifySave)
			r.Get("/modlogs", s.handleModlogsPage)
			r.Post("/modlogs", s.handleModlogsSave)
			r.Get("/access", s.handleAccessPage)
			r.Post("/access", s.handleAccessSave)
			r.Get("/rapsheets", s.handleRapsheetsPage)
			r.Post("/rapsheets/clear", s.handleRapsheetClear)
			r.Get("/reactionroles", s.handleReactionRolesPage)
			r.Get("/music", s.handleMusicPage)
			r.Post("/music/clear", s.handleMusicClear)
			r.Get("/audit", s.handleAuditPage)
		})
	})

	return r
}

// Start runs the HTTP server (blocking). Call in a goroutine.
func (s *Server) Start() error {
	s.http = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.cfg.DashboardPort),
		Handler:           s.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Info().Int("port", s.cfg.DashboardPort).Msg("dashboard listening")
	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) {
	if s.http != nil {
		_ = s.http.Shutdown(ctx)
	}
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Error().Err(err).Str("template", name).Msg("dashboard: render")
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
