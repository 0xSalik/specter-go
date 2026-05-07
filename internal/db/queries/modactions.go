package queries

import (
	"context"
	"time"
)

// ModAction mirrors a row of the mod_actions table (the rapsheet history).
type ModAction struct {
	ID        int
	GuildID   string
	UserID    string
	ModID     string
	Action    string
	Reason    *string
	Duration  *time.Duration
	CreatedAt time.Time
}

// RecordAction inserts a moderation action into the rapsheet history.
func (s *Store) RecordAction(ctx context.Context, guildID, userID, modID, action string, reason *string, duration *time.Duration) (int, error) {
	var id int
	var interval *string
	if duration != nil {
		v := duration.String()
		interval = &v
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO mod_actions (guild_id, user_id, mod_id, action, reason, duration)
		VALUES ($1, $2, $3, $4, $5, $6::interval) RETURNING id`,
		guildID, userID, modID, action, reason, interval).Scan(&id)
	return id, err
}

// ListActions returns rapsheet entries for a user, newest first, paginated.
func (s *Store) ListActions(ctx context.Context, guildID, userID string, limit, offset int) ([]ModAction, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, guild_id, user_id, mod_id, action, reason, created_at
		FROM mod_actions WHERE guild_id = $1 AND user_id = $2
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`, guildID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ModAction
	for rows.Next() {
		var a ModAction
		if err := rows.Scan(&a.ID, &a.GuildID, &a.UserID, &a.ModID, &a.Action, &a.Reason, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CountActions returns the total rapsheet entries for a user in a guild.
func (s *Store) CountActions(ctx context.Context, guildID, userID string) (int, error) {
	var c int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM mod_actions WHERE guild_id = $1 AND user_id = $2`,
		guildID, userID).Scan(&c)
	return c, err
}

// ClearActions removes all rapsheet entries for a user in a guild.
func (s *Store) ClearActions(ctx context.Context, guildID, userID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM mod_actions WHERE guild_id = $1 AND user_id = $2`, guildID, userID)
	return err
}

// CountActionsByType returns the number of actions of a given type in a guild.
func (s *Store) CountActionsByType(ctx context.Context, guildID, action string) (int, error) {
	var c int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM mod_actions WHERE guild_id = $1 AND action = $2`,
		guildID, action).Scan(&c)
	return c, err
}
