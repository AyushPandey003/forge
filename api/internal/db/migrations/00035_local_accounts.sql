-- +goose Up
CREATE TABLE forge_workspaces (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE forge_workspace_members (
 workspace_id TEXT NOT NULL REFERENCES forge_workspaces(id),
 user_id TEXT NOT NULL REFERENCES users(id),
 role TEXT NOT NULL CHECK (role IN ('owner','admin','member','guest')),
 PRIMARY KEY(workspace_id,user_id)
);
CREATE TABLE forge_teams (
 id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES forge_workspaces(id),
 name TEXT NOT NULL, UNIQUE(workspace_id,name)
);
CREATE TABLE forge_team_members (
 team_id TEXT NOT NULL REFERENCES forge_teams(id), user_id TEXT NOT NULL REFERENCES users(id),
 role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('lead','member')),
 PRIMARY KEY(team_id,user_id)
);
CREATE TABLE forge_local_accounts (
 user_id TEXT PRIMARY KEY REFERENCES users(id),
 username TEXT NOT NULL UNIQUE CHECK (username = lower(username)),
 password_hash TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE forge_account_links (
 id TEXT PRIMARY KEY, token_hash TEXT NOT NULL UNIQUE,
 kind TEXT NOT NULL CHECK (kind IN ('invite','reset')),
 workspace_id TEXT NOT NULL REFERENCES forge_workspaces(id),
 role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member','guest')),
 team_id TEXT REFERENCES forge_teams(id),
 user_id TEXT REFERENCES users(id),
 created_by TEXT NOT NULL REFERENCES users(id),
 expires_at TIMESTAMPTZ NOT NULL,
 used_at TIMESTAMPTZ, revoked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE forge_local_sessions (
 token_hash TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES forge_local_accounts(user_id),
 expires_at TIMESTAMPTZ NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE forge_account_events (
 id BIGSERIAL PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES forge_workspaces(id),
 actor_id TEXT NOT NULL, kind TEXT NOT NULL, target_id TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE forge_account_events;
DROP TABLE forge_local_sessions;
DROP TABLE forge_account_links;
DROP TABLE forge_local_accounts;
DROP TABLE forge_team_members;
DROP TABLE forge_teams;
DROP TABLE forge_workspace_members;
DROP TABLE forge_workspaces;
