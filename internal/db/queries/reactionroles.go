package queries

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/0xSalik/specter/internal/db"
)

// ReactionRoleMenu mirrors a row of reaction_role_menus.
type ReactionRoleMenu struct {
	ID          int
	GuildID     string
	ChannelID   string
	MessageID   string
	Name        string
	Description *string
	Type        string
	CreatedAt   time.Time
}

// ReactionRoleEntry mirrors a row of reaction_role_entries.
type ReactionRoleEntry struct {
	ID     int
	MenuID int
	Emoji  string
	RoleID string
}

// CreateMenu inserts a new reaction role menu and returns its ID.
func (s *Store) CreateMenu(ctx context.Context, guildID, channelID, messageID, name string, description *string, typ string) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx, `
		INSERT INTO reaction_role_menus (guild_id, channel_id, message_id, name, description, type)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		guildID, channelID, messageID, name, description, typ).Scan(&id)
	return id, err
}

// SetMenuMessageID updates the recorded Discord message ID for a menu.
func (s *Store) SetMenuMessageID(ctx context.Context, menuID int, messageID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE reaction_role_menus SET message_id = $2 WHERE id = $1`, menuID, messageID)
	return err
}

// UpdateMenu edits a menu's name and description.
func (s *Store) UpdateMenu(ctx context.Context, guildID string, menuID int, name string, description *string) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE reaction_role_menus SET name = $3, description = $4 WHERE id = $1 AND guild_id = $2`,
		menuID, guildID, name, description)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// GetMenu fetches a single menu by ID scoped to a guild.
func (s *Store) GetMenu(ctx context.Context, guildID string, menuID int) (*ReactionRoleMenu, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, guild_id, channel_id, message_id, name, description, type, created_at
		FROM reaction_role_menus WHERE id = $1 AND guild_id = $2`, menuID, guildID)
	return scanMenu(row)
}

// GetMenuByMessage fetches the menu associated with a Discord message.
func (s *Store) GetMenuByMessage(ctx context.Context, messageID string) (*ReactionRoleMenu, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, guild_id, channel_id, message_id, name, description, type, created_at
		FROM reaction_role_menus WHERE message_id = $1`, messageID)
	return scanMenu(row)
}

func scanMenu(row pgx.Row) (*ReactionRoleMenu, error) {
	var m ReactionRoleMenu
	err := row.Scan(&m.ID, &m.GuildID, &m.ChannelID, &m.MessageID, &m.Name, &m.Description, &m.Type, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListMenus returns all reaction role menus for a guild.
func (s *Store) ListMenus(ctx context.Context, guildID string) ([]ReactionRoleMenu, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, guild_id, channel_id, message_id, name, description, type, created_at
		FROM reaction_role_menus WHERE guild_id = $1 ORDER BY id`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ReactionRoleMenu
	for rows.Next() {
		var m ReactionRoleMenu
		if err := rows.Scan(&m.ID, &m.GuildID, &m.ChannelID, &m.MessageID, &m.Name, &m.Description, &m.Type, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteMenu removes a menu (entries cascade via FK).
func (s *Store) DeleteMenu(ctx context.Context, guildID string, menuID int) (bool, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM reaction_role_menus WHERE id = $1 AND guild_id = $2`, menuID, guildID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// AddEntry adds an emoji-to-role mapping to a menu.
func (s *Store) AddEntry(ctx context.Context, menuID int, emoji, roleID string) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx, `
		INSERT INTO reaction_role_entries (menu_id, emoji, role_id)
		VALUES ($1,$2,$3) RETURNING id`, menuID, emoji, roleID).Scan(&id)
	return id, err
}

// ListEntries returns all entries for a menu.
func (s *Store) ListEntries(ctx context.Context, menuID int) ([]ReactionRoleEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, menu_id, emoji, role_id FROM reaction_role_entries WHERE menu_id = $1 ORDER BY id`, menuID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ReactionRoleEntry
	for rows.Next() {
		var e ReactionRoleEntry
		if err := rows.Scan(&e.ID, &e.MenuID, &e.Emoji, &e.RoleID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountEntries returns the number of entries for a menu.
func (s *Store) CountEntries(ctx context.Context, menuID int) (int, error) {
	var c int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reaction_role_entries WHERE menu_id = $1`, menuID).Scan(&c)
	return c, err
}
