-- Rollback: drop users.is_operations_admin column and its index.

DO $$ BEGIN RAISE NOTICE '[Migration 000114 DOWN] Dropping users.is_operations_admin...'; END $$;

DROP INDEX IF EXISTS idx_users_is_operations_admin;

ALTER TABLE users DROP COLUMN IF EXISTS is_operations_admin;

DO $$ BEGIN RAISE NOTICE '[Migration 000114 DOWN] Done.'; END $$;
