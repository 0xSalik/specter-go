package dashboard

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/salik/specter/internal/db/queries"
)

type pageData struct {
	Title    string
	Username string
	GuildID  string
	Guild    *discordgo.Guild
	Data     map[string]any
}

func (s *Server) base(r *http.Request, title string) pageData {
	pd := pageData{Title: title, Data: map[string]any{}}
	if sess := sessionFrom(r); sess != nil {
		pd.Username = sess.Username
	}
	if gid := guildIDFrom(r); gid != "" {
		pd.GuildID = gid
		if g, err := s.session.State.Guild(gid); err == nil {
			pd.Guild = g
		}
	}
	return pd
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
		return
	}
	s.render(w, "login.html", pageData{Title: "Specter Dashboard"})
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)
	manageable := s.manageableGuilds(r.Context(), sess)

	type guildItem struct {
		ID, Name string
		HasBot   bool
	}
	var items []guildItem
	for id, name := range manageable {
		_, err := s.session.State.Guild(id)
		items = append(items, guildItem{ID: id, Name: name, HasBot: err == nil})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].HasBot != items[j].HasBot {
			return items[i].HasBot
		}
		return items[i].Name < items[j].Name
	})

	pd := s.base(r, "Your Servers")
	pd.Data["Guilds"] = items
	s.render(w, "home.html", pd)
}

func (s *Server) handleGuildOverview(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	stats, err := s.store.GuildStats(ctx, gid)
	if err != nil {
		http.Error(w, "failed to load stats", http.StatusInternalServerError)
		return
	}
	pd := s.base(r, "Overview")
	pd.Data["Stats"] = stats
	s.render(w, "guild.html", pd)
}

// ---- Levels ----

func (s *Server) handleLevelsPage(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	cfg, err := s.store.GetLevelConfig(ctx, gid)
	if err != nil {
		http.Error(w, "failed to load level config", http.StatusInternalServerError)
		return
	}
	pd := s.base(r, "Level Settings")
	pd.Data["Config"] = cfg
	pd.Data["Channels"] = s.textChannels(gid)
	s.render(w, "levels.html", pd)
}

func (s *Server) handleLevelsSave(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cfg, err := s.store.GetLevelConfig(ctx, gid)
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}
	cfg.Enabled = r.FormValue("enabled") == "on"
	cfg.XPMin = atoiDefault(r.FormValue("xp_min"), cfg.XPMin)
	cfg.XPMax = atoiDefault(r.FormValue("xp_max"), cfg.XPMax)
	cfg.XPCooldownSecs = atoiDefault(r.FormValue("cooldown"), cfg.XPCooldownSecs)
	if ch := r.FormValue("announce_channel"); ch != "" {
		cfg.AnnounceChannelID = &ch
	} else {
		cfg.AnnounceChannelID = nil
	}
	if msg := r.FormValue("announce_msg"); msg != "" {
		cfg.AnnounceMsg = &msg
	}
	cfg.NoXPRoles = splitCSV(r.FormValue("no_xp_roles"))
	cfg.NoXPChannels = splitCSV(r.FormValue("no_xp_channels"))

	if cfg.XPMax < cfg.XPMin {
		http.Error(w, "xp_max cannot be less than xp_min", http.StatusBadRequest)
		return
	}
	if err := s.store.UpsertLevelConfig(ctx, cfg); err != nil {
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}
	s.audit(ctx, r, "levels.update", nil, map[string]any{"xp_min": cfg.XPMin, "xp_max": cfg.XPMax})
	http.Redirect(w, r, "/dashboard/"+gid+"/levels", http.StatusSeeOther)
}

// ---- Automod ----

func (s *Server) handleAutomodPage(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	cfg, err := s.store.GetAutomodConfig(ctx, gid)
	if err != nil {
		http.Error(w, "failed to load automod config", http.StatusInternalServerError)
		return
	}
	pd := s.base(r, "Automod Settings")
	pd.Data["Config"] = cfg
	pd.Data["BadWords"] = strings.Join(cfg.BadWords, "\n")
	s.render(w, "automod.html", pd)
}

