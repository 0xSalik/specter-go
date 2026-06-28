package invites_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xSalik/specter/internal/invites"
)

// fakeLister returns a scripted sequence of invite lists on successive calls.
type fakeLister struct {
	calls [][]*discordgo.Invite
	err   error
	idx   int
}

func (f *fakeLister) GuildInvites(_ string, _ ...discordgo.RequestOption) ([]*discordgo.Invite, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.idx >= len(f.calls) {
		return f.calls[len(f.calls)-1], nil
	}
	out := f.calls[f.idx]
	f.idx++
	return out, nil
}

func inv(code string, uses, maxUses int, inviterID string) *discordgo.Invite {
	return &discordgo.Invite{
		Code:    code,
		Uses:    uses,
		MaxUses: maxUses,
		Inviter: &discordgo.User{ID: inviterID, Username: "inviter-" + inviterID},
		Channel: &discordgo.Channel{ID: "chan-" + code},
	}
}

func TestResolveJoin_IncrementedExistingInvite(t *testing.T) {
	f := &fakeLister{calls: [][]*discordgo.Invite{
		{inv("abc", 3, 0, "100"), inv("xyz", 1, 0, "200")}, // prime baseline
		{inv("abc", 4, 0, "100"), inv("xyz", 1, 0, "200")}, // abc incremented
	}}
	tr := invites.New()
	require.NoError(t, tr.Prime(f, "g1"))

	used, err := tr.ResolveJoin(f, "g1")
	require.NoError(t, err)
	require.NotNil(t, used)
	assert.Equal(t, "abc", used.Code)
	assert.Equal(t, 4, used.Uses)
	require.NotNil(t, used.Inviter)
	assert.Equal(t, "100", used.Inviter.ID)
	assert.Equal(t, "chan-abc", used.ChannelID)
}

func TestResolveJoin_BrandNewInviteWithUse(t *testing.T) {
	f := &fakeLister{calls: [][]*discordgo.Invite{
		{inv("abc", 3, 0, "100")},
		{inv("abc", 3, 0, "100"), inv("new", 1, 1, "300")},
	}}
	tr := invites.New()
	require.NoError(t, tr.Prime(f, "g1"))

	used, err := tr.ResolveJoin(f, "g1")
	require.NoError(t, err)
	require.NotNil(t, used)
	assert.Equal(t, "new", used.Code)
	assert.Equal(t, "300", used.Inviter.ID)
}

func TestResolveJoin_SingleUseInviteConsumedAndDeleted(t *testing.T) {
	f := &fakeLister{calls: [][]*discordgo.Invite{
		{inv("once", 0, 1, "400"), inv("perm", 5, 0, "100")},
		{inv("perm", 5, 0, "100")}, // "once" vanished after consumption
	}}
	tr := invites.New()
	require.NoError(t, tr.Prime(f, "g1"))

	used, err := tr.ResolveJoin(f, "g1")
	require.NoError(t, err)
	require.NotNil(t, used)
	assert.Equal(t, "once", used.Code)
	assert.Equal(t, "400", used.Inviter.ID)
	assert.Equal(t, 1, used.Uses, "consumed single-use invite reports the final use")
}

func TestResolveJoin_NotPrimedReturnsNil(t *testing.T) {
	f := &fakeLister{calls: [][]*discordgo.Invite{
		{inv("abc", 4, 0, "100")},
	}}
	tr := invites.New()

	used, err := tr.ResolveJoin(f, "g1")
	require.NoError(t, err)
	assert.Nil(t, used, "without a baseline the inviter is indeterminate")
}

func TestResolveJoin_AmbiguousReturnsNil(t *testing.T) {
	f := &fakeLister{calls: [][]*discordgo.Invite{
		{inv("a", 1, 0, "1"), inv("b", 1, 0, "2")},
		{inv("a", 1, 0, "1"), inv("b", 1, 0, "2")}, // nothing changed
	}}
	tr := invites.New()
	require.NoError(t, tr.Prime(f, "g1"))

	used, err := tr.ResolveJoin(f, "g1")
	require.NoError(t, err)
	assert.Nil(t, used)
}

func TestResolveJoin_FetchErrorPropagates(t *testing.T) {
	f := &fakeLister{err: errors.New("missing access")}
	tr := invites.New()

	used, err := tr.ResolveJoin(f, "g1")
	require.Error(t, err)
	assert.Nil(t, used)
}

func TestInviteCreateDeleteAffectBaseline(t *testing.T) {
	// Prime with one invite, then simulate a gateway InviteCreate for a new
	// single-use invite, then a join that consumes it.
	f := &fakeLister{calls: [][]*discordgo.Invite{
		{inv("perm", 5, 0, "100")}, // prime
		{inv("perm", 6, 0, "100")}, // join used perm
	}}
	tr := invites.New()
	require.NoError(t, tr.Prime(f, "g1"))

	tr.AddInvite("g1", "temp", 0, 1, false, "chan-temp", &discordgo.User{ID: "999"})
	tr.RemoveInvite("g1", "temp") // deleted before use; must not be attributed

	used, err := tr.ResolveJoin(f, "g1")
	require.NoError(t, err)
	require.NotNil(t, used)
	assert.Equal(t, "perm", used.Code)
}

func TestMemberSnapshotAndPop(t *testing.T) {
	tr := invites.New()
	joined := time.Now().Add(-48 * time.Hour)
	tr.SnapshotMember("g1", &discordgo.Member{
		User:     &discordgo.User{ID: "u1", Username: "alice"},
		JoinedAt: joined,
		Roles:    []string{"r1", "r2"},
	})

	snap, ok := tr.PopMember("g1", "u1")
	require.True(t, ok)
	assert.Equal(t, "alice", snap.Username)
	assert.Equal(t, joined, snap.JoinedAt)
	assert.Equal(t, []string{"r1", "r2"}, snap.Roles)

	_, ok = tr.PopMember("g1", "u1")
	assert.False(t, ok, "snapshot is removed after pop")
}
