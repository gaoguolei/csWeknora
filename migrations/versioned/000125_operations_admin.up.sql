-- Migration: 000114_operations_admin
-- Adds users.is_operations_admin: a platform role one level ABOVE
-- system administrator (is_system_admin).
--
-- Semantics:
--   * An operations admin automatically inherits every system-admin
--     privilege. The auth middleware folds this into the request context
--     (SystemAdminContextKey = is_system_admin OR is_operations_admin),
--     so no existing RequireSystemAdmin guard needs to change.
--   * is_operations_admin additionally unlocks the operations-admin-only
--     surface (granting/revoking other operations admins), gated by the
--     distinct RequireOperationsAdmin guard.
--   * Only existing operations admins may promote/revoke this role.
--
-- Indexed for efficient privilege checks and admin listing, mirroring
-- idx_users_is_system_admin.

DO $$ BEGIN RAISE NOTICE '[Migration 000114] Adding users.is_operations_admin column...'; END $$;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_operations_admin BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_is_operations_admin ON users (is_operations_admin);

COMMENT ON COLUMN users.is_operations_admin IS 'Whether the user is an operations administrator (a level above system administrator; inherits all system-admin privileges)';

DO $$ BEGIN RAISE NOTICE '[Migration 000114] Done.'; END $$;
