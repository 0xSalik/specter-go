// Package invites tracks per-guild invite usage so member-join events can be
// attributed to the invite (and inviter) that was used, and snapshots members
// so member-leave events can report tenure and roles after the gateway has
// already dropped that data. All state is in-memory and concurrency-safe.
package invites

import (
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// inviteSnap is a point-in-time record of a single guild invite.
type inviteSnap struct {
	Code      string
	Uses      int
	MaxUses   int
	Temporary bool
	ChannelID string
	Inviter   *discordgo.User
	CreatedAt time.Time
}

// MemberSnap captures the details we need to enrich a leave event, since the
// GuildMemberRemove gateway payload only carries the bare user.
type MemberSnap struct {
	JoinedAt time.Time
	Roles    []string
	Username string
}

// Used describes the invite resolved as responsible for a member join.
type Used struct {
	Code      string
	Uses      int
	MaxUses   int
	Temporary bool
	ChannelID string
	Inviter   *discordgo.User
	Vanity    bool
}

// inviteLister is the subset of *discordgo.Session the tracker needs. It keeps
// the package unit-testable without a live session.
type inviteLister interface {
	GuildInvites(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Invite, error)
}

// Tracker maintains invite-usage and member snapshots per guild.
type Tracker struct {
	mu      sync.Mutex
	invites map[string]map[string]inviteSnap // guildID -> code -> snapshot
	members map[string]map[string]MemberSnap // guildID -> userID -> snapshot
}

// New constructs an empty Tracker.
func New() *Tracker {
	return &Tracker{
		invites: make(map[string]map[string]inviteSnap),
		members: make(map[string]map[string]MemberSnap),
	}
}

// Prime fetches the current invites for a guild and stores them as the baseline
// against which future joins are diffed. Safe to call repeatedly (e.g. on every
// GuildCreate / reconnect). Returns the underlying error so callers can decide
// whether the bot lacks the Manage Server permission.
func (t *Tracker) Prime(s inviteLister, guildID string) error {
	list, err := s.GuildInvites(guildID)
	if err != nil {
		return err
	}
	snap := make(map[string]inviteSnap, len(list))
	for _, inv := range list {
		snap[inv.Code] = toSnap(inv)
	}
	t.mu.Lock()
	t.invites[guildID] = snap
	t.mu.Unlock()
	return nil
}

// ResolveJoin fetches the current invites and diffs them against the stored
// baseline to determine which invite was used for the join that just occurred.
// It updates the baseline as a side effect. A nil Used (with nil error) means
// the inviter could not be determined (vanity URL, server discovery, a single
// bot-added member, or the guild was not primed yet). A non-nil error means the
// invite list could not be fetched (typically a missing-permission situation).
func (t *Tracker) ResolveJoin(s inviteLister, guildID string) (*Used, error) {
	list, err := s.GuildInvites(guildID)
	if err != nil {
		return nil, err
	}

	current := make(map[string]inviteSnap, len(list))
	for _, inv := range list {
		current[inv.Code] = toSnap(inv)
	}

	t.mu.Lock()
	prev := t.invites[guildID]
	t.invites[guildID] = current
	primed := prev != nil
	t.mu.Unlock()

	if !primed {
		// No baseline to diff against; we just established one.
		return nil, nil
	}

	var used *Used
	// Case 1: an existing invite's use count incremented.
	for code, cur := range current {
		if old, ok := prev[code]; ok && cur.Uses > old.Uses {
			used = toUsed(cur)
			break
		}
		if _, ok := prev[code]; !ok && cur.Uses > 0 {
			// A brand-new invite that already has a use is the one used.
			used = toUsed(cur)
			break
		}
	}
	// Case 2: a single-use invite was consumed and therefore deleted by Discord.
	if used == nil {
		var disappeared []inviteSnap
		for code, old := range prev {
			if _, ok := current[code]; !ok {
				disappeared = append(disappeared, old)
			}
		}
		if len(disappeared) == 1 {
			used = toUsed(disappeared[0])
			used.Uses = disappeared[0].Uses + 1
		}
	}
	return used, nil
}

// AddInvite records a newly created invite (InviteCreate gateway event).
func (t *Tracker) AddInvite(guildID, code string, uses, maxUses int, temporary bool, channelID string, inviter *discordgo.User) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.invites[guildID] == nil {
		t.invites[guildID] = make(map[string]inviteSnap)
	}
	t.invites[guildID][code] = inviteSnap{
		Code: code, Uses: uses, MaxUses: maxUses,
		Temporary: temporary, ChannelID: channelID, Inviter: inviter,
		CreatedAt: time.Now(),
	}
}

// RemoveInvite drops an invite from the baseline (InviteDelete gateway event).
func (t *Tracker) RemoveInvite(guildID, code string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if m := t.invites[guildID]; m != nil {
		delete(m, code)
	}
}

// SnapshotMember stores the join time, roles, and username of a member so a
// later leave can report how long they were in the server.
func (t *Tracker) SnapshotMember(guildID string, m *discordgo.Member) {
	if m == nil || m.User == nil {
		return
	}
	roles := make([]string, len(m.Roles))
	copy(roles, m.Roles)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.members[guildID] == nil {
		t.members[guildID] = make(map[string]MemberSnap)
	}
	t.members[guildID][m.User.ID] = MemberSnap{
		JoinedAt: m.JoinedAt,
		Roles:    roles,
		Username: m.User.Username,
	}
}

// SnapshotGuild bulk-snapshots a guild's known members (e.g. from GuildCreate).
func (t *Tracker) SnapshotGuild(guildID string, members []*discordgo.Member) {
	for _, m := range members {
		t.SnapshotMember(guildID, m)
	}
}

// PopMember returns and removes a member's snapshot.
func (t *Tracker) PopMember(guildID, userID string) (MemberSnap, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.members[guildID]
	if m == nil {
		return MemberSnap{}, false
	}
	snap, ok := m[userID]
	if ok {
		delete(m, userID)
	}
	return snap, ok
}

// Forget drops all cached state for a guild (e.g. on guild removal).
func (t *Tracker) Forget(guildID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.invites, guildID)
	delete(t.members, guildID)
}

func toSnap(inv *discordgo.Invite) inviteSnap {
	s := inviteSnap{
		Code:      inv.Code,
		Uses:      inv.Uses,
		MaxUses:   inv.MaxUses,
		Temporary: inv.Temporary,
		Inviter:   inv.Inviter,
		CreatedAt: inv.CreatedAt,
	}
	if inv.Channel != nil {
		s.ChannelID = inv.Channel.ID
	}
	return s
}

func toUsed(s inviteSnap) *Used {
	return &Used{
		Code:      s.Code,
		Uses:      s.Uses,
		MaxUses:   s.MaxUses,
		Temporary: s.Temporary,
		ChannelID: s.ChannelID,
		Inviter:   s.Inviter,
	}
}
