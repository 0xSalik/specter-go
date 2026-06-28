# Specter

Specter is a professional-grade Discord administration and utility bot written in Go. It is slash-command only, every response is a Discord embed, and the embed accent color is configurable per server. A built-in web dashboard (Discord OAuth2 + HTMX + Tailwind) provides full configuration without leaving the browser.

## Features

- **Moderation** — `/ban`, `/unban`, `/kick`, `/timeout`, `/warning`, `/rapsheet`, `/clear`, `/lock`, `/unlock`, `/massban` with role-hierarchy safety checks and full rapsheet history.
- **Automod** — anti-spam, anti-invite, anti-link, anti-caps, and bad-word filtering with exempt roles/channels and configurable actions.
- **Mod logging** — a private `Specter Logs` category with per-event channel routing and overrides.
- **Leveling** — XP with cooldowns, exemptions, rank cards, and a paginated leaderboard.
- **Music** — multi-source playback (YouTube, YouTube Music, Spotify, SoundCloud) powered by a Lavalink node, with a per-guild queue. Lavalink owns the voice connection and supports Discord's DAVE E2EE protocol.
- **Reaction roles** — normal, unique, verify, and reverse menu types.
- **Join-to-create voice** — automatic temporary channels with owner controls (`/voice`).
- **Fun & utility** — `/advice`, `/cat`, `/dog`, `/meme`, `/wiki`, `/uwuify`, `/tweet`, downloads, `/avatar`, `/userinfo`, `/translate`, `/afk`, and more.
- **Access control** — layer custom per-group allow/deny rules on top of Discord permissions.
- **Dashboard** — server overview, level/automod/modlog/access configuration, rapsheet search, reaction-role listing, music queue, and an audit log.

## Tech stack

| Concern | Choice |
|---|---|
| Language | Go 1.23+ |
| Discord | `bwmarrin/discordgo` |
| Music | Lavalink 4.2+ node via `disgolink/v4` (youtube-source + LavaSrc + SoundCloud) |
| Database | PostgreSQL via `pgx/v5` |
| Migrations | Embedded SQL, applied at startup |
| HTTP / dashboard | `net/http` + `go-chi/chi` |
| Frontend | Server-rendered `html/template` + HTMX + Tailwind (CDN) |
| Auth | Discord OAuth2 |
| Config | `viper` + `.env` / environment |
| Logging | `zerolog` |

## Requirements

- Go 1.23+
- PostgreSQL 14+
- A **Lavalink 4.2+** node for music (run via `docker compose up -d lavalink`; needs Java only if you run it outside Docker)
- `yt-dlp` on `PATH` for the `/tiktok` and `/ytdownload` media commands (not music)
- A DejaVu/Arial TrueType font for rank-card and tweet image rendering (bundled in the Docker image via `ttf-dejavu`)

## Configuration

Copy `.env.example` to `.env` and fill in the values:

```
DISCORD_TOKEN=...
DISCORD_CLIENT_ID=...
DISCORD_CLIENT_SECRET=...
DISCORD_REDIRECT_URI=http://localhost:8080/auth/callback
DATABASE_URL=postgres://specter:specter@localhost:5432/specter?sslmode=disable
DASHBOARD_PORT=8080
DASHBOARD_SESSION_SECRET=<32+ character random string>
YTDLP_PATH=yt-dlp
LAVALINK_ADDRESS=localhost:2333
LAVALINK_PASSWORD=youshallnotpass
LAVALINK_SECURE=false
# Optional Spotify source (consumed by the Lavalink container, not the bot).
SPOTIFY_CLIENT_ID=
SPOTIFY_CLIENT_SECRET=
LOG_LEVEL=info
ENVIRONMENT=production
# Optional: register commands instantly to one guild during development.
DEV_GUILD_ID=
```

## Running

### Local

```bash
createdb specter                 # or use docker-compose's postgres
docker compose up -d lavalink    # start the music node (needs Docker running)
go run ./cmd/specter
```

Migrations run automatically on startup. Slash commands register globally, or to `DEV_GUILD_ID` if set (instant updates while developing). The bot connects to Lavalink on startup and retries with backoff if the node isn't up yet, so music becomes available as soon as the node is reachable.

### Docker Compose

```bash
cp .env.example .env        # fill in Discord credentials
docker compose up --build
```

This starts Specter, PostgreSQL, and a Lavalink music node (which auto-downloads the youtube-source and LavaSrc plugins on first boot). The dashboard is available at `http://localhost:8080`.

## Project layout

```
cmd/specter         Entry point
internal/
  bot               Wiring: session, router, events, dashboard
  commands/*        Slash-command handlers, grouped by domain
  events            Gateway event handlers
  core              Dependency container, interaction context, router
  db                Connection pool, embedded migrations
  db/queries        Typed SQL access per domain
  embed             Fluent embed builder (per-guild color)
  access            Permission gate
  modlog            Centralized log dispatch + message cache
  automod           Rule engine
  levels            XP engine + rank card
  music             Lavalink-backed player, per-guild queue, voice forwarding
  reactionroles     Reaction-role event handling
  voice             Join-to-create
  dashboard         Web dashboard (OAuth2 + HTMX)
tests/
  unit              Pure-logic tests
  integration       PostgreSQL-backed tests (TEST_DATABASE_URL)
  e2e               Live Discord tests (build tag: e2e)
```

## Testing

```bash
# Unit tests (no dependencies)
go test -race ./tests/unit/...

# Integration tests (requires a PostgreSQL instance)
TEST_DATABASE_URL=postgres://specter:specter@localhost:5432/specter_test?sslmode=disable \
  go test -race ./tests/integration/...

# End-to-end tests (requires a dedicated test bot + guild)
go test -tags e2e ./tests/e2e/...
```

## License

MIT
