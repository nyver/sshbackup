-- Adds the command text to run_steps, so a failed PRE_BACKUP/POST_BACKUP
-- script step's run details can show which command actually ran (not just
-- its exit code/stderr) — see docs/architecture/persisted-formats.md.
-- Empty for step types that never run a user-supplied command
-- (PREFLIGHT, ARCHIVE, DOWNLOAD, etc.).
ALTER TABLE run_steps ADD COLUMN command TEXT NOT NULL DEFAULT '';
