// Command specter is the entrypoint for the Specter Discord bot and dashboard.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/salik/specter/internal/bot"
	"github.com/salik/specter/internal/config"
	"github.com/salik/specter/internal/db"
	"github.com/salik/specter/internal/db/queries"
)

// version is overridden at build time via -ldflags.
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Logger is not configured yet; use a basic console logger.
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	setupLogger(cfg.LogLevel)
	log.Info().Str("version", version).Str("env", cfg.Environment).Msg("starting Specter")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		cancel()
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	if err := database.Migrate(ctx); err != nil {
		cancel()
		log.Fatal().Err(err).Msg("failed to run migrations")
	}
	cancel()

	store := queries.New(database.Pool)

	b, err := bot.New(cfg, store)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to construct bot")
	}
	if err := b.Open(); err != nil {
		log.Fatal().Err(err).Msg("failed to start bot")
	}
	log.Info().Msg("Specter is running. Press Ctrl+C to exit.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutting down...")
	b.Close()
	database.Close()
	log.Info().Msg("shutdown complete")
}

func setupLogger(level string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}
