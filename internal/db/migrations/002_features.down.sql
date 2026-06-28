ALTER TABLE automod_config DROP COLUMN IF EXISTS rule_role_scopes;
ALTER TABLE level_config DROP COLUMN IF EXISTS stack_rewards;
DROP TABLE IF EXISTS level_rewards;
DROP TABLE IF EXISTS starboard_messages;
DROP TABLE IF EXISTS starboard_config;
DROP TABLE IF EXISTS mod_settings;
DROP TABLE IF EXISTS autorole_config;
DROP TABLE IF EXISTS welcome_config;
