package queries

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/salik/specter/internal/db"
)

// LevelEntry mirrors a row of the levels table.
type LevelEntry struct {
	GuildID   string
	UserID    string
	XP        int64
	Level     int
	TotalMsgs int64
	LastXPAt  *time.Time
}

// LevelConfig mirrors a row of the level_config table.
type LevelConfig struct {
	GuildID           string
	Enabled           bool
	AnnounceChannelID *string
	AnnounceMsg       *string
	XPMin             int
	XPMax             int
	XPCooldownSecs    int
	NoXPRoles         []string
	NoXPChannels      []string
}

// GetLevelConfig fetches level configuration, returning defaults if no row exists.
func (s *Store) GetLevelConfig(ctx context.Context, guildID string) (*LevelConfig, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT guild_id, enabled, announce_channel_id, announce_msg,
		       xp_min, xp_max, xp_cooldown_secs, no_xp_roles, no_xp_channels
		FROM level_config WHERE guild_id = $1`, guildID)

	var c LevelConfig
	err := row.Scan(&c.GuildID, &c.Enabled, &c.AnnounceChannelID, &c.AnnounceMsg,
		&c.XPMin, &c.XPMax, &c.XPCooldownSecs, &c.NoXPRoles, &c.NoXPChannels)
	if errors.Is(err, pgx.ErrNoRows) {
		return &LevelConfig{GuildID: guildID, Enabled: true, XPMin: 15, XPMax: 40, XPCooldownSecs: 60}, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpsertLevelConfig writes the full level configuration for a guild.
func (s *Store) UpsertLevelConfig(ctx context.Context, c *LevelConfig) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO level_config (guild_id, enabled, announce_channel_id, announce_msg,
			xp_min, xp_max, xp_cooldown_secs, no_xp_roles, no_xp_channels)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (guild_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			announce_channel_id = EXCLUDED.announce_channel_id,
			announce_msg = EXCLUDED.announce_msg,
			xp_min = EXCLUDED.xp_min,
			xp_max = EXCLUDED.xp_max,
			xp_cooldown_secs = EXCLUDED.xp_cooldown_secs,
			no_xp_roles = EXCLUDED.no_xp_roles,
			no_xp_channels = EXCLUDED.no_xp_channels`,
		c.GuildID, c.Enabled, c.AnnounceChannelID, c.AnnounceMsg,
		c.XPMin, c.XPMax, c.XPCooldownSecs, c.NoXPRoles, c.NoXPChannels)
	return err
}

// GetLevel fetches a single user's level entry. Returns db.ErrNotFound if absent.
func (s *Store) GetLevel(ctx context.Context, guildID, userID string) (*LevelEntry, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT guild_id, user_id, xp, level, total_msgs, last_xp_at
		FROM levels WHERE guild_id = $1 AND user_id = $2`, guildID, userID)

	var e LevelEntry
	err := row.Scan(&e.GuildID, &e.UserID, &e.XP, &e.Level, &e.TotalMsgs, &e.LastXPAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// AddXP upserts a user's XP and level in a single statement, returning the new entry.
func (s *Store) AddXP(ctx context.Context, guildID, userID string, xpDelta int64, newLevel int, at time.Time) (*LevelEntry, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO levels (guild_id, user_id, xp, level, total_msgs, last_xp_at)
		VALUES ($1, $2, $3, $4, 1, $5)
		ON CONFLICT (guild_id, user_id) DO UPDATE SET
			xp = levels.xp + $3,
			level = $4,
			total_msgs = levels.total_msgs + 1,
			last_xp_at = $5
		RETURNING guild_id, user_id, xp, level, total_msgs, last_xp_at`,
		guildID, userID, xpDelta, newLevel, at)

	var e LevelEntry
	if err := row.Scan(&e.GuildID, &e.UserID, &e.XP, &e.Level, &e.TotalMsgs, &e.LastXPAt); err != nil {
		return nil, err
	}
	return &e, nil
}

// SetLevel forcibly overrides a user's level and XP (admin override).
func (s *Store) SetLevel(ctx context.Context, guildID, userID string, level int, xp int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO levels (guild_id, user_id, xp, level)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (guild_id, user_id) DO UPDATE SET xp = $3, level = $4`,
		guildID, userID, xp, level)
	return err
}

// GetRank returns a user's 1-based rank within their guild by XP.
func (s *Store) GetRank(ctx context.Context, guildID, userID string) (int, error) {
	var rank int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) + 1 FROM levels
		WHERE guild_id = $1 AND xp > (SELECT xp FROM levels WHERE guild_id = $1 AND user_id = $2)`,
		guildID, userID).Scan(&rank)
	if err != nil {
		return 0, err
	}
	return rank, nil
}

// GetTopN returns the top N users in a guild ordered by XP descending.
func (s *Store) GetTopN(ctx context.Context, guildID string, n, offset int) ([]LevelEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT guild_id, user_id, xp, level, total_msgs, last_xp_at
		FROM levels WHERE guild_id = $1
		ORDER BY xp DESC LIMIT $2 OFFSET $3`, guildID, n, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LevelEntry
	for rows.Next() {
		var e LevelEntry
		if err := rows.Scan(&e.GuildID, &e.UserID, &e.XP, &e.Level, &e.TotalMsgs, &e.LastXPAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountLevelEntries returns the number of ranked users in a guild.
func (s *Store) CountLevelEntries(ctx context.Context, guildID string) (int, error) {
	var c int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM levels WHERE guild_id = $1`, guildID).Scan(&c)
	return c, err
}
