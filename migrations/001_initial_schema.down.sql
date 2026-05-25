-- 001_initial_schema.down.sql

DROP TRIGGER IF EXISTS api_keys_updated_at  ON api_keys;
DROP TRIGGER IF EXISTS models_updated_at    ON models;
DROP TRIGGER IF EXISTS providers_updated_at ON providers;
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS usage_logs;
DROP TABLE IF EXISTS api_key_model_rules;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS admin_users;
DROP TABLE IF EXISTS models;
DROP TABLE IF EXISTS providers;
