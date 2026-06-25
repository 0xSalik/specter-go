// Package testdb provides helpers for integration tests that exercise the real
// PostgreSQL query layer. Tests are skipped automatically when TEST_DATABASE_URL
// is not configured.
package testdb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/salik/specter/internal/db"
	"github.com/salik/specter/internal/db/queries"
)

var allTables = []string{
	"levels", "level_config", "warnings", "mod_actions", "automod_config",
	"reaction_role_entries", "reaction_role_menus", "jtc_config", "jtc_channels",
	"afk_users", "modlog_overrides", "access_control", "music_queue", "audit_log",
	"sessions", "guilds",
}

// Setup connects to the test database, applies migrations, and truncates all
// tables so each test starts from a clean slate. It returns the store and a
// cleanup function. The test is skipped if TEST_DATABASE_URL is unset.
func Setup(t *testing.T) (*queries.Store, func()) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	database, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if err := database.Migrate(ctx); err != nil {
		database.Close()
		t.Fatalf("migrate test db: %v", err)
	}
	truncate(t, database)

	store := queries.New(database.Pool)
	return store, func() {
		truncate(t, database)
		database.Close()
	}
}

func truncate(t *testing.T, database *db.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, tbl := range allTables {
		if _, err := database.Pool.Exec(ctx, "TRUNCATE TABLE "+tbl+" RESTART IDENTITY CASCADE"); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}
}
