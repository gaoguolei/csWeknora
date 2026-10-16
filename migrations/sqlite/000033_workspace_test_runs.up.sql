CREATE TABLE IF NOT EXISTS workspace_test_runs (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    project_id TEXT NOT NULL DEFAULT '',
    sandbox_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    runner_started BOOLEAN NOT NULL DEFAULT false,
    cancel_requested BOOLEAN NOT NULL DEFAULT false,
    request TEXT NOT NULL DEFAULT '{}',
    result TEXT NOT NULL DEFAULT '{}',
    report_message_id TEXT NOT NULL DEFAULT '',
    report_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_workspace_test_runs_history ON workspace_test_runs (tenant_id, user_id, session_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_test_runs_active ON workspace_test_runs (tenant_id, session_id) WHERE status IN ('queued', 'preparing', 'running', 'cancelling');
