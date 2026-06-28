# Specter

Specter is a professional-grade Discord administration and utility bot written in Go with a built-in web dashboard (Discord OAuth2 + HTMX + Tailwind) that provides full configuration without leaving the browser.

## Features

- **Moderation** — `/ban`, `/unban`, `/kick`, `/timeout`, `/warning`, `/rapsheet`, `/clear`, `/lock`, `/unlock`, `/massban` with role-hierarchy safety checks and full rapsheet history.
- **Automod** — anti-spam, anti-invite, anti-link, anti-caps, and bad-word filtering with exempt roles/channels, configurable actions, and **per-rule role scoping** (limit a rule to, or exempt it from, specific roles).
- **Mod logging** — a private `Specter Logs` category with per-event channel routing and overrides, including **voice activity** (join/leave/move, server mute/deafen, streaming).
- **Moderation DMs** — optional DMs to members on warn/timeout/kick/ban with an appeal note (`/modnotify`).
- **Welcome & goodbye** — customizable join/leave messages (channel and/or DM) with placeholders, plain-text or embed (`/welcome`).
- **Autorole** — automatically assign roles to new members and bots on join (`/autorole`).
- **Starboard** — repost messages that reach a star threshold into a highlight channel (`/starboard`).
- **Leveling** — XP with cooldowns, exemptions, rank cards, a paginated leaderboard, and **role rewards** at levels with optional stacking (`/levelrole`).
- **Music** — multi-source playback (YouTube, YouTube Music, Spotify, SoundCloud) powered by a Lavalink node, with a per-guild queue. Lavalink owns the voice connection and supports Discord's DAVE E2EE protocol.
- **Reaction roles** — normal, unique, verify, and reverse menu types.
- **Join-to-create voice** — automatic temporary channels with owner controls (`/voice`).
- **Fun & utility** — `/advice`, `/cat`, `/dog`, `/meme`, `/wiki`, `/uwuify`, `/tweet`, downloads, `/avatar`, `/userinfo`, `/translate`, `/afk`, and more.
- **Access control** — layer custom per-group allow/deny rules on top of Discord permissions.
- **Dashboard** — server overview, configuration for levels, level rewards, automod (with role scopes), welcome, autorole, starboard, mod logs, mod DMs, and access; plus rapsheet search, reaction-role listing, music queue, and an audit log.

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

**Docker deploy (recommended):** just **Docker Engine + the Compose v2 plugin**
(`docker compose version` ≥ 2). Everything else — Go toolchain, PostgreSQL,
Lavalink, yt-dlp, fonts — is built/installed inside the containers.

**Running from source (local dev):**

- Go 1.26+ (matches `go.mod`)
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

Migrations run automatically on startup. Slash commands register **globally** (visible in every server) in production; set `ENVIRONMENT=development` + `DEV_GUILD_ID` to scope them to one test guild for instant updates while developing. Global commands can take a few minutes to propagate the first time. The bot connects to Lavalink on startup and retries with backoff if the node isn't up yet, so music becomes available as soon as the node is reachable.

### Docker Compose (one-click deploy)

Everything — the bot, PostgreSQL, and a Lavalink music node — runs from a single
`docker compose up -d`. On a fresh server with Docker Engine + the Compose v2
plugin installed:

```bash
git clone <your-repo-url> specter && cd specter
cp .env.example .env

# Fill in the REQUIRED values (Discord token + OAuth creds, public redirect URI)
# and generate a session secret:
echo "DASHBOARD_SESSION_SECRET=$(openssl rand -hex 32)" >> .env
nano .env                       # set DISCORD_TOKEN, DISCORD_CLIENT_ID/SECRET, etc.

docker compose up -d --build
docker compose logs -f specter  # watch it connect to Discord + Lavalink
```

That's it. The stack:

- **specter** — the bot + dashboard (`http://<host>:8080`). Database migrations
  run automatically on startup.
- **postgres** — data store, persisted in the `pgdata` volume. Only bound to
  `127.0.0.1`.
- **lavalink** — the music node, which **auto-downloads** the youtube-source and
  LavaSrc plugins on first boot (needs outbound internet; first start takes
  ~30–60s). Only bound to `127.0.0.1`.

Notes:

- Only port **8080** is published publicly; put it behind a reverse proxy
  (Caddy/Nginx) with TLS for production and point `DISCORD_REDIRECT_URI` at the
  HTTPS URL.
- The bot retries the Lavalink connection with backoff, so ordering during the
  first boot is handled automatically — music becomes available once the node
  finishes downloading plugins.
- Update to a new version with `git pull && docker compose up -d --build`.
- For a pure container deploy you can remove the `127.0.0.1:5432`/`127.0.0.1:2333`
  `ports` blocks from `docker-compose.yml` entirely (they exist only for local
  `go run` / debugging).

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
