package queries

import "context"

// QueuedTrack mirrors a row of music_queue.
type QueuedTrack struct {
	GuildID   string
	Position  int
	Title     string
	URL       string
	Requester string
	Duration  *int
}

// SaveQueue replaces the persisted queue for a guild atomically.
func (s *Store) SaveQueue(ctx context.Context, guildID string, tracks []QueuedTrack) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `DELETE FROM music_queue WHERE guild_id = $1`, guildID); err != nil {
		return err
	}
	for i, t := range tracks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO music_queue (guild_id, position, title, url, requester, duration)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			guildID, i, t.Title, t.URL, t.Requester, t.Duration); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// LoadQueue returns the persisted queue for a guild in position order.
func (s *Store) LoadQueue(ctx context.Context, guildID string) ([]QueuedTrack, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT guild_id, position, title, url, requester, duration
		FROM music_queue WHERE guild_id = $1 ORDER BY position`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QueuedTrack
	for rows.Next() {
		var t QueuedTrack
		if err := rows.Scan(&t.GuildID, &t.Position, &t.Title, &t.URL, &t.Requester, &t.Duration); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ClearQueue removes all persisted tracks for a guild.
func (s *Store) ClearQueue(ctx context.Context, guildID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM music_queue WHERE guild_id = $1`, guildID)
	return err
}
