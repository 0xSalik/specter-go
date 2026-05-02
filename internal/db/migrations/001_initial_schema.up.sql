-- Specter initial schema.
-- All Discord snowflakes are stored as TEXT to avoid integer overflow edge cases.

CREATE TABLE IF NOT EXISTS guilds (
    guild_id        TEXT PRIMARY KEY,
    embed_color     TEXT NOT NULL DEFAULT '#5865F2',
    prefix          TEXT NOT NULL DEFAULT '/',
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    log_category_id TEXT,
    general_log_id  TEXT,
    user_log_id     TEXT,
    message_log_id  TEXT,
    warn_log_id     TEXT,
    kick_log_id     TEXT,
    ban_log_id      TEXT
);

CREATE TABLE IF NOT EXISTS levels (
    guild_id    TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    xp          BIGINT NOT NULL DEFAULT 0,
    level       INT NOT NULL DEFAULT 0,
    total_msgs  BIGINT NOT NULL DEFAULT 0,
    last_xp_at  TIMESTAMPTZ,
    PRIMARY KEY (guild_id, user_id)
);

CREATE TABLE IF NOT EXISTS level_config (
    guild_id            TEXT PRIMARY KEY,
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    announce_channel_id TEXT,
    announce_msg        TEXT,
    xp_min              INT NOT NULL DEFAULT 15,
    xp_max              INT NOT NULL DEFAULT 40,
    xp_cooldown_secs    INT NOT NULL DEFAULT 60,
    no_xp_roles         TEXT[],
    no_xp_channels      TEXT[]
);

CREATE TABLE IF NOT EXISTS warnings (
    id          SERIAL PRIMARY KEY,
    guild_id    TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    mod_id      TEXT NOT NULL,
    reason      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    active      BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE INDEX IF NOT EXISTS idx_warnings_guild_user ON warnings(guild_id, user_id);

CREATE TABLE IF NOT EXISTS mod_actions (
    id          SERIAL PRIMARY KEY,
    guild_id    TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    mod_id      TEXT NOT NULL,
    action      TEXT NOT NULL,
    reason      TEXT,
    duration    INTERVAL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_mod_actions_guild_user ON mod_actions(guild_id, user_id);

CREATE TABLE IF NOT EXISTS automod_config (
    guild_id                TEXT PRIMARY KEY,
    enabled                 BOOLEAN NOT NULL DEFAULT FALSE,
    anti_spam_enabled       BOOLEAN NOT NULL DEFAULT FALSE,
    anti_spam_threshold     INT NOT NULL DEFAULT 5,
    anti_spam_window_secs   INT NOT NULL DEFAULT 5,
    anti_invite_enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    anti_link_enabled       BOOLEAN NOT NULL DEFAULT FALSE,
    allowed_link_domains    TEXT[],
    anti_caps_enabled       BOOLEAN NOT NULL DEFAULT FALSE,
    caps_threshold_pct      INT NOT NULL DEFAULT 70,
    bad_words_enabled       BOOLEAN NOT NULL DEFAULT FALSE,
    bad_words               TEXT[],
    exempt_roles            TEXT[],
    exempt_channels         TEXT[],
    action                  TEXT NOT NULL DEFAULT 'delete',
    log_channel_id          TEXT
);

CREATE TABLE IF NOT EXISTS reaction_role_menus (
    id          SERIAL PRIMARY KEY,
    guild_id    TEXT NOT NULL,
    channel_id  TEXT NOT NULL,
    message_id  TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    type        TEXT NOT NULL DEFAULT 'normal',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reaction_role_entries (
    id          SERIAL PRIMARY KEY,
    menu_id     INT NOT NULL REFERENCES reaction_role_menus(id) ON DELETE CASCADE,
    emoji       TEXT NOT NULL,
    role_id     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS jtc_config (
    guild_id        TEXT PRIMARY KEY,
    enabled         BOOLEAN NOT NULL DEFAULT FALSE,
    trigger_channel TEXT,
    category_id     TEXT,
    default_name    TEXT NOT NULL DEFAULT '{username}''s Channel',
    default_limit   INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS jtc_channels (
    channel_id  TEXT PRIMARY KEY,
    guild_id    TEXT NOT NULL,
    owner_id    TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS afk_users (
    guild_id    TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    reason      TEXT NOT NULL DEFAULT 'AFK',
    set_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guild_id, user_id)
);

CREATE TABLE IF NOT EXISTS modlog_overrides (
    guild_id    TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    channel_id  TEXT,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (guild_id, event_type)
);

CREATE TABLE IF NOT EXISTS access_control (
    guild_id        TEXT NOT NULL,
    command_group   TEXT NOT NULL,
    entity_type     TEXT NOT NULL,
    entity_id       TEXT NOT NULL,
    allowed         BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (guild_id, command_group, entity_type, entity_id)
);

CREATE TABLE IF NOT EXISTS music_queue (
    guild_id    TEXT NOT NULL,
    position    INT NOT NULL,
    title       TEXT NOT NULL,
    url         TEXT NOT NULL,
    requester   TEXT NOT NULL,
    duration    INT,
    PRIMARY KEY (guild_id, position)
);

CREATE TABLE IF NOT EXISTS audit_log (
    id          SERIAL PRIMARY KEY,
    guild_id    TEXT NOT NULL,
    actor_id    TEXT NOT NULL,
    action      TEXT NOT NULL,
    target      TEXT,
    detail      JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_log_guild ON audit_log(guild_id, created_at DESC);

-- PostgreSQL-backed dashboard session store.
CREATE TABLE IF NOT EXISTS sessions (
    token       TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    username    TEXT NOT NULL,
    avatar      TEXT,
    access_token TEXT NOT NULL,
    data        JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
