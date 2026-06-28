# Specter

Specter is a production-grade Discord moderation and community bot written in Go, with a built-in web dashboard for full configuration in the browser. It bundles the feature set you would normally assemble from several paid bots — moderation with audit history, automod, leveling, multi-source music, reaction roles, starboard, welcome flows, and more — into a single statically-linked binary that deploys with one `docker compose up`.

This README doubles as an architecture overview: it explains what the bot does, how the pieces fit together, and the reasoning behind the design decisions.

---

## What it does

Specter is built around a few coherent subsystems rather than a grab-bag of commands:

- **Moderation** — `/ban`, `/unban`, `/kick`, `/timeout`, `/warning`, `/rapsheet`, `/clear`, `/lock`, `/massban`, with role-hierarchy safety checks, persistent rapsheets, and optional DM notifications to the target (with an appeal note).
- **Automod** — anti-spam, invite/link filtering, caps, and bad-word matching with exempt roles/channels, configurable enforcement actions, and per-rule role scoping (apply a rule only to, or exempt it from, specific roles).
- **Mod & audit logging** — an auto-provisioned private log category with per-event channel routing and overrides, covering message edits/deletes (including attachment- and embed-only messages), member join/leave with invite attribution, and voice activity (join/leave/move, mute/deafen, streaming).
- **Leveling** — message XP with cooldowns and exemptions, rendered rank cards, a paginated leaderboard, and role rewards granted at configurable levels (with optional stacking).
- **Music** — multi-source playback (YouTube, YouTube Music, Spotify, SoundCloud) backed by a Lavalink node, plus a full web-based player with a live queue, drag-and-drop reordering, and DJ-role-gated controls.
- **Community** — reaction roles (normal/unique/verify/reverse), join-to-create voice channels, starboard, customizable welcome/goodbye messages, and autorole for humans and bots.
- **Utility & fun** — user/server info, avatars, translation, AFK, media downloads, and assorted API-backed commands.
- **Dashboard** — Discord OAuth2-authenticated web UI for configuring every subsystem, plus rapsheet search, an audit log, and the live music player.

Every user-facing response is a Discord embed themed with a per-guild accent color, produced by a single fluent builder so the bot has one consistent visual voice.

---

## How it works

### System shape

Specter is a single long-lived Go process that maintains a persistent gateway connection to Discord, talks to PostgreSQL for all state, and delegates audio to a Lavalink node. The dashboard is served from the same process.

```mermaid
flowchart LR
  subgraph Discord
    GW[Gateway WS]
    REST[REST API]
    VOICE[Voice / DAVE E2EE]
  end

  subgraph Host
    BOT["Specter process (Go)"]
    DASH["Dashboard :8080"]
    PG[("PostgreSQL")]
    LL["Lavalink node"]
  end

  GW <--> BOT
  BOT --> REST
  BOT --> PG
  BOT <-->|"voice events forwarded"| LL
  LL <--> VOICE
  BOT --- DASH
  DASH --> PG
  BROWSER["Browser (admins + DJs)"] --> DASH
```

The process is intentionally monolithic. A community bot is not a high-throughput service that benefits from horizontal scaling; it benefits from low operational surface area. One binary, one database, one optional music node is something a single person can reason about, deploy, and debug.

### Startup sequence

On boot the bot: loads config from environment/`.env`, opens a pooled PostgreSQL connection, applies embedded SQL migrations idempotently, constructs the shared dependency container, registers commands and event handlers, opens the Discord session, performs bulk slash-command registration, dials Lavalink (with retry/backoff so the music node can come up independently), and starts the dashboard HTTP server. Shutdown is the reverse, gated on context timeouts.

### Command framework

Interactions flow through a small internal router (`internal/core`) rather than scattered `switch` statements:

- Each command is a `Command{Def, Group, RequiredPerm, Handler}`. `Def` is the Discord schema; `Group` ties it to the access-control system; `RequiredPerm` is the Discord permission bit.
- Handlers receive a `Context` that wraps the interaction with ergonomic helpers (reply, ephemeral reply, deferred reply, error embeds) and the per-guild embed builder. `Context` is constructed defensively — it only reads `ApplicationCommandData` for command interactions, never for components — which is what keeps a mistyped interaction type from panicking.
- **Two layers of authorization.** At registration time, `RequiredPerm` is surfaced to Discord as `default_member_permissions`, so the client hides commands a member can't use (a regular user never sees `/ban`). At runtime, a custom access-control gate re-checks every command against per-guild allow/deny rules layered on top of Discord permissions, so admins can grant or revoke access per role/member without the client ever being the source of truth.
- **Nothing a handler does can crash the process.** discordgo dispatches each event in its own goroutine, so an unrecovered panic would terminate the program. The router wraps both the top-level dispatch and each individual handler in `recover()`, logs the panic with context, and replies with a generic error embed instead of dying.

