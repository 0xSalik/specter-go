// Package queries contains all SQL access for Specter, grouped by domain.
// Every exported method takes a context and returns explicit errors; no error
// is ever silently discarded.
package queries

import "github.com/jackc/pgx/v5/pgxpool"

// Store is the single entry point for database access. It is safe for
// concurrent use because pgxpool.Pool is concurrency-safe.
type Store struct {
	pool *pgxpool.Pool
}

// New constructs a Store backed by the supplied pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool exposes the underlying pool for advanced callers (e.g. test helpers).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }
