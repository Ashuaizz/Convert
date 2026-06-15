CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE,
    password_hash TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO users (id, email, password_hash, status)
VALUES ('dev-user', 'dev-user@local', NULL, 'active')
ON CONFLICT (id) DO NOTHING;

CREATE TABLE files (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size BIGINT NOT NULL,
    sha256 TEXT,
    storage_uri TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_files_user_id_created_at ON files(user_id, created_at DESC);

CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    progress INT NOT NULL DEFAULT 0,
    options JSONB NOT NULL DEFAULT '{}',
    message TEXT,
    error_code TEXT,
    error_message TEXT,
    idempotency_key TEXT,
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 2,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_jobs_user_id_created_at ON jobs(user_id, created_at DESC);
CREATE INDEX idx_jobs_status_created_at ON jobs(status, created_at);
CREATE UNIQUE INDEX idx_jobs_idempotency
ON jobs(user_id, idempotency_key)
WHERE idempotency_key IS NOT NULL;

CREATE TABLE job_files (
    job_id TEXT NOT NULL REFERENCES jobs(id),
    file_id TEXT NOT NULL REFERENCES files(id),
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, file_id, role)
);
