CREATE TABLE servers (
    id                          TEXT PRIMARY KEY,
    name                        TEXT NOT NULL,
    host                        TEXT NOT NULL,
    port                        INTEGER NOT NULL,
    username                    TEXT NOT NULL,
    authentication_type         TEXT NOT NULL,
    credential_reference        TEXT NOT NULL DEFAULT '',
    private_key_path            TEXT NOT NULL DEFAULT '',
    host_key_algorithm          TEXT NOT NULL DEFAULT '',
    host_key_fingerprint        TEXT NOT NULL DEFAULT '',
    connection_timeout_seconds  INTEGER NOT NULL,
    command_timeout_seconds     INTEGER NOT NULL,
    created_at                  TEXT NOT NULL,
    updated_at                  TEXT NOT NULL
);

CREATE TABLE backup_jobs (
    id                              TEXT PRIMARY KEY,
    name                            TEXT NOT NULL,
    server_id                       TEXT NOT NULL REFERENCES servers(id),
    enabled                         INTEGER NOT NULL,
    archive_format                  TEXT NOT NULL,
    remote_temp_directory           TEXT NOT NULL,
    local_destination               TEXT NOT NULL,
    archive_timeout_seconds         INTEGER NOT NULL,
    health_check_command            TEXT NOT NULL DEFAULT '',
    health_check_attempts           INTEGER NOT NULL DEFAULT 0,
    health_check_interval_seconds   INTEGER NOT NULL DEFAULT 0,
    created_at                      TEXT NOT NULL,
    updated_at                      TEXT NOT NULL
);

CREATE INDEX idx_backup_jobs_server ON backup_jobs(server_id);

CREATE TABLE backup_sources (
    id               TEXT PRIMARY KEY,
    job_id           TEXT NOT NULL REFERENCES backup_jobs(id) ON DELETE CASCADE,
    remote_path      TEXT NOT NULL,
    position         INTEGER NOT NULL,
    include_patterns TEXT NOT NULL DEFAULT '',
    exclude_patterns TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_backup_sources_job ON backup_sources(job_id);

CREATE TABLE job_scripts (
    id               TEXT PRIMARY KEY,
    job_id           TEXT NOT NULL REFERENCES backup_jobs(id) ON DELETE CASCADE,
    script_type      TEXT NOT NULL,
    command          TEXT NOT NULL,
    position         INTEGER NOT NULL,
    timeout_seconds  INTEGER NOT NULL,
    run_condition    TEXT NOT NULL,
    critical_cleanup INTEGER NOT NULL,
    retry_on_failure INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_job_scripts_job ON job_scripts(job_id);

CREATE TABLE schedules (
    id                TEXT PRIMARY KEY,
    job_id            TEXT NOT NULL UNIQUE REFERENCES backup_jobs(id) ON DELETE CASCADE,
    schedule_type     TEXT NOT NULL,
    hour              INTEGER NOT NULL DEFAULT 0,
    minute            INTEGER NOT NULL DEFAULT 0,
    weekdays          TEXT NOT NULL DEFAULT '',
    day_of_month      INTEGER NOT NULL DEFAULT 0,
    cron_expression   TEXT NOT NULL DEFAULT '',
    missed_run_policy TEXT NOT NULL
);

CREATE TABLE retention_policies (
    id           TEXT PRIMARY KEY,
    job_id       TEXT NOT NULL UNIQUE REFERENCES backup_jobs(id) ON DELETE CASCADE,
    keep_last    INTEGER,
    max_age_days INTEGER
);

-- backup_runs.job_id intentionally carries no foreign key constraint: safe
-- job deletion (backup-jobs specification) can remove a job while keeping
-- its run history, so a run may reference a job id that no longer exists.
CREATE TABLE backup_runs (
    id               TEXT PRIMARY KEY,
    job_id           TEXT NOT NULL,
    server_id        TEXT NOT NULL,
    source_paths     TEXT NOT NULL,
    trigger          TEXT NOT NULL,
    started_at       TEXT NOT NULL,
    finished_at      TEXT,
    status           TEXT NOT NULL,
    archive_name     TEXT NOT NULL DEFAULT '',
    archive_size     INTEGER NOT NULL DEFAULT 0,
    checksum         TEXT NOT NULL DEFAULT '',
    error_code       TEXT NOT NULL DEFAULT '',
    error_message    TEXT NOT NULL DEFAULT '',
    recovery_outcome TEXT NOT NULL DEFAULT 'NOT_APPLICABLE'
);

CREATE INDEX idx_backup_runs_job_started ON backup_runs(job_id, started_at);

CREATE TABLE run_steps (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL REFERENCES backup_runs(id) ON DELETE CASCADE,
    step_type   TEXT NOT NULL,
    position    INTEGER NOT NULL,
    started_at  TEXT NOT NULL,
    finished_at TEXT,
    status      TEXT NOT NULL,
    output      TEXT NOT NULL DEFAULT '',
    truncated   INTEGER NOT NULL DEFAULT 0,
    error       TEXT NOT NULL DEFAULT '',
    exit_code   INTEGER,
    duration_ms INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_run_steps_run ON run_steps(run_id, position);

CREATE TABLE job_locks (
    job_id     TEXT PRIMARY KEY REFERENCES backup_jobs(id) ON DELETE CASCADE,
    run_id     TEXT NOT NULL,
    started_at TEXT NOT NULL,
    process_id INTEGER NOT NULL
);

CREATE TABLE settings (
    id                              INTEGER PRIMARY KEY CHECK (id = 1),
    schedules_paused                INTEGER NOT NULL DEFAULT 0,
    global_concurrency_limit        INTEGER NOT NULL DEFAULT 3,
    notify_success                  INTEGER NOT NULL DEFAULT 1,
    notify_failure                  INTEGER NOT NULL DEFAULT 1,
    stale_remote_cleanup_enabled    INTEGER NOT NULL DEFAULT 0,
    stale_remote_cleanup_age_hours  INTEGER NOT NULL DEFAULT 24
);

INSERT INTO settings (id) VALUES (1);
