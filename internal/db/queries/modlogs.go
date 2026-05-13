package queries

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/salik/specter/internal/db"
)

// ModlogOverride mirrors a row of modlog_overrides.
type ModlogOverride struct {
	GuildID   string
	EventType string
	ChannelID *string
	Enabled   bool
}

// GetOverride returns the override for an event type, or db.ErrNotFound.
func (s *Store) GetOverride(ctx context.Context, guildID, eventType string) (*ModlogOverride, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT guild_id, event_type, channel_id, enabled
		FROM modlog_overrides WHERE guild_id = $1 AND event_type = $2`, guildID, eventType)
	var o ModlogOverride
	err := row.Scan(&o.GuildID, &o.EventType, &o.ChannelID, &o.Enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// SetOverride upserts an event-type override.
func (s *Store) SetOverride(ctx context.Context, o ModlogOverride) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO modlog_overrides (guild_id, event_type, channel_id, enabled)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (guild_id, event_type)
		DO UPDATE SET channel_id = EXCLUDED.channel_id, enabled = EXCLUDED.enabled`,
		o.GuildID, o.EventType, o.ChannelID, o.Enabled)
	return err
}

// ListOverrides returns all overrides for a guild.
func (s *Store) ListOverrides(ctx context.Context, guildID string) ([]ModlogOverride, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT guild_id, event_type, channel_id, enabled FROM modlog_overrides WHERE guild_id = $1`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ModlogOverride
	for rows.Next() {
		var o ModlogOverride
		if err := rows.Scan(&o.GuildID, &o.EventType, &o.ChannelID, &o.Enabled); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
