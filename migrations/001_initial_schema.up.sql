-- 001_initial_schema.up.sql
-- Core schema for Bridage Token Relay Server

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ─── Providers ───────────────────────────────────────────────────────────────

CREATE TABLE providers (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT        NOT NULL UNIQUE,
    display_name     TEXT        NOT NULL,
    adapter_type     TEXT        NOT NULL CHECK (adapter_type IN ('openai_compatible','anthropic','gemini')),
    base_url         TEXT        NOT NULL,
    auth_header      TEXT        NOT NULL DEFAULT 'Authorization',
    auth_scheme      TEXT        NOT NULL DEFAULT 'Bearer',
    encrypted_api_key TEXT       NOT NULL,
    enabled          BOOLEAN     NOT NULL DEFAULT TRUE,
    -- capability flags
    cap_chat         BOOLEAN     NOT NULL DEFAULT TRUE,
    cap_responses    BOOLEAN     NOT NULL DEFAULT FALSE,
    cap_embeddings   BOOLEAN     NOT NULL DEFAULT FALSE,
    cap_images       BOOLEAN     NOT NULL DEFAULT FALSE,
    cap_streaming    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Model catalog ────────────────────────────────────────────────────────────

CREATE TABLE models (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id     UUID        NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    name            TEXT        NOT NULL,          -- canonical name for downstream callers
    provider_model  TEXT        NOT NULL,          -- actual model id sent to upstream
    description     TEXT        NOT NULL DEFAULT '',
    enabled         BOOLEAN     NOT NULL DEFAULT TRUE,
    supports_images BOOLEAN     NOT NULL DEFAULT FALSE,
    context_window  INT         NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider_id, name)
);

CREATE INDEX models_provider_id_idx ON models(provider_id);

-- ─── Admin users ──────────────────────────────────────────────────────────────

CREATE TABLE admin_users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Downstream API keys ──────────────────────────────────────────────────────

CREATE TABLE api_keys (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT        NOT NULL,
    key_hash      TEXT        NOT NULL,              -- bcrypt hash
    key_index     TEXT        NOT NULL UNIQUE,       -- SHA-256 fast lookup
    status        TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled','expired')),
    expires_at    TIMESTAMPTZ,
    rate_limit    INT         NOT NULL DEFAULT 0,    -- req/min; 0 = unlimited
    quota_tokens  BIGINT      NOT NULL DEFAULT 0,    -- 0 = unlimited
    used_tokens   BIGINT      NOT NULL DEFAULT 0,
    quota_reqs    BIGINT      NOT NULL DEFAULT 0,    -- 0 = unlimited
    used_reqs     BIGINT      NOT NULL DEFAULT 0,
    provider_id   UUID        REFERENCES providers(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX api_keys_key_index_idx ON api_keys(key_index);
CREATE INDEX api_keys_status_idx    ON api_keys(status);

-- ─── Per-key model rules ──────────────────────────────────────────────────────

CREATE TABLE api_key_model_rules (
    id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID    NOT NULL REFERENCES api_keys(id)  ON DELETE CASCADE,
    model_id   UUID    NOT NULL REFERENCES models(id)    ON DELETE CASCADE,
    allow      BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE (api_key_id, model_id)
);

CREATE INDEX api_key_model_rules_key_idx ON api_key_model_rules(api_key_id);

-- ─── Usage logs ───────────────────────────────────────────────────────────────

CREATE TABLE usage_logs (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id          UUID        NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    provider_id         UUID        NOT NULL,
    model_id            UUID        NOT NULL,
    endpoint            TEXT        NOT NULL,
    prompt_tokens       INT         NOT NULL DEFAULT 0,
    completion_tokens   INT         NOT NULL DEFAULT 0,
    total_tokens        INT         NOT NULL DEFAULT 0,
    duration_ms         BIGINT      NOT NULL DEFAULT 0,
    status_code         INT         NOT NULL DEFAULT 200,
    error               TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX usage_logs_api_key_idx  ON usage_logs(api_key_id);
CREATE INDEX usage_logs_created_idx  ON usage_logs(created_at DESC);
CREATE INDEX usage_logs_provider_idx ON usage_logs(provider_id);

-- ─── Audit log ────────────────────────────────────────────────────────────────

CREATE TABLE audit_logs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    actor       TEXT        NOT NULL,   -- admin username or api_key id
    action      TEXT        NOT NULL,
    resource    TEXT        NOT NULL,
    detail      JSONB       NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX audit_logs_created_idx ON audit_logs(created_at DESC);

-- ─── updated_at trigger ───────────────────────────────────────────────────────

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE TRIGGER providers_updated_at BEFORE UPDATE ON providers FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER models_updated_at    BEFORE UPDATE ON models    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER api_keys_updated_at  BEFORE UPDATE ON api_keys  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
