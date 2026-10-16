-- Mirrors versioned migration 000114_operations_admin:
-- users.is_operations_admin — a platform role one level above
-- is_system_admin. Operations admins inherit all system-admin privileges
-- (folded in at the auth layer) and additionally gate the operations-admin
-- management surface.

ALTER TABLE users ADD COLUMN is_operations_admin BOOLEAN NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_users_is_operations_admin ON users (is_operations_admin);
