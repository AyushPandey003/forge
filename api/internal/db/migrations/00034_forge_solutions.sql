-- +goose Up
-- Forge: Solutions move through a configurable workflow of stages. Each stage
-- collects artifacts (ordinary documents) and closes at an approval gate.

CREATE TABLE forge_workflows (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  -- Full definition: stages, artifact kinds, gate rules. See forge.Workflow.
  definition  JSONB NOT NULL,
  is_default  BOOLEAN NOT NULL DEFAULT false,
  created_by  TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX forge_workflows_one_default ON forge_workflows (is_default) WHERE is_default;

CREATE TABLE forge_solutions (
  id            TEXT PRIMARY KEY,
  key           TEXT NOT NULL UNIQUE,           -- human key, e.g. SOL-12
  title         TEXT NOT NULL,
  summary       TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'active', -- active | shipped | archived
  workflow_id   TEXT NOT NULL REFERENCES forge_workflows(id),
  -- Snapshot of the workflow at creation, so editing a template never
  -- silently changes the rules of an in-flight solution.
  workflow      JSONB NOT NULL,
  current_stage TEXT NOT NULL,
  folder_id     TEXT REFERENCES folders(id) ON DELETE SET NULL,
  created_by    TEXT NOT NULL REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE SEQUENCE forge_solution_key_seq;

CREATE TABLE forge_members (
  solution_id TEXT NOT NULL REFERENCES forge_solutions(id) ON DELETE CASCADE,
  user_id     TEXT NOT NULL REFERENCES users(id),
  -- product_owner | engineering_owner | ux_owner | implementer |
  -- solution_approver | implementation_approver | ux_approver | consumer
  role        TEXT NOT NULL,
  added_by    TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (solution_id, user_id, role)
);
CREATE INDEX forge_members_user ON forge_members (user_id);

CREATE TABLE forge_artifacts (
  id          TEXT PRIMARY KEY,
  solution_id TEXT NOT NULL REFERENCES forge_solutions(id) ON DELETE CASCADE,
  stage       TEXT NOT NULL,
  kind        TEXT NOT NULL,   -- problem_statement | prd | design | prototype | impl_doc | ...
  document_id TEXT REFERENCES documents(id) ON DELETE CASCADE,
  url         TEXT NOT NULL DEFAULT '',   -- external links (prototype link, Linear, PR)
  title       TEXT NOT NULL DEFAULT '',
  meta        JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_by  TEXT NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX forge_artifacts_solution ON forge_artifacts (solution_id, stage);
CREATE INDEX forge_artifacts_document ON forge_artifacts (document_id);

-- One row per decision. The latest row per (solution, stage, gate, user) is
-- that approver's current decision. Pinned to the artifact versions it saw.
CREATE TABLE forge_approvals (
  id          TEXT PRIMARY KEY,
  solution_id TEXT NOT NULL REFERENCES forge_solutions(id) ON DELETE CASCADE,
  stage       TEXT NOT NULL,
  gate        TEXT NOT NULL,        -- gate id within the stage (e.g. "solution", "ux")
  user_id     TEXT NOT NULL REFERENCES users(id),
  decision    TEXT NOT NULL,        -- approved | changes_requested | rejected
  comment     TEXT NOT NULL DEFAULT '',
  -- {artifact_id: version_id} at decision time.
  pinned      JSONB NOT NULL DEFAULT '{}'::jsonb,
  override    BOOLEAN NOT NULL DEFAULT false,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX forge_approvals_lookup ON forge_approvals (solution_id, stage, gate, created_at DESC);

-- Append-only audit log / timeline.
CREATE TABLE forge_events (
  id          BIGSERIAL PRIMARY KEY,
  solution_id TEXT NOT NULL REFERENCES forge_solutions(id) ON DELETE CASCADE,
  actor_id    TEXT NOT NULL DEFAULT '',
  kind        TEXT NOT NULL,
  data        JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX forge_events_solution ON forge_events (solution_id, id DESC);

-- Durable background jobs (Stitch generation, AI checks).
CREATE TABLE forge_jobs (
  id          TEXT PRIMARY KEY,
  kind        TEXT NOT NULL,
  owner_id    TEXT NOT NULL,
  status      TEXT NOT NULL,        -- queued | running | done | needs_input | error
  stage       TEXT NOT NULL DEFAULT '',
  input       JSONB NOT NULL DEFAULT '{}'::jsonb,
  result      JSONB NOT NULL DEFAULT '{}'::jsonb,
  error       TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX forge_jobs_owner ON forge_jobs (owner_id, created_at DESC);

-- +goose Down
DROP TABLE forge_jobs;
DROP TABLE forge_events;
DROP TABLE forge_approvals;
DROP TABLE forge_artifacts;
DROP TABLE forge_members;
DROP SEQUENCE forge_solution_key_seq;
DROP TABLE forge_solutions;
DROP TABLE forge_workflows;
