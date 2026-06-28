package queries

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ModSettings mirrors a row of the mod_settings table: whether the bot DMs a
// member when a moderation action is taken, plus an optional appeal note.
type ModSettings struct {
	GuildID       string
	DMOnWarn      bool
	DMOnTimeout   bool
	DMOnKick      bool
	DMOnBan       bool
	AppealMessage *string
}

// DefaultModSettings returns the default DM configuration (all enabled).
func DefaultModSettings(guildID string) *ModSettings {
	return &ModSettings{
		GuildID:     guildID,
		DMOnWarn:    true,
		DMOnTimeout: true,
		DMOnKick:    true,
		DMOnBan:     true,
	}
}

// GetModSettings fetches moderation DM settings, returning defaults if absent.
func (s *Store) GetModSettings(ctx context.Context, guildID string) (*ModSettings, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT guild_id, dm_on_warn, dm_on_timeout, dm_on_kick, dm_on_ban, appeal_message
		 FROM mod_settings WHERE guild_id = $1`, guildID)

	var c ModSettings
	err := row.Scan(&c.GuildID, &c.DMOnWarn, &c.DMOnTimeout, &c.DMOnKick, &c.DMOnBan, &c.AppealMessage)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultModSettings(guildID), nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpsertModSettings writes the full moderation DM configuration for a guild.
func (s *Store) UpsertModSettings(ctx context.Context, c *ModSettings) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO mod_settings (guild_id, dm_on_warn, dm_on_timeout, dm_on_kick, dm_on_ban, appeal_message)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (guild_id) DO UPDATE SET
			dm_on_warn = EXCLUDED.dm_on_warn,
			dm_on_timeout = EXCLUDED.dm_on_timeout,
			dm_on_kick = EXCLUDED.dm_on_kick,
			dm_on_ban = EXCLUDED.dm_on_ban,
			appeal_message = EXCLUDED.appeal_message`,
		c.GuildID, c.DMOnWarn, c.DMOnTimeout, c.DMOnKick, c.DMOnBan, c.AppealMessage)
	return err
}
