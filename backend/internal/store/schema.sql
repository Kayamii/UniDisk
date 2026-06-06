-- UniDisk metadata schema.
-- UniDisk stores ONLY metadata and orchestration state. File bytes live in
-- the user's connected providers, never here.

PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

-- Roles group privileges. Built-in roles (Admin, Viewer) are seeded on boot;
-- admins can create custom roles. is_system marks the un-deletable built-ins.
CREATE TABLE IF NOT EXISTS roles (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    is_system  INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- One row per privilege granted to a role (e.g. 'files.upload').
CREATE TABLE IF NOT EXISTS role_privileges (
    role_id   INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    privilege TEXT NOT NULL,
    PRIMARY KEY (role_id, privilege)
);

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    -- Exactly one role per user. RESTRICT prevents deleting a role still in use.
    role_id       INTEGER REFERENCES roles(id) ON DELETE RESTRICT,
    -- When 1, the user must set a new password before doing anything else
    -- (admin-created accounts start with a temp password).
    must_change_password INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- A connected storage account (one row per provider login the user added
-- through the dashboard). credentials_json is encrypted at rest.
CREATE TABLE IF NOT EXISTS accounts (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL,            -- 'googledrive' | 'dropbox' | 's3' ...
    display_name     TEXT NOT NULL,            -- user-facing label
    credentials_json TEXT NOT NULL,            -- provider credentials, AES-GCM encrypted at rest
    status           TEXT NOT NULL DEFAULT 'active', -- 'active' | 'error'
    quota_bytes      INTEGER NOT NULL DEFAULT 0,     -- last-known total capacity
    used_bytes       INTEGER NOT NULL DEFAULT 0,     -- last-known used space
    priority         INTEGER NOT NULL DEFAULT 100,   -- routing priority (lower first)
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_checked_at  TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_accounts_user ON accounts(user_id);

-- A logical file or folder in the unified pool. A file's bytes may be stored
-- on one account (or, in future, split across several via the blocks table).
CREATE TABLE IF NOT EXISTS files (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id   INTEGER REFERENCES files(id) ON DELETE CASCADE, -- NULL = root
    name        TEXT NOT NULL,
    is_dir      INTEGER NOT NULL DEFAULT 0,
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    mime_type   TEXT NOT NULL DEFAULT 'application/octet-stream',
    -- For a file: which account holds it and the provider's own object id.
    account_id  INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    remote_id   TEXT,                                  -- provider's file id
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- The pool is shared, so names are unique within a folder across all users.
    -- COALESCE maps root's NULL parent to 0 so the constraint applies there too.
    UNIQUE(parent_id, name)
);

CREATE INDEX IF NOT EXISTS idx_files_parent ON files(parent_id);

-- Instance-wide routing settings. The pool is shared across all users, so
-- there is a single row (id = 1). fill_threshold_pct is the soft cap:
-- accounts at/above this percent used are skipped for new files unless every
-- account is above it.
CREATE TABLE IF NOT EXISTS settings (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    fill_threshold_pct INTEGER NOT NULL DEFAULT 80
);
INSERT OR IGNORE INTO settings (id, fill_threshold_pct) VALUES (1, 80);

-- API keys let a user authenticate programmatic requests. Only a hash of the
-- key is stored (the plaintext is shown once at creation). Keys carry a subset
-- of file privileges, never exceeding the owner's at use time.
CREATE TABLE IF NOT EXISTS api_keys (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    key_hash     TEXT NOT NULL UNIQUE,      -- sha256 of the plaintext key
    key_prefix   TEXT NOT NULL,             -- first chars, shown for identification
    privileges   TEXT NOT NULL DEFAULT '',  -- comma-separated file privileges
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at   TIMESTAMP,                 -- NULL = never expires
    last_used_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);

-- Presigned URLs are public, time-limited, read-only links to a single file.
-- The token is the random URL segment; downloads run through UniDisk.
CREATE TABLE IF NOT EXISTS presigned_urls (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    token       TEXT NOT NULL UNIQUE,
    file_id     INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    created_by  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at  TIMESTAMP,                  -- NULL = never expires
    downloads   INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_presigned_file ON presigned_urls(file_id);

-- Sessions are stateless JWTs, so no session table is needed for the MVP.
