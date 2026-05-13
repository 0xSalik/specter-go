package queries

import (
	"context"
	"encoding/json"
	"time"
)

// AuditEntry mirrors a row of audit_log.
type AuditEntry struct {
	ID        int
	GuildID   string
	ActorID   string
	Action    string
	Target    *string
	Detail    json.RawMessage
	CreatedAt time.Time
}

// WriteAudit records a dashboard mutation in the audit log.
func (s *Store) WriteAudit(ctx context.Context, guildID, actorID, action string, target *string, detail any) error {
	var raw []byte
	if detail != nil {
		b, err := json.Marshal(detail)
		if err != nil {
			return err
		}
		raw = b
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_log (guild_id, actor_id, action, target, detail)
		VALUES ($1,$2,$3,$4,$5)`, guildID, actorID, action, target, raw)
	return err
}

// ListAudit returns audit entries for a guild, newest first, paginated.
func (s *Store) ListAudit(ctx context.Context, guildID string, limit, offset int) ([]AuditEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, guild_id, actor_id, action, target, detail, created_at
		FROM audit_log WHERE guild_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, guildID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var a AuditEntry
		if err := rows.Scan(&a.ID, &a.GuildID, &a.ActorID, &a.Action, &a.Target, &a.Detail, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CountAudit returns the number of audit entries for a guild.
func (s *Store) CountAudit(ctx context.Context, guildID string) (int, error) {
	var c int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE guild_id = $1`, guildID).Scan(&c)
	return c, err
}
