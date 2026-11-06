-- Restores the per-workspace uniqueness of skill definitions. The rows
-- collapsed by the up migration stay collapsed: resurrecting them would hand
-- back duplicates no workspace asked for.

DROP INDEX IF EXISTS uq_tenant_skill_catalog_name;

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_skill_catalog_name
ON tenant_skill_catalog (tenant_id, name) WHERE deleted_at IS NULL;