func (s *Server) handleAutomodSave(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	cfg, err := s.store.GetAutomodConfig(ctx, gid)
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}
	cfg.Enabled = r.FormValue("enabled") == "on"
	cfg.AntiSpamEnabled = r.FormValue("anti_spam") == "on"
	cfg.AntiInviteEnabled = r.FormValue("anti_invite") == "on"
	cfg.AntiLinkEnabled = r.FormValue("anti_link") == "on"
	cfg.AntiCapsEnabled = r.FormValue("anti_caps") == "on"
	cfg.BadWordsEnabled = r.FormValue("bad_words_enabled") == "on"
	cfg.AntiSpamThreshold = atoiDefault(r.FormValue("spam_threshold"), cfg.AntiSpamThreshold)
	cfg.AntiSpamWindowSecs = atoiDefault(r.FormValue("spam_window"), cfg.AntiSpamWindowSecs)
	cfg.CapsThresholdPct = atoiDefault(r.FormValue("caps_threshold"), cfg.CapsThresholdPct)
	cfg.BadWords = splitLines(r.FormValue("bad_words"))
	cfg.AllowedLinkDomains = splitCSV(r.FormValue("allowed_domains"))
	cfg.ExemptRoles = splitCSV(r.FormValue("exempt_roles"))
	cfg.ExemptChannels = splitCSV(r.FormValue("exempt_channels"))
	if a := r.FormValue("action"); a != "" {
		cfg.Action = a
	}

	if err := s.store.UpsertAutomodConfig(ctx, cfg); err != nil {
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}
	s.audit(ctx, r, "automod.update", nil, map[string]any{"enabled": cfg.Enabled})
	http.Redirect(w, r, "/dashboard/"+gid+"/automod", http.StatusSeeOther)
}

// ---- Mod logs ----

func (s *Server) handleModlogsPage(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	overrides, _ := s.store.ListOverrides(ctx, gid)
	byType := make(map[string]queries.ModlogOverride, len(overrides))
	for _, o := range overrides {
		byType[o.EventType] = o
	}
	pd := s.base(r, "Mod Log Settings")
	pd.Data["EventTypes"] = modlogEventTypes
	pd.Data["Overrides"] = byType
	pd.Data["Channels"] = s.textChannels(gid)
	s.render(w, "modlogs.html", pd)
}

var modlogEventTypes = []string{
	"message_delete", "message_edit", "member_join", "member_leave", "member_update",
	"ban", "unban", "kick", "warn", "channel_update", "guild_update",
}

func (s *Server) handleModlogsSave(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	for _, et := range modlogEventTypes {
		o := queries.ModlogOverride{GuildID: gid, EventType: et, Enabled: r.FormValue("enabled_"+et) == "on"}
		if ch := r.FormValue("channel_" + et); ch != "" {
			o.ChannelID = &ch
		}
		if err := s.store.SetOverride(ctx, o); err != nil {
			http.Error(w, "failed to save", http.StatusInternalServerError)
			return
		}
	}
	s.audit(ctx, r, "modlogs.update", nil, nil)
	http.Redirect(w, r, "/dashboard/"+gid+"/modlogs", http.StatusSeeOther)
}

// ---- Access control ----

func (s *Server) handleAccessPage(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rules, _ := s.store.ListAllAccessRules(ctx, gid)
	pd := s.base(r, "Access Control")
	pd.Data["Rules"] = rules
	pd.Data["Groups"] = []string{"moderation", "fun", "music", "levels", "user", "voice", "reactionroles", "system"}
	s.render(w, "access.html", pd)
}

