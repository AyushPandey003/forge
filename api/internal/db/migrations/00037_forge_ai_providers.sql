-- +goose Up
-- Bring-your-own model providers for Forge design generation. API keys are
-- sealed (AES-GCM) before they reach this table and are never returned.
CREATE TABLE forge_ai_providers (
  id             TEXT PRIMARY KEY,
  user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  label          TEXT NOT NULL,
  kind           TEXT NOT NULL CHECK (kind IN ('openai','anthropic')),
  base_url       TEXT NOT NULL,
  key_ciphertext BYTEA NOT NULL,
  key_hint       TEXT NOT NULL DEFAULT '',
  default_model  TEXT NOT NULL DEFAULT '',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, label)
);
CREATE INDEX forge_ai_providers_user ON forge_ai_providers(user_id);

-- +goose Down
DROP TABLE forge_ai_providers;
