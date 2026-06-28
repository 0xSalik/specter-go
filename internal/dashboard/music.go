package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/0xSalik/specter/internal/db/queries"
	"github.com/0xSalik/specter/internal/music"
)

// ---- Music player page ----

func (s *Server) handleMusicPlayerPage(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	sess := sessionFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	canControl, isAdmin, dj := s.musicPerms(ctx, sess, gid)

	pd := s.base(r, "Music Player")
	pd.Data["CanControl"] = canControl
	pd.Data["IsAdmin"] = isAdmin
	pd.Data["DJRoleID"] = deref(dj)
	pd.Data["Roles"] = s.roles(gid)
	s.render(w, "musicplayer.html", pd)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ---- Music player JSON API ----

type trackJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Source    string `json:"source"`
	URL       string `json:"url"`
	Artwork   string `json:"artwork"`
	Requester string `json:"requester"`
	Duration  int64  `json:"duration"` // milliseconds; 0 for streams
	IsStream  bool   `json:"isStream"`
}

type stateJSON struct {
	Ready      bool        `json:"ready"`
	CanControl bool        `json:"canControl"`
	IsAdmin    bool        `json:"isAdmin"`
	DJRoleID   string      `json:"djRoleId"`
	State      string      `json:"state"` // playing | paused | idle
	Volume     int         `json:"volume"`
	Position   int64       `json:"position"` // milliseconds into current track
	Current    *trackJSON  `json:"current"`
	Queue      []trackJSON `json:"queue"`
}

func (s *Server) handleMusicState(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	sess := sessionFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, s.buildState(ctx, sess, gid))
}

func (s *Server) handleMusicControl(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	sess := sessionFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	canControl, _, _ := s.musicPerms(ctx, sess, gid)
	if !canControl {
		writeJSONError(w, http.StatusForbidden, "You need the DJ role to control the player.")
		return
	}
	if s.music == nil || !s.music.Ready() {
		writeJSONError(w, http.StatusServiceUnavailable, "Music backend is not connected.")
		return
	}

	action := r.FormValue("action")
	switch action {
	case "play", "playnext":
		query := strings.TrimSpace(r.FormValue("query"))
		if query == "" {
			writeJSONError(w, http.StatusBadRequest, "Enter a song name or link.")
			return
		}
		vc := s.userVoiceChannel(gid, sess.UserID)
		if vc == "" {
			writeJSONError(w, http.StatusBadRequest, "Join a voice channel first.")
			return
		}
		res, err := s.music.Load(ctx, query)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if _, _, err := s.music.Add(ctx, gid, vc, sess.UserID, res.Tracks, action == "playnext"); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.audit(ctx, r, "music.add", &query, map[string]any{"next": action == "playnext", "count": len(res.Tracks)})

	case "pause", "resume", "toggle":
		gp, ok := s.music.Get(gid)
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "Nothing is playing.")
			return
		}
		var err error
		switch {
		case action == "pause" || (action == "toggle" && gp.State() == music.StatePlaying):
			_, err = gp.Pause(ctx)
		default:
			_, err = gp.Resume(ctx)
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

	case "skip":
		gp, ok := s.music.Get(gid)
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "Nothing is playing.")
			return
		}
		if _, err := gp.Skip(ctx); err != nil && err != music.ErrNoVoice {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

	case "stop", "leave":
		if err := s.music.Leave(gid); err != nil && err != music.ErrNoVoice {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

	case "volume":
		gp, ok := s.music.Get(gid)
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "Nothing is playing.")
			return
		}
		v, _ := strconv.Atoi(r.FormValue("value"))
		if err := gp.SetVolume(ctx, v); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

	case "remove":
		gp, ok := s.music.Get(gid)
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "Nothing is playing.")
			return
		}
		if _, ok := gp.RemoveByID(r.FormValue("id")); !ok {
			writeJSONError(w, http.StatusBadRequest, "That track is no longer in the queue.")
			return
		}

	case "move":
		gp, ok := s.music.Get(gid)
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "Nothing is playing.")
			return
		}
		to, _ := strconv.Atoi(r.FormValue("to"))
		if !gp.MoveByID(r.FormValue("id"), to) {
			writeJSONError(w, http.StatusBadRequest, "That track is no longer in the queue.")
			return
		}

	default:
		writeJSONError(w, http.StatusBadRequest, "Unknown action.")
		return
	}

	writeJSON(w, http.StatusOK, s.buildState(ctx, sess, gid))
}

func (s *Server) handleSetDJRole(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	sess := sessionFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if !s.isGuildManager(ctx, sess, gid) {
		http.Error(w, "Only members who can Manage Server may set the DJ role.", http.StatusForbidden)
		return
	}
	role := optPtr(r.FormValue("role_id"))
	if role != nil && (*role == "none" || *role == "") {
		role = nil
	}
	if err := s.store.SetDJRole(ctx, gid, role); err != nil {
		http.Error(w, "failed to save DJ role", http.StatusInternalServerError)
		return
	}
	s.audit(ctx, r, "music.set_dj_role", role, nil)
	http.Redirect(w, r, "/music-player/"+gid, http.StatusSeeOther)
}

