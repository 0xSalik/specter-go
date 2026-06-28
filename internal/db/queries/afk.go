package queries

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/0xSalik/specter/internal/db"
)

// AFKEntry mirrors a row of afk_users.
type AFKEntry struct {
	GuildID string
	UserID  string
	Reason  string
	SetAt   time.Time
}

// SetAFK records or updates a user's AFK status.
func (s *Store) SetAFK(ctx context.Context, guildID, userID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO afk_users (guild_id, user_id, reason, set_at)
		VALUES ($1,$2,$3,NOW())
		ON CONFLICT (guild_id, user_id) DO UPDATE SET reason = EXCLUDED.reason, set_at = NOW()`,
		guildID, userID, reason)
	return err
}

// GetAFK fetches a user's AFK status, or db.ErrNotFound if not AFK.
func (s *Store) GetAFK(ctx context.Context, guildID, userID string) (*AFKEntry, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT guild_id, user_id, reason, set_at FROM afk_users WHERE guild_id = $1 AND user_id = $2`,
		guildID, userID)
	var a AFKEntry
	err := row.Scan(&a.GuildID, &a.UserID, &a.Reason, &a.SetAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ClearAFK removes a user's AFK status, returning whether they were AFK.
func (s *Store) ClearAFK(ctx context.Context, guildID, userID string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM afk_users WHERE guild_id = $1 AND user_id = $2`, guildID, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
