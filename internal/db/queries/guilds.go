package queries

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/salik/specter/internal/db"
)

// GuildConfig mirrors a row of the guilds table. Nullable columns use pointers.
type GuildConfig struct {
	GuildID       string
	EmbedColor    string
	Prefix        string
	JoinedAt      time.Time
	LogCategoryID *string
	GeneralLogID  *string
	UserLogID     *string
	MessageLogID  *string
	WarnLogID     *string
	KickLogID     *string
	BanLogID      *string
}

// EnsureGuild inserts a guild row with defaults if it does not already exist.
// It returns true when a new row was created (i.e. a genuinely new guild).
func (s *Store) EnsureGuild(ctx context.Context, guildID string) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO guilds (guild_id) VALUES ($1) ON CONFLICT (guild_id) DO NOTHING`, guildID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// GetGuild fetches a guild configuration. Returns db.ErrNotFound if absent.
func (s *Store) GetGuild(ctx context.Context, guildID string) (*GuildConfig, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT guild_id, embed_color, prefix, joined_at,
		       log_category_id, general_log_id, user_log_id, message_log_id,
		       warn_log_id, kick_log_id, ban_log_id
		FROM guilds WHERE guild_id = $1`, guildID)

	var g GuildConfig
	err := row.Scan(&g.GuildID, &g.EmbedColor, &g.Prefix, &g.JoinedAt,
		&g.LogCategoryID, &g.GeneralLogID, &g.UserLogID, &g.MessageLogID,
		&g.WarnLogID, &g.KickLogID, &g.BanLogID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// SetEmbedColor updates the per-guild embed accent color.
func (s *Store) SetEmbedColor(ctx context.Context, guildID, color string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE guilds SET embed_color = $2 WHERE guild_id = $1`, guildID, color)
	return err
}

// SetLogChannels persists the IDs of the auto-created log channels.
func (s *Store) SetLogChannels(ctx context.Context, guildID string, category, general, user, message, warn, kick, ban *string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE guilds SET
			log_category_id = $2, general_log_id = $3, user_log_id = $4,
			message_log_id = $5, warn_log_id = $6, kick_log_id = $7, ban_log_id = $8
		WHERE guild_id = $1`,
		guildID, category, general, user, message, warn, kick, ban)
	return err
}

// GuildStats holds aggregate counts for the dashboard overview.
type GuildStats struct {
	Bans       int
	Kicks      int
	Warnings   int
	XPEntries  int
	QueueSize  int
	AuditCount int
}

// GuildStats computes overview counts in a single round of queries.
func (s *Store) GuildStats(ctx context.Context, guildID string) (*GuildStats, error) {
	var st GuildStats
	row := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM mod_actions WHERE guild_id = $1 AND action = 'ban'),
			(SELECT COUNT(*) FROM mod_actions WHERE guild_id = $1 AND action = 'kick'),
			(SELECT COUNT(*) FROM warnings WHERE guild_id = $1 AND active = TRUE),
			(SELECT COUNT(*) FROM levels WHERE guild_id = $1),
			(SELECT COUNT(*) FROM music_queue WHERE guild_id = $1),
			(SELECT COUNT(*) FROM audit_log WHERE guild_id = $1)`, guildID)
	if err := row.Scan(&st.Bans, &st.Kicks, &st.Warnings, &st.XPEntries, &st.QueueSize, &st.AuditCount); err != nil {
		return nil, err
	}
	return &st, nil
}

// DeleteGuild removes a guild and all dependent rows. Tables without ON DELETE
// cascade are cleaned explicitly to keep the database tidy when the bot is
// removed from a server.
func (s *Store) DeleteGuild(ctx context.Context, guildID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback is a no-op after commit

	tables := []string{
		"levels", "level_config", "warnings", "mod_actions", "automod_config",
		"reaction_role_menus", "jtc_config", "jtc_channels", "afk_users",
		"modlog_overrides", "access_control", "music_queue", "audit_log", "guilds",
	}
	for _, t := range tables {
		if _, err := tx.Exec(ctx, "DELETE FROM "+t+" WHERE guild_id = $1", guildID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