### Events and background work

Gateway events (messages, member add/remove, reactions, voice state, guild join/leave, invites) are handled in `internal/events`, which fans out to the relevant subsystems: automod evaluation, XP accrual, starboard sync, welcome/autorole, invite attribution, and log dispatch. Per-guild mutable state (music players, message cache) is held in `sync.Map`/mutex-guarded structures so concurrent events across guilds never contend on a global lock.

### Data layer

PostgreSQL via `pgx/v5` with a connection pool. SQL lives in versioned migration files embedded into the binary with `embed.FS` and applied at startup, so a fresh deploy self-provisions its schema with no manual step. Query code is split into typed, domain-scoped packages (`internal/db/queries/*`) — `guilds`, `levels`, `automod`, `warnings`, `starboard`, `sessions`, and so on — each owning its own structs and SQL. There is no ORM; the queries are hand-written and explicit, which keeps the data access obvious and the dependency surface small.

### Music subsystem

Discord now mandates end-to-end-encrypted voice (the DAVE protocol). Rather than reimplement DAVE and Opus handling in Go, Specter offloads audio to **Lavalink** via `disgolink/v4`. The bot's only job is to forward the relevant Discord voice gateway events to Lavalink and translate user commands into player updates; Lavalink owns the UDP voice connection, the encryption handshake, and source resolution (youtube-source for YouTube/YouTube Music, LavaSrc for Spotify, built-in SoundCloud).

The per-guild queue is held **in memory**, not in the database. Playback state is inherently ephemeral and tied to a live voice connection — persisting it would create more consistency problems (stale queues after a restart, orphaned entries) than it solves. Each queued track carries a stable ID so the dashboard can target it for removal or reordering, and queue mutations are guarded by a per-guild mutex.

### Dashboard and the public music player

The dashboard is server-rendered `html/template` enhanced with HTMX and Tailwind — no SPA, no build step, no client framework. For a configuration UI whose interactions are "submit a form, re-render a fragment," this removes an entire toolchain and ships HTML the browser can render immediately.

Authentication is Discord OAuth2 with server-side sessions stored in PostgreSQL. Authorization is deliberately two-tiered:

