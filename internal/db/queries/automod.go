package queries

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// RuleScope optionally limits a single automod rule to (or away from) roles.
// Include: if non-empty, the rule only applies to members with one of these
// roles. Exclude: members with any of these roles are never caught by the rule.
type RuleScope struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// AutomodConfig mirrors a row of the automod_config table.
type AutomodConfig struct {
	GuildID            string
	Enabled            bool
	AntiSpamEnabled    bool
	AntiSpamThreshold  int
	AntiSpamWindowSecs int
	AntiInviteEnabled  bool
	AntiLinkEnabled    bool
	AllowedLinkDomains []string
	AntiCapsEnabled    bool
	CapsThresholdPct   int
	BadWordsEnabled    bool
	BadWords           []string
	ExemptRoles        []string
	ExemptChannels     []string
	Action             string
	LogChannelID       *string
	// RuleRoleScopes maps a rule name ("spam","invite","link","caps","badwords")
	// to its per-rule include/exclude role lists. Absent rules apply to everyone.
	RuleRoleScopes map[string]RuleScope
}

// DefaultAutomodConfig returns the zero-value configuration for a guild.
func DefaultAutomodConfig(guildID string) *AutomodConfig {
	return &AutomodConfig{
		GuildID:            guildID,
		AntiSpamThreshold:  5,
		AntiSpamWindowSecs: 5,
		CapsThresholdPct:   70,
		Action:             "delete",
		RuleRoleScopes:     map[string]RuleScope{},
	}
}

// GetAutomodConfig fetches automod configuration, returning defaults if absent.
func (s *Store) GetAutomodConfig(ctx context.Context, guildID string) (*AutomodConfig, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT guild_id, enabled, anti_spam_enabled, anti_spam_threshold, anti_spam_window_secs,
		       anti_invite_enabled, anti_link_enabled, allowed_link_domains,
		       anti_caps_enabled, caps_threshold_pct, bad_words_enabled, bad_words,
		       exempt_roles, exempt_channels, action, log_channel_id, rule_role_scopes
		FROM automod_config WHERE guild_id = $1`, guildID)

	var c AutomodConfig
	var scopesRaw []byte
	err := row.Scan(&c.GuildID, &c.Enabled, &c.AntiSpamEnabled, &c.AntiSpamThreshold, &c.AntiSpamWindowSecs,
		&c.AntiInviteEnabled, &c.AntiLinkEnabled, &c.AllowedLinkDomains,
		&c.AntiCapsEnabled, &c.CapsThresholdPct, &c.BadWordsEnabled, &c.BadWords,
		&c.ExemptRoles, &c.ExemptChannels, &c.Action, &c.LogChannelID, &scopesRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultAutomodConfig(guildID), nil
	}
	if err != nil {
		return nil, err
	}
	c.RuleRoleScopes = map[string]RuleScope{}
	if len(scopesRaw) > 0 {
		_ = json.Unmarshal(scopesRaw, &c.RuleRoleScopes)
	}
	return &c, nil
}

// UpsertAutomodConfig writes the full automod configuration for a guild.
func (s *Store) UpsertAutomodConfig(ctx context.Context, c *AutomodConfig) error {
	scopes := c.RuleRoleScopes
	if scopes == nil {
		scopes = map[string]RuleScope{}
	}
	scopesRaw, err := json.Marshal(scopes)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO automod_config (guild_id, enabled, anti_spam_enabled, anti_spam_threshold,
			anti_spam_window_secs, anti_invite_enabled, anti_link_enabled, allowed_link_domains,
			anti_caps_enabled, caps_threshold_pct, bad_words_enabled, bad_words,
			exempt_roles, exempt_channels, action, log_channel_id, rule_role_scopes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17::jsonb)
		ON CONFLICT (guild_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			anti_spam_enabled = EXCLUDED.anti_spam_enabled,
			anti_spam_threshold = EXCLUDED.anti_spam_threshold,
			anti_spam_window_secs = EXCLUDED.anti_spam_window_secs,
			anti_invite_enabled = EXCLUDED.anti_invite_enabled,
			anti_link_enabled = EXCLUDED.anti_link_enabled,
			allowed_link_domains = EXCLUDED.allowed_link_domains,
			anti_caps_enabled = EXCLUDED.anti_caps_enabled,
			caps_threshold_pct = EXCLUDED.caps_threshold_pct,
			bad_words_enabled = EXCLUDED.bad_words_enabled,
			bad_words = EXCLUDED.bad_words,
			exempt_roles = EXCLUDED.exempt_roles,
			exempt_channels = EXCLUDED.exempt_channels,
			action = EXCLUDED.action,
			log_channel_id = EXCLUDED.log_channel_id,
			rule_role_scopes = EXCLUDED.rule_role_scopes`,
		c.GuildID, c.Enabled, c.AntiSpamEnabled, c.AntiSpamThreshold, c.AntiSpamWindowSecs,
		c.AntiInviteEnabled, c.AntiLinkEnabled, c.AllowedLinkDomains,
		c.AntiCapsEnabled, c.CapsThresholdPct, c.BadWordsEnabled, c.BadWords,
		c.ExemptRoles, c.ExemptChannels, c.Action, c.LogChannelID, string(scopesRaw))
	return err
}
