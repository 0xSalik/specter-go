package queries

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/0xSalik/specter/internal/db"
)

// StarboardConfig mirrors a row of the starboard_config table.
type StarboardConfig struct {
	GuildID   string
	Enabled   bool
	ChannelID *string
	Emoji     string
	Threshold int
	SelfStar  bool // whether an author may star their own message
}

// GetStarboardConfig fetches starboard configuration, returning defaults if absent.
func (s *Store) GetStarboardConfig(ctx context.Context, guildID string) (*StarboardConfig, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT guild_id, enabled, channel_id, emoji, threshold, self_star
		 FROM starboard_config WHERE guild_id = $1`, guildID)

	var c StarboardConfig
	err := row.Scan(&c.GuildID, &c.Enabled, &c.ChannelID, &c.Emoji, &c.Threshold, &c.SelfStar)
	if errors.Is(err, pgx.ErrNoRows) {
		return &StarboardConfig{GuildID: guildID, Emoji: "⭐", Threshold: 3}, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpsertStarboardConfig writes the full starboard configuration for a guild.
func (s *Store) UpsertStarboardConfig(ctx context.Context, c *StarboardConfig) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO starboard_config (guild_id, enabled, channel_id, emoji, threshold, self_star)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (guild_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			channel_id = EXCLUDED.channel_id,
			emoji = EXCLUDED.emoji,
			threshold = EXCLUDED.threshold,
			self_star = EXCLUDED.self_star`,
		c.GuildID, c.Enabled, c.ChannelID, c.Emoji, c.Threshold, c.SelfStar)
	return err
}

// StarboardEntry tracks a message that has been posted to the starboard.
type StarboardEntry struct {
	GuildID            string
	MessageID          string
	ChannelID          string
	StarboardMessageID string
	StarCount          int
}

// GetStarboardEntry returns the starboard entry for an original message, or
// db.ErrNotFound if the message has not been posted to the starboard.
func (s *Store) GetStarboardEntry(ctx context.Context, guildID, messageID string) (*StarboardEntry, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT guild_id, message_id, channel_id, starboard_message_id, star_count
		 FROM starboard_messages WHERE guild_id = $1 AND message_id = $2`, guildID, messageID)

	var e StarboardEntry
	err := row.Scan(&e.GuildID, &e.MessageID, &e.ChannelID, &e.StarboardMessageID, &e.StarCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// UpsertStarboardEntry records or updates a starboard entry.
func (s *Store) UpsertStarboardEntry(ctx context.Context, e *StarboardEntry) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO starboard_messages (guild_id, message_id, channel_id, starboard_message_id, star_count)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (guild_id, message_id) DO UPDATE SET
			starboard_message_id = EXCLUDED.starboard_message_id,
			star_count = EXCLUDED.star_count`,
		e.GuildID, e.MessageID, e.ChannelID, e.StarboardMessageID, e.StarCount)
	return err
}

// DeleteStarboardEntry removes a starboard entry (e.g. when stars drop below the
// threshold or the post is removed).
func (s *Store) DeleteStarboardEntry(ctx context.Context, guildID, messageID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM starboard_messages WHERE guild_id = $1 AND message_id = $2`, guildID, messageID)
	return err
}
