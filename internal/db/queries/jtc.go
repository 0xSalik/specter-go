package queries

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/salik/specter/internal/db"
)

// JTCConfig mirrors a row of jtc_config.
type JTCConfig struct {
	GuildID        string
	Enabled        bool
	TriggerChannel *string
	CategoryID     *string
	DefaultName    string
	DefaultLimit   int
}

// JTCChannel mirrors a row of jtc_channels.
type JTCChannel struct {
	ChannelID string
	GuildID   string
	OwnerID   string
}

// GetJTCConfig fetches join-to-create config, returning defaults if absent.
func (s *Store) GetJTCConfig(ctx context.Context, guildID string) (*JTCConfig, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT guild_id, enabled, trigger_channel, category_id, default_name, default_limit
		FROM jtc_config WHERE guild_id = $1`, guildID)
	var c JTCConfig
	err := row.Scan(&c.GuildID, &c.Enabled, &c.TriggerChannel, &c.CategoryID, &c.DefaultName, &c.DefaultLimit)
	if errors.Is(err, pgx.ErrNoRows) {
		return &JTCConfig{GuildID: guildID, DefaultName: "{username}'s Channel"}, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpsertJTCConfig writes join-to-create configuration.
func (s *Store) UpsertJTCConfig(ctx context.Context, c *JTCConfig) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO jtc_config (guild_id, enabled, trigger_channel, category_id, default_name, default_limit)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (guild_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			trigger_channel = EXCLUDED.trigger_channel,
			category_id = EXCLUDED.category_id,
			default_name = EXCLUDED.default_name,
			default_limit = EXCLUDED.default_limit`,
		c.GuildID, c.Enabled, c.TriggerChannel, c.CategoryID, c.DefaultName, c.DefaultLimit)
	return err
}

// AddJTCChannel records an ephemeral join-to-create channel.
func (s *Store) AddJTCChannel(ctx context.Context, channelID, guildID, ownerID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO jtc_channels (channel_id, guild_id, owner_id) VALUES ($1,$2,$3)
		ON CONFLICT (channel_id) DO UPDATE SET owner_id = EXCLUDED.owner_id`,
		channelID, guildID, ownerID)
	return err
}

// GetJTCChannel returns an active JTC channel record, or db.ErrNotFound.
func (s *Store) GetJTCChannel(ctx context.Context, channelID string) (*JTCChannel, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT channel_id, guild_id, owner_id FROM jtc_channels WHERE channel_id = $1`, channelID)
	var c JTCChannel
	err := row.Scan(&c.ChannelID, &c.GuildID, &c.OwnerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SetJTCOwner transfers ownership of a JTC channel.
func (s *Store) SetJTCOwner(ctx context.Context, channelID, ownerID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE jtc_channels SET owner_id = $2 WHERE channel_id = $1`, channelID, ownerID)
	return err
}

// RemoveJTCChannel deletes a JTC channel record.
func (s *Store) RemoveJTCChannel(ctx context.Context, channelID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM jtc_channels WHERE channel_id = $1`, channelID)
	return err
}

// ListJTCChannels returns all recorded JTC channels (for startup cleanup).
func (s *Store) ListJTCChannels(ctx context.Context) ([]JTCChannel, error) {
	rows, err := s.pool.Query(ctx, `SELECT channel_id, guild_id, owner_id FROM jtc_channels`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []JTCChannel
	for rows.Next() {
		var c JTCChannel
		if err := rows.Scan(&c.ChannelID, &c.GuildID, &c.OwnerID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
