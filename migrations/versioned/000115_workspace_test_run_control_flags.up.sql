-- Some development databases applied 000114 before these fields were added.
-- A completed migration is never replayed, so repair existing tables here.
ALTER TABLE IF EXISTS workspace_test_runs
    ADD COLUMN IF NOT EXISTS runner_started BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS cancel_requested BOOLEAN NOT NULL DEFAULT false;