// buildState assembles the live player snapshot plus the caller's permissions.
func (s *Server) buildState(ctx context.Context, sess *queries.Session, guildID string) stateJSON {
	canControl, isAdmin, dj := s.musicPerms(ctx, sess, guildID)
	st := stateJSON{
		CanControl: canControl,
		IsAdmin:    isAdmin,
		DJRoleID:   deref(dj),
		State:      "idle",
		Volume:     100,
		Queue:      []trackJSON{},
	}
	if s.music == nil {
		return st
	}
	st.Ready = s.music.Ready()
	gp, ok := s.music.Get(guildID)
	if !ok {
		return st
	}
	st.State = strings.ToLower(gp.State().String())
	st.Volume = gp.Volume()
	if cur, pos, ok := gp.Current(); ok {
		tj := s.trackToJSON(guildID, cur)
		st.Current = &tj
		st.Position = pos.Milliseconds()
	}
	for _, t := range gp.QueueList() {
		st.Queue = append(st.Queue, s.trackToJSON(guildID, t))
	}
	return st
}

func (s *Server) trackToJSON(guildID string, t music.Track) trackJSON {
	return trackJSON{
		ID:        t.QID,
		Title:     t.Title(),
		Author:    t.Author(),
		Source:    t.Source(),
		URL:       t.URL(),
		Artwork:   t.Artwork(),
		Requester: s.displayName(guildID, t.Requester),
		Duration:  t.Duration().Milliseconds(),
		IsStream:  t.IsStream(),
	}
}

// ---- Permission + lookup helpers ----

// musicPerms resolves whether the caller can mutate the player. Manage Server
// members are always allowed (and flagged as admins). When a DJ role is set,
// only members holding it may control playback; when none is set, anyone
// currently in a voice channel may.
func (s *Server) musicPerms(ctx context.Context, sess *queries.Session, guildID string) (canControl, isAdmin bool, djRole *string) {
	if sess == nil {
		return false, false, nil
	}
	admin := s.isGuildManager(ctx, sess, guildID)
	if cfg, err := s.store.GetGuild(ctx, guildID); err == nil && cfg != nil {
		djRole = cfg.DJRoleID
	}
	if admin {
		return true, true, djRole
	}
	if djRole != nil && *djRole != "" {
		if m, ok := s.resolveMember(guildID, sess.UserID); ok {
			for _, rid := range m.Roles {
				if rid == *djRole {
					return true, false, djRole
				}
			}
		}
		return false, false, djRole
	}
	return s.userVoiceChannel(guildID, sess.UserID) != "", false, djRole
}

func (s *Server) isGuildManager(ctx context.Context, sess *queries.Session, guildID string) bool {
	if sess == nil {
		return false
	}
	_, ok := s.manageableGuilds(ctx, sess)[guildID]
	return ok
}

// resolveMember fetches a guild member from cache, falling back to the REST API
// and warming the state cache on success.
func (s *Server) resolveMember(guildID, userID string) (*discordgo.Member, bool) {
	if m, err := s.session.State.Member(guildID, userID); err == nil && m != nil {
		return m, true
	}
	m, err := s.session.GuildMember(guildID, userID)
	if err != nil || m == nil {
		return nil, false
	}
	m.GuildID = guildID
	_ = s.session.State.MemberAdd(m)
	return m, true
}

// userVoiceChannel returns the ID of the voice channel the user is currently in
// within the guild, or "" when they are not connected.
func (s *Server) userVoiceChannel(guildID, userID string) string {
	g, err := s.session.State.Guild(guildID)
	if err != nil || g == nil {
		return ""
	}
	for _, vs := range g.VoiceStates {
		if vs.UserID == userID {
			return vs.ChannelID
		}
	}
	return ""
}

// displayName resolves a user ID to a human-friendly name, cached per process.
func (s *Server) displayName(guildID, userID string) string {
	if userID == "" {
		return "Unknown"
	}
	s.nameCacheMu.Lock()
	if n, ok := s.nameCache[userID]; ok {
		s.nameCacheMu.Unlock()
		return n
	}
	s.nameCacheMu.Unlock()

	name := "Unknown"
	if m, ok := s.resolveMember(guildID, userID); ok {
		switch {
		case m.Nick != "":
			name = m.Nick
		case m.User != nil && m.User.GlobalName != "":
			name = m.User.GlobalName
		case m.User != nil:
			name = m.User.Username
		}
	} else if u, err := s.session.User(userID); err == nil && u != nil {
		if u.GlobalName != "" {
			name = u.GlobalName
		} else {
			name = u.Username
		}
	}

	s.nameCacheMu.Lock()
	s.nameCache[userID] = name
	s.nameCacheMu.Unlock()
	return name
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