func (s *Server) handleAccessSave(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	action := r.FormValue("op")
	rule := queries.AccessRule{
		GuildID:      gid,
		CommandGroup: r.FormValue("command_group"),
		EntityType:   r.FormValue("entity_type"),
		EntityID:     strings.TrimSpace(r.FormValue("entity_id")),
		Allowed:      r.FormValue("allowed") == "allow",
	}
	if rule.CommandGroup == "" || rule.EntityID == "" {
		http.Error(w, "command group and entity ID are required", http.StatusBadRequest)
		return
	}
	var err error
	if action == "delete" {
		err = s.store.DeleteAccessRule(ctx, gid, rule.CommandGroup, rule.EntityType, rule.EntityID)
	} else {
		err = s.store.SetAccessRule(ctx, rule)
	}
	if err != nil {
		http.Error(w, "failed to save rule", http.StatusInternalServerError)
		return
	}
	s.audit(ctx, r, "access."+action, &rule.EntityID, rule)
	http.Redirect(w, r, "/dashboard/"+gid+"/access", http.StatusSeeOther)
}

// ---- Rapsheets ----

func (s *Server) handleRapsheetsPage(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	pd := s.base(r, "Rapsheets")
	pd.Data["UserID"] = userID
	if userID != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		actions, _ := s.store.ListActions(ctx, gid, userID, 100, 0)
		pd.Data["Actions"] = actions
	}
	s.render(w, "rapsheets.html", pd)
}

func (s *Server) handleRapsheetClear(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_ = s.store.ClearActions(ctx, gid, userID)
	_ = s.store.ClearWarnings(ctx, gid, userID)
	s.audit(ctx, r, "rapsheet.clear", &userID, nil)
	http.Redirect(w, r, "/dashboard/"+gid+"/rapsheets", http.StatusSeeOther)
}

// ---- Reaction roles ----

func (s *Server) handleReactionRolesPage(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	menus, _ := s.store.ListMenus(ctx, gid)
	type menuRow struct {
		queries.ReactionRoleMenu
		Entries int
	}
	var rows []menuRow
	for _, m := range menus {
		count, _ := s.store.CountEntries(ctx, m.ID)
		rows = append(rows, menuRow{ReactionRoleMenu: m, Entries: count})
	}
	pd := s.base(r, "Reaction Roles")
	pd.Data["Menus"] = rows
	s.render(w, "reactionroles.html", pd)
}

// ---- Music ----

func (s *Server) handleMusicPage(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	queue, _ := s.store.LoadQueue(ctx, gid)
	pd := s.base(r, "Music")
	pd.Data["Queue"] = queue
	s.render(w, "music.html", pd)
}

func (s *Server) handleMusicClear(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_ = s.store.ClearQueue(ctx, gid)
	s.audit(ctx, r, "music.clear_queue", nil, nil)
	http.Redirect(w, r, "/dashboard/"+gid+"/music", http.StatusSeeOther)
}

// ---- Audit ----

func (s *Server) handleAuditPage(w http.ResponseWriter, r *http.Request) {
	gid := guildIDFrom(r)
	page := atoiDefault(r.URL.Query().Get("page"), 0)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	const pageSize = 25
	entries, _ := s.store.ListAudit(ctx, gid, pageSize, page*pageSize)
	total, _ := s.store.CountAudit(ctx, gid)
	pd := s.base(r, "Audit Log")
	pd.Data["Entries"] = entries
	pd.Data["Page"] = page
	pd.Data["HasNext"] = (page+1)*pageSize < total
	pd.Data["PrevPage"] = page - 1
	pd.Data["NextPage"] = page + 1
	s.render(w, "audit.html", pd)
}

// ---- helpers ----

func (s *Server) audit(ctx context.Context, r *http.Request, action string, target *string, detail any) {
	sess := sessionFrom(r)
	gid := guildIDFrom(r)
	if sess == nil || gid == "" {
		return
	}
	_ = s.store.WriteAudit(ctx, gid, sess.UserID, action, target, detail)
}

func (s *Server) textChannels(guildID string) []*discordgo.Channel {
	g, err := s.session.State.Guild(guildID)
	if err != nil || g == nil {
		return nil
	}
	var out []*discordgo.Channel
	for _, ch := range g.Channels {
		if ch.Type == discordgo.ChannelTypeGuildText {
			out = append(out, ch)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
