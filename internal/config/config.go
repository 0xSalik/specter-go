package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration for Specter. Values are loaded from
// environment variables (12-factor) with optional .env support via viper.
type Config struct {
	DiscordToken          string
	DiscordClientID       string
	DiscordClientSecret   string
	DiscordRedirectURI    string
	DatabaseURL           string
	DashboardPort         int
	DashboardSessionKey   string
	YTDLPPath             string
	LogLevel              string
	Environment           string
	DevGuildID            string // optional: register commands to this guild for instant updates
	GuildJoinLogChannelID string // optional: channel where bot server joins/leaves are logged
}

// Load reads configuration from the environment and an optional .env file.
// It returns an error if any required value is missing so the process can fail
// fast at startup rather than at first use.
func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	// Missing .env is not fatal; environment variables take precedence.
	_ = v.ReadInConfig()

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetDefault("DASHBOARD_PORT", 8080)
	v.SetDefault("DISCORD_REDIRECT_URI", "http://localhost:8080/auth/callback")
	v.SetDefault("YTDLP_PATH", "yt-dlp")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("ENVIRONMENT", "production")

	cfg := &Config{
		DiscordToken:          v.GetString("DISCORD_TOKEN"),
		DiscordClientID:       v.GetString("DISCORD_CLIENT_ID"),
		DiscordClientSecret:   v.GetString("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURI:    v.GetString("DISCORD_REDIRECT_URI"),
		DatabaseURL:           v.GetString("DATABASE_URL"),
		DashboardPort:         v.GetInt("DASHBOARD_PORT"),
		DashboardSessionKey:   v.GetString("DASHBOARD_SESSION_SECRET"),
		YTDLPPath:             v.GetString("YTDLP_PATH"),
		LogLevel:              v.GetString("LOG_LEVEL"),
		Environment:           v.GetString("ENVIRONMENT"),
		DevGuildID:            v.GetString("DEV_GUILD_ID"),
		GuildJoinLogChannelID: v.GetString("GUILD_JOIN_LOG_CHANNEL_ID"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	var missing []string
	if c.DiscordToken == "" {
		missing = append(missing, "DISCORD_TOKEN")
	}
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	if c.DashboardSessionKey != "" && len(c.DashboardSessionKey) < 32 {
		return fmt.Errorf("DASHBOARD_SESSION_SECRET must be at least 32 characters")
	}
	return nil
}

// IsDevelopment reports whether the bot runs in a development environment.
func (c *Config) IsDevelopment() bool {
	return strings.EqualFold(c.Environment, "development") || strings.EqualFold(c.Environment, "dev")
}
