package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/0xSalik/specter/internal/db/queries"
)

type ctxKey string

const (
	ctxSession ctxKey = "session"
	ctxGuildID ctxKey = "guildID"
)

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		sess, err := s.store.GetSession(ctx, c.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxSession, sess))
		next.ServeHTTP(w, r)
	})
}

// requireGuildAdmin ensures the user manages the requested guild AND Specter is
// present in it. It never exposes data from guilds the user does not administer.
func (s *Server) requireGuildAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFrom(r)
		if sess == nil {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}
		guildID := chi.URLParam(r, "guildID")

		manageable := s.manageableGuilds(r.Context(), sess)
		if _, ok := manageable[guildID]; !ok {
			http.Error(w, "You do not have permission to manage this server.", http.StatusForbidden)
			return
		}
		// Bot presence check.
		if g, err := s.session.State.Guild(guildID); err != nil || g == nil {
			http.Error(w, "Specter is not a member of this server.", http.StatusForbidden)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), ctxGuildID, guildID))
		next.ServeHTTP(w, r)
	})
}

// manageableGuilds returns the user's manageable guilds, cached for 60 seconds.
func (s *Server) manageableGuilds(parent context.Context, sess *queries.Session) map[string]string {
	s.guildCacheMu.Lock()
	if c, ok := s.guildCache[sess.Token]; ok && time.Now().Before(c.expires) {
		s.guildCacheMu.Unlock()
		return c.guilds
	}
	s.guildCacheMu.Unlock()

	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	guilds, err := s.fetchManageableGuilds(ctx, sess.AccessToken)
	if err != nil {
		return map[string]string{}
	}
	s.guildCacheMu.Lock()
	s.guildCache[sess.Token] = cachedGuilds{guilds: guilds, expires: time.Now().Add(60 * time.Second)}
	s.guildCacheMu.Unlock()
	return guilds
}

func sessionFrom(r *http.Request) *queries.Session {
	v := r.Context().Value(ctxSession)
	if v == nil {
		return nil
	}
	sess, _ := v.(*queries.Session)
	return sess
}

func guildIDFrom(r *http.Request) string {
	v := r.Context().Value(ctxGuildID)
	if v == nil {
		return ""
	}
	id, _ := v.(string)
	return id
}
