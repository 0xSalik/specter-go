-- DJ role: the role allowed to control the public music player. When unset,
-- anyone currently in a voice channel may control playback.
ALTER TABLE guilds ADD COLUMN IF NOT EXISTS dj_role_id TEXT;
