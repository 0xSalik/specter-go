-- Specter feature expansion: welcome/goodbye messages, autorole, moderation DM
-- notifications, starboard, level role rewards, and per-rule automod role scoping.

-- Welcome / goodbye messages with placeholder support ({user}, {server}, ...).
CREATE TABLE IF NOT EXISTS welcome_config (
    guild_id         TEXT PRIMARY KEY,
    join_enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    join_channel_id  TEXT,
    join_message     TEXT,
    join_dm_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    join_dm_message  TEXT,
    leave_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    leave_channel_id TEXT,
    leave_message    TEXT,
    use_embed        BOOLEAN NOT NULL DEFAULT TRUE
);

-- Autorole: roles applied automatically when a member (or bot) joins.
CREATE TABLE IF NOT EXISTS autorole_config (
    guild_id     TEXT PRIMARY KEY,
    enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    role_ids     TEXT[] NOT NULL DEFAULT '{}',
    bot_role_ids TEXT[] NOT NULL DEFAULT '{}'
);

-- Moderation DM notification settings (per action) plus an appeal note.
CREATE TABLE IF NOT EXISTS mod_settings (
    guild_id       TEXT PRIMARY KEY,
    dm_on_warn     BOOLEAN NOT NULL DEFAULT TRUE,
    dm_on_timeout  BOOLEAN NOT NULL DEFAULT TRUE,
    dm_on_kick     BOOLEAN NOT NULL DEFAULT TRUE,
    dm_on_ban      BOOLEAN NOT NULL DEFAULT TRUE,
    appeal_message TEXT
);

-- Starboard configuration and posted-entry tracking.
CREATE TABLE IF NOT EXISTS starboard_config (
    guild_id   TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    channel_id TEXT,
    emoji      TEXT NOT NULL DEFAULT '⭐',
    threshold  INT NOT NULL DEFAULT 3,
    self_star  BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS starboard_messages (
    guild_id             TEXT NOT NULL,
    message_id           TEXT NOT NULL,
    channel_id           TEXT NOT NULL,
    starboard_message_id TEXT NOT NULL,
    star_count           INT NOT NULL DEFAULT 0,
    PRIMARY KEY (guild_id, message_id)
);

-- Level role rewards: assign a role when a member reaches a level.
CREATE TABLE IF NOT EXISTS level_rewards (
    guild_id TEXT NOT NULL,
    level    INT NOT NULL,
    role_id  TEXT NOT NULL,
    PRIMARY KEY (guild_id, level)
);

-- Whether lower reward roles are kept (stacked) as members level up.
ALTER TABLE level_config ADD COLUMN IF NOT EXISTS stack_rewards BOOLEAN NOT NULL DEFAULT TRUE;

-- Per-rule automod role scoping: JSON map of rule -> {include:[],exclude:[]}.
ALTER TABLE automod_config ADD COLUMN IF NOT EXISTS rule_role_scopes JSONB NOT NULL DEFAULT '{}'::jsonb;
