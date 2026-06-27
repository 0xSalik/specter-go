package queries

import "context"

// AccessRule mirrors a row of the access_control table.
type AccessRule struct {
	GuildID      string
	CommandGroup string
	EntityType   string // "role" or "user"
	EntityID     string
	Allowed      bool
}

// SetAccessRule upserts an allow/deny rule for a command group and entity.
func (s *Store) SetAccessRule(ctx context.Context, r AccessRule) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO access_control (guild_id, command_group, entity_type, entity_id, allowed)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (guild_id, command_group, entity_type, entity_id)
		DO UPDATE SET allowed = EXCLUDED.allowed`,
		r.GuildID, r.CommandGroup, r.EntityType, r.EntityID, r.Allowed)
	return err
}

// DeleteAccessRule removes an access rule.
func (s *Store) DeleteAccessRule(ctx context.Context, guildID, group, entityType, entityID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM access_control
		WHERE guild_id = $1 AND command_group = $2 AND entity_type = $3 AND entity_id = $4`,
		guildID, group, entityType, entityID)
	return err
}

// ListAccessRules returns all access rules for a command group in a guild.
func (s *Store) ListAccessRules(ctx context.Context, guildID, group string) ([]AccessRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT guild_id, command_group, entity_type, entity_id, allowed
		FROM access_control WHERE guild_id = $1 AND command_group = $2`, guildID, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AccessRule
	for rows.Next() {
		var r AccessRule
		if err := rows.Scan(&r.GuildID, &r.CommandGroup, &r.EntityType, &r.EntityID, &r.Allowed); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListAllAccessRules returns every access rule for a guild (dashboard view).
func (s *Store) ListAllAccessRules(ctx context.Context, guildID string) ([]AccessRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT guild_id, command_group, entity_type, entity_id, allowed
		FROM access_control WHERE guild_id = $1 ORDER BY command_group`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AccessRule
	for rows.Next() {
		var r AccessRule
		if err := rows.Scan(&r.GuildID, &r.CommandGroup, &r.EntityType, &r.EntityID, &r.Allowed); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