- **Configuration pages** require **Manage Server** on the target guild (verified against the user's OAuth guild list).
- **The music player** (`/music-player/{guild}`) lives outside that gate: any logged-in member can view it, but mutating actions (play, queue edits, volume) require a configurable **DJ role** — or, if no DJ role is set, simply being in a voice channel. This mirrors how people actually use a music bot: everyone watches, trusted users drive.

The player itself is a live view (artwork, source, requester, progress) with drag-and-drop queue reordering, backed by a small JSON state/control API on the same routes.

---

## Key technical decisions and tradeoffs

- **Go.** A bot is a long-lived, I/O-bound, highly concurrent process — exactly Go's wheelhouse. Goroutines map cleanly onto per-event handling, and a single static binary makes the container image small and the deploy trivial. The cost is more boilerplate than a dynamic language; the payoff is a process that is fast to start, cheap to run, and hard to crash.
- **Lavalink over native audio.** Implementing DAVE E2EE and source extraction in-process would be a large, fragile surface that breaks every time Discord or YouTube changes something. Delegating to Lavalink trades one extra service (and ~1 GB of JVM memory) for a robust, well-maintained audio layer and four sources for free. For a music feature, that is the right trade.
- **Server-rendered HTMX, not a SPA.** The dashboard is forms and fragments. A React/Vue front end would add a build pipeline, a bundle, and an API contract to maintain for no user-visible benefit. HTMX keeps everything in `html/template` and the logic on the server where the data already is.
- **Embedded migrations, no ORM.** Schema travels with the binary and applies itself, so there is never a "did you run migrations?" step. Hand-written SQL in typed query packages keeps queries legible and avoids the abstraction tax and hidden N+1s of an ORM.
- **In-memory queue.** Accepts that music state is lost on restart in exchange for not maintaining a second source of truth for something that is meaningless without a live voice connection.
- **Global vs. guild command registration.** Commands register globally in production (visible everywhere, with a few minutes of first-time propagation) and can be scoped to a single dev guild for instant iteration. Command visibility per role is pushed down to Discord via `default_member_permissions` so the client does the filtering, with the runtime gate as the authoritative backstop.
- **Defensive routing.** Panic recovery at the dispatch and handler level, type-checked interaction parsing, and context timeouts on all external I/O reflect a simple operating principle: a single bad input or flaky API call should degrade one interaction, never the whole process.

---

## Tech stack

| Concern | Choice |
|---|---|
| Language | Go 1.26 |
| Discord | `bwmarrin/discordgo` |
| Music | Lavalink 4 node via `disgolink/v4` (youtube-source + LavaSrc + SoundCloud) |
| Database | PostgreSQL via `pgx/v5` (pooled) |
| Migrations | Embedded SQL, applied idempotently at startup |
| HTTP / routing | `net/http` + `go-chi/chi` |
| Frontend | Server-rendered `html/template` + HTMX + Tailwind (CDN) |
| Auth | Discord OAuth2, PostgreSQL-backed sessions |
| Config | `viper` (`.env` + environment) |
| Logging | `zerolog` (structured) |
| Image rendering | `fogleman/gg` (rank cards, tweet cards) |
| Deploy | Multi-stage Docker build + Docker Compose |

---

## Project layout

```
cmd/specter            Entry point and lifecycle
internal/
  bot                  Top-level wiring: session, router, events, dashboard
  core                 Command/component router, interaction Context, deps container
  commands/*           Slash-command handlers grouped by domain
  events               Gateway event handlers and fan-out
  db                   Pooled connection + embedded migrations
  db/queries           Typed, domain-scoped SQL access
  embed                Fluent embed builder (per-guild accent color)
  access               Runtime access-control gate
  modlog               Centralized log dispatch + message cache
  automod              Rule engine (with per-rule role scoping)
  levels               XP engine + rendered rank cards
  music                Lavalink-backed player, in-memory queue, voice forwarding
  reactionroles        Reaction-role event handling
  voice                Join-to-create temporary channels
  starboard            Star reaction aggregation
  invites              Invite tracking for join attribution
  guildsetup           First-join provisioning (log channels, defaults)
  dashboard            Web dashboard + public music player (OAuth2, sessions)
  discordutil, httpx   Shared helpers
migrations             Versioned schema
tests/                 unit / integration / e2e
```

---

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
LAVALINK_ADDRESS=localhost:2333
LAVALINK_PASSWORD=youshallnotpass
LAVALINK_SECURE=false
# Optional Spotify source (consumed by the Lavalink container, not the bot).
SPOTIFY_CLIENT_ID=
SPOTIFY_CLIENT_SECRET=
LOG_LEVEL=info
ENVIRONMENT=production
# Optional: scope command registration to one guild for instant dev iteration.
DEV_GUILD_ID=
# Optional: channel for bot-level guild join/leave logging.
GUILD_JOIN_LOG_CHANNEL_ID=
```

---

## Running

### Docker Compose (recommended)

The whole stack — bot + dashboard, PostgreSQL, and Lavalink — comes up from one command. On a host with Docker Engine and the Compose v2 plugin:

```bash
git clone <your-repo-url> specter && cd specter
cp .env.example .env
echo "DASHBOARD_SESSION_SECRET=$(openssl rand -hex 32)" >> .env
# Fill in DISCORD_TOKEN, DISCORD_CLIENT_ID/SECRET, and a public DISCORD_REDIRECT_URI.

docker compose up -d --build
docker compose logs -f specter
```

Migrations run automatically on startup. Only port `8080` is published; put it behind a reverse proxy (Caddy/Nginx) with TLS in production and point `DISCORD_REDIRECT_URI` at the HTTPS URL. The Lavalink container auto-downloads its source plugins on first boot, and the bot retries the connection with backoff, so startup ordering is handled for you.

### From source (development)

```bash
docker compose up -d postgres lavalink   # dependencies only
go run ./cmd/specter
```

Set `ENVIRONMENT=development` and `DEV_GUILD_ID` to register commands to a single guild for instant updates while iterating.

---

## Testing

The suite follows a standard pyramid:

```bash
# Unit — pure logic, no external dependencies
go test -race ./tests/unit/...

# Integration — against a real PostgreSQL instance
TEST_DATABASE_URL=postgres://specter:specter@localhost:5432/specter_test?sslmode=disable \
  go test -race ./tests/integration/...

# End-to-end — against a live test bot + guild (build-tagged)
go test -tags e2e ./tests/e2e/...
```

Unit tests cover the logic that is easy to get wrong and cheap to verify in isolation (access-control resolution, automod rule evaluation including role scoping, embed construction, leveling math). Integration tests exercise the query layer against real Postgres; e2e tests drive the bot against Discord behind a build tag so they never run by accident.

---

## License

[MIT](LICENSE)
