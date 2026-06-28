package queries

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// AutoroleConfig mirrors a row of the autorole_config table.
type AutoroleConfig struct {
	GuildID    string
	Enabled    bool
	RoleIDs    []string // applied to human members on join
	BotRoleIDs []string // applied to bots on join
}

// GetAutoroleConfig fetches autorole configuration, returning defaults if absent.
func (s *Store) GetAutoroleConfig(ctx context.Context, guildID string) (*AutoroleConfig, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT guild_id, enabled, role_ids, bot_role_ids FROM autorole_config WHERE guild_id = $1`, guildID)

	var c AutoroleConfig
	err := row.Scan(&c.GuildID, &c.Enabled, &c.RoleIDs, &c.BotRoleIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		return &AutoroleConfig{GuildID: guildID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpsertAutoroleConfig writes the full autorole configuration for a guild.
func (s *Store) UpsertAutoroleConfig(ctx context.Context, c *AutoroleConfig) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO autorole_config (guild_id, enabled, role_ids, bot_role_ids)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (guild_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			role_ids = EXCLUDED.role_ids,
			bot_role_ids = EXCLUDED.bot_role_ids`,
		c.GuildID, c.Enabled, c.RoleIDs, c.BotRoleIDs)
	return err
}
