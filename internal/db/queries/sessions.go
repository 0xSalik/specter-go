package queries

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/0xSalik/specter/internal/db"
)

// Session mirrors a row of the sessions table (dashboard auth).
type Session struct {
	Token       string
	UserID      string
	Username    string
	Avatar      *string
	AccessToken string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// CreateSession persists a new dashboard session.
func (s *Store) CreateSession(ctx context.Context, sess Session) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (token, user_id, username, avatar, access_token, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		sess.Token, sess.UserID, sess.Username, sess.Avatar, sess.AccessToken, sess.ExpiresAt)
	return err
}

// GetSession fetches a non-expired session by token.
func (s *Store) GetSession(ctx context.Context, token string) (*Session, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT token, user_id, username, avatar, access_token, created_at, expires_at
		FROM sessions WHERE token = $1 AND expires_at > NOW()`, token)
	var sess Session
	err := row.Scan(&sess.Token, &sess.UserID, &sess.Username, &sess.Avatar,
		&sess.AccessToken, &sess.CreatedAt, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// DeleteSession removes a session (logout).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}

// PruneSessions deletes expired sessions.
func (s *Store) PruneSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= NOW()`)
	return err
}
