DROP INDEX IF EXISTS idx_users_is_operations_admin;
ALTER TABLE users DROP COLUMN is_operations_admin;
