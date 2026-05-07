package queries

import (
	"context"
	"time"
)

// Warning mirrors a row of the warnings table.
type Warning struct {
	ID        int
	GuildID   string
	UserID    string
	ModID     string
	Reason    string
	CreatedAt time.Time
	Active    bool
}

// AddWarning records a new active warning and returns its generated ID.
func (s *Store) AddWarning(ctx context.Context, guildID, userID, modID, reason string) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx, `
		INSERT INTO warnings (guild_id, user_id, mod_id, reason)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		guildID, userID, modID, reason).Scan(&id)
	return id, err
}

// RemoveWarning soft-deletes a warning (sets active = false). Returns whether a
// matching active warning existed.
func (s *Store) RemoveWarning(ctx context.Context, guildID string, warningID int) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE warnings SET active = FALSE WHERE id = $1 AND guild_id = $2 AND active = TRUE`,
		warningID, guildID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListWarnings returns active warnings for a user in a guild, newest first.
func (s *Store) ListWarnings(ctx context.Context, guildID, userID string) ([]Warning, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, guild_id, user_id, mod_id, reason, created_at, active
		FROM warnings WHERE guild_id = $1 AND user_id = $2 AND active = TRUE
		ORDER BY created_at DESC`, guildID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Warning
	for rows.Next() {
		var w Warning
		if err := rows.Scan(&w.ID, &w.GuildID, &w.UserID, &w.ModID, &w.Reason, &w.CreatedAt, &w.Active); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ClearWarnings hard-deletes all warnings for a user (rapsheet clear).
func (s *Store) ClearWarnings(ctx context.Context, guildID, userID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM warnings WHERE guild_id = $1 AND user_id = $2`, guildID, userID)
	return err
}
